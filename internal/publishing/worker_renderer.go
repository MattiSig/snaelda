package publishing

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerRendererConfig configures the long-lived Node-based artifact renderer.
type WorkerRendererConfig struct {
	// PublicBaseURL is used when the inbound render request leaves the field empty.
	PublicBaseURL string
	// APIBaseURL is exported as API_BASE_URL to the worker so that asset URLs
	// in rendered HTML resolve to this API (e.g. http://localhost:8080 in dev,
	// https://api.snaelda.io in prod). Without it, the Node renderer has no way
	// to know the API host and falls back to a hard-coded default.
	APIBaseURL string
	// Command is the executable used to launch the renderer. Defaults to "npm".
	Command string
	// Args are the arguments passed to Command. Defaults to the npm-script invocation
	// that runs `tsx src/scripts/render-published-artifacts.ts` in worker mode.
	Args []string
	// WorkingDir is the working directory for the worker process. Defaults to "".
	WorkingDir string
	// StartupTimeout caps how long we wait for the worker to accept a request before
	// declaring it unhealthy. Defaults to 30s.
	StartupTimeout time.Duration
	// RequestTimeout caps how long a single render may take before we kill the worker
	// and retry. Defaults to 60s.
	RequestTimeout time.Duration
	// Logger receives worker stderr lines and lifecycle events.
	Logger *slog.Logger
}

// workerArtifactRenderer keeps a single Node child process alive and serializes
// render requests through it. On crash or timeout, the worker is rebuilt on
// next use.
type workerArtifactRenderer struct {
	publicBaseURL  string
	apiBaseURL     string
	command        string
	args           []string
	workingDir     string
	requestTimeout time.Duration
	logger         *slog.Logger

	mu      sync.Mutex
	current *workerProcess
	nextID  atomic.Uint64
}

type workerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	wait   chan error
	closed atomic.Bool
}

type workerRequest struct {
	ID    string              `json:"id"`
	Input ArtifactRenderInput `json:"input"`
}

type workerResponse struct {
	ID     string         `json:"id,omitempty"`
	Bundle ArtifactBundle `json:"bundle"`
	Error  string         `json:"error,omitempty"`
}

// NewWorkerArtifactRenderer builds a renderer that talks to a long-lived Node
// process over stdin/stdout newline-delimited JSON.
func NewWorkerArtifactRenderer(cfg WorkerRendererConfig) ArtifactRenderer {
	command := strings.TrimSpace(cfg.Command)
	args := cfg.Args
	if command == "" {
		command = "npm"
		args = []string{"run", "--workspace", "@snaelda/web", "--silent", "render:artifacts"}
	}
	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 60 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &workerArtifactRenderer{
		publicBaseURL:  strings.TrimSpace(cfg.PublicBaseURL),
		apiBaseURL:     strings.TrimSpace(cfg.APIBaseURL),
		command:        command,
		args:           args,
		workingDir:     cfg.WorkingDir,
		requestTimeout: requestTimeout,
		logger:         logger,
	}
}

func (r *workerArtifactRenderer) Render(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error) {
	payload := input
	if strings.TrimSpace(payload.PublicBaseURL) == "" {
		payload.PublicBaseURL = r.publicBaseURL
	}

	// One retry to cover the case where the worker died between requests.
	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		bundle, err := r.renderOnce(ctx, payload)
		if err == nil {
			return bundle, nil
		}
		lastErr = err
		if !isRetryableWorkerError(err) {
			return ArtifactBundle{}, err
		}
		r.logger.Warn("artifact render worker failed; restarting", "attempt", attempt+1, "error", err.Error())
		r.killCurrent()
	}
	return ArtifactBundle{}, lastErr
}

func (r *workerArtifactRenderer) renderOnce(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureWorkerLocked(ctx); err != nil {
		return ArtifactBundle{}, err
	}

	worker := r.current
	requestID := strconv.FormatUint(r.nextID.Add(1), 10)
	body, err := json.Marshal(workerRequest{ID: requestID, Input: input})
	if err != nil {
		return ArtifactBundle{}, fmt.Errorf("encode artifact render input: %w", err)
	}
	body = append(body, '\n')

	requestCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	defer cancel()

	type readResult struct {
		line []byte
		err  error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		line, err := worker.stdout.ReadBytes('\n')
		resultCh <- readResult{line: line, err: err}
	}()

	if _, err := worker.stdin.Write(body); err != nil {
		return ArtifactBundle{}, &workerProtocolError{message: fmt.Sprintf("write render request: %v", err)}
	}

	select {
	case <-requestCtx.Done():
		return ArtifactBundle{}, &workerProtocolError{message: fmt.Sprintf("render request: %v", requestCtx.Err())}
	case res := <-resultCh:
		if res.err != nil {
			if errors.Is(res.err, io.EOF) || errors.Is(res.err, io.ErrUnexpectedEOF) {
				return ArtifactBundle{}, &workerProtocolError{message: "render worker terminated unexpectedly"}
			}
			return ArtifactBundle{}, &workerProtocolError{message: fmt.Sprintf("read render response: %v", res.err)}
		}
		var response workerResponse
		if err := json.Unmarshal(res.line, &response); err != nil {
			return ArtifactBundle{}, &workerProtocolError{message: fmt.Sprintf("decode render response: %v", err)}
		}
		if response.Error != "" {
			return ArtifactBundle{}, fmt.Errorf("render published artifacts: %s", response.Error)
		}
		if response.ID != "" && response.ID != requestID {
			return ArtifactBundle{}, &workerProtocolError{message: fmt.Sprintf("render response id mismatch: want %q, got %q", requestID, response.ID)}
		}
		if response.Bundle.SchemaVersion == "" {
			return ArtifactBundle{}, fmt.Errorf("decode rendered artifacts: missing schema version")
		}
		return response.Bundle, nil
	}
}

func (r *workerArtifactRenderer) ensureWorkerLocked(_ context.Context) error {
	if r.current != nil && !r.current.closed.Load() {
		return nil
	}

	cmd := exec.Command(r.command, r.args...)
	if r.workingDir != "" {
		cmd.Dir = r.workingDir
	}
	if r.apiBaseURL != "" {
		cmd.Env = append(os.Environ(), "API_BASE_URL="+r.apiBaseURL)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("attach render worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("attach render worker stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("attach render worker stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start render worker: %w", err)
	}

	worker := &workerProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReaderSize(stdout, 1<<20),
		wait:   make(chan error, 1),
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, 8*1024), 1<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			r.logger.Warn("artifact render worker stderr", "message", line)
		}
	}()

	go func() {
		err := cmd.Wait()
		worker.closed.Store(true)
		worker.wait <- err
		close(worker.wait)
		if err != nil {
			r.logger.Warn("artifact render worker exited", "error", err.Error())
		}
	}()

	r.current = worker
	r.logger.Info("artifact render worker started", "pid", cmd.Process.Pid)
	return nil
}

func (r *workerArtifactRenderer) killCurrent() {
	r.mu.Lock()
	worker := r.current
	r.current = nil
	r.mu.Unlock()
	if worker == nil {
		return
	}
	if !worker.closed.Load() {
		_ = worker.stdin.Close()
		if worker.cmd.Process != nil {
			_ = worker.cmd.Process.Kill()
		}
	}
	// Drain wait channel to avoid goroutine leak.
	select {
	case <-worker.wait:
	case <-time.After(2 * time.Second):
	}
}

// Close stops the worker process. Safe to call multiple times.
func (r *workerArtifactRenderer) Close() error {
	r.killCurrent()
	return nil
}

// workerProtocolError marks errors that warrant restarting the worker.
type workerProtocolError struct {
	message string
}

func (e *workerProtocolError) Error() string { return "render worker: " + e.message }

func isRetryableWorkerError(err error) bool {
	var protoErr *workerProtocolError
	return errors.As(err, &protoErr)
}
