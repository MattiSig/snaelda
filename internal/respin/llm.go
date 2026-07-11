package respin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrLLMRefusal is returned when the model refuses to answer a stage prompt.
var ErrLLMRefusal = errors.New("respin: llm refused the request")

// ErrCompleterUnavailable is returned by stage helpers when no completer is
// configured; the pipeline treats it as a degradation signal.
var ErrCompleterUnavailable = errors.New("respin: llm completer is not configured")

// Completer runs a strict structured-JSON completion for one re-spin LLM stage
// and reports token usage so the daily budget can account for spend. The
// implementation returns the model's raw JSON content, which the caller
// unmarshals into a stage-specific shape.
//
// It is deliberately small: the three re-spin stages (classify, extract,
// rewrite) each drive a single structured call, so they share one narrow seam
// that is trivial to fake in tests.
type Completer interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

// CompletionRequest is one structured-output call: a name and JSON schema the
// response must satisfy, plus the system and user messages.
type CompletionRequest struct {
	Name   string
	Schema map[string]any
	System string
	User   string
}

// CompletionResult carries the model's raw JSON content and the token usage the
// budget accounts against.
type CompletionResult struct {
	Content     string
	TotalTokens int
}

// OpenAICompleterConfig configures the OpenAI-compatible completer.
type OpenAICompleterConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

// OpenAICompleter is a Completer backed by an OpenAI-compatible chat-completions
// endpoint with structured (json_schema) output. It is intentionally independent
// of the generation planner's client: re-spin's stages have their own prompts,
// schemas, and — unlike generation — a cost budget, so coupling them to the
// generation adapter would be the wrong seam.
type OpenAICompleter struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewOpenAICompleter builds an OpenAI-backed completer. It returns (nil, nil)
// when no API key is configured so callers can degrade gracefully — a re-spin
// with no completer simply runs the fetch/extract substrate and degrades to the
// prompt flow.
func NewOpenAICompleter(cfg OpenAICompleterConfig) (*OpenAICompleter, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, nil
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("respin: llm model is required when an api key is set")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAICompleter{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

type openAIChatRequest struct {
	Model          string              `json:"model"`
	Messages       []openAIChatMessage `json:"messages"`
	ResponseFormat openAIResponseFmt   `json:"response_format"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFmt struct {
	Type       string           `json:"type"`
	JSONSchema openAISchemaWrap `json:"json_schema"`
}

type openAISchemaWrap struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Refusal string `json:"refusal,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete runs one strict structured-output call.
func (c *OpenAICompleter) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	body := openAIChatRequest{
		Model: c.model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		ResponseFormat: openAIResponseFmt{
			Type: "json_schema",
			JSONSchema: openAISchemaWrap{
				Name:   req.Name,
				Strict: true,
				Schema: req.Schema,
			},
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("encode llm request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return CompletionResult{}, fmt.Errorf("create llm request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("send llm request: %w", err)
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return CompletionResult{}, fmt.Errorf("read llm response: %w", err)
	}

	var completion openAIChatResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return CompletionResult{}, fmt.Errorf("decode llm response: %w", err)
	}
	if res.StatusCode >= http.StatusBadRequest {
		if completion.Error != nil && completion.Error.Message != "" {
			return CompletionResult{}, fmt.Errorf("llm request failed: %s", completion.Error.Message)
		}
		return CompletionResult{}, fmt.Errorf("llm request failed with status %d", res.StatusCode)
	}
	if len(completion.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("llm response did not include a choice")
	}
	message := completion.Choices[0].Message
	if strings.TrimSpace(message.Refusal) != "" {
		return CompletionResult{}, fmt.Errorf("%w: %s", ErrLLMRefusal, message.Refusal)
	}
	if strings.TrimSpace(message.Content) == "" {
		return CompletionResult{}, fmt.Errorf("llm response did not include structured content")
	}

	result := CompletionResult{Content: message.Content}
	if completion.Usage != nil {
		result.TotalTokens = completion.Usage.TotalTokens
	}
	return result, nil
}
