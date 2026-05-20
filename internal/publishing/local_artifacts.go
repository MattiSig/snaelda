package publishing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type localArtifactStore struct {
	rootDir string
}

func newLocalArtifactStore(rootDir string) ArtifactStore {
	return &localArtifactStore{rootDir: strings.TrimSpace(rootDir)}
}

func (s *localArtifactStore) Save(_ context.Context, siteID string, versionID string, bundle ArtifactBundle) error {
	if strings.TrimSpace(siteID) == "" {
		return fmt.Errorf("save artifacts: site id is required")
	}
	if strings.TrimSpace(versionID) == "" {
		return fmt.Errorf("save artifacts: version id is required")
	}
	if strings.TrimSpace(s.rootDir) == "" {
		return fmt.Errorf("save artifacts: root directory is required")
	}

	targetDir := filepath.Join(s.rootDir, siteID, versionID)
	tempDir := filepath.Join(s.rootDir, siteID, fmt.Sprintf(".tmp-%s-%d", versionID, time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return fmt.Errorf("create temporary artifact directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for _, file := range bundle.Files {
		targetPath, err := safeArtifactPath(tempDir, file.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create artifact parent directory: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(file.Body), 0o644); err != nil {
			return fmt.Errorf("write artifact %s: %w", file.Path, err)
		}
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("clear existing artifact version: %w", err)
	}
	if err := os.Rename(tempDir, targetDir); err != nil {
		return fmt.Errorf("publish artifact version: %w", err)
	}
	return nil
}

func (s *localArtifactStore) Delete(_ context.Context, siteID string, versionID string) error {
	if strings.TrimSpace(siteID) == "" || strings.TrimSpace(versionID) == "" {
		return nil
	}
	if strings.TrimSpace(s.rootDir) == "" {
		return nil
	}
	targetDir := filepath.Join(s.rootDir, siteID, versionID)
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("delete artifact version: %w", err)
	}
	return nil
}

func (s *localArtifactStore) Load(_ context.Context, siteID string, versionID string, path string) (ArtifactFile, error) {
	if strings.TrimSpace(siteID) == "" {
		return ArtifactFile{}, fmt.Errorf("load artifacts: site id is required")
	}
	if strings.TrimSpace(versionID) == "" {
		return ArtifactFile{}, fmt.Errorf("load artifacts: version id is required")
	}
	if strings.TrimSpace(s.rootDir) == "" {
		return ArtifactFile{}, fmt.Errorf("load artifacts: root directory is required")
	}

	targetPath, err := safeArtifactPath(filepath.Join(s.rootDir, siteID, versionID), path)
	if err != nil {
		return ArtifactFile{}, err
	}

	body, err := os.ReadFile(targetPath)
	if errors.Is(err, fs.ErrNotExist) {
		return ArtifactFile{}, ErrArtifactNotFound
	}
	if err != nil {
		return ArtifactFile{}, fmt.Errorf("load artifact %s: %w", path, err)
	}

	return ArtifactFile{
		Path:        filepath.Clean(strings.TrimSpace(path)),
		ContentType: http.DetectContentType(body),
		Body:        string(body),
	}, nil
}

func safeArtifactPath(root string, relativePath string) (string, error) {
	cleanRoot := filepath.Clean(root)
	cleanRelative := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelative == "." || cleanRelative == "" {
		return "", fmt.Errorf("save artifacts: artifact path is required")
	}
	if cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("save artifacts: artifact path %q escapes the artifact root", relativePath)
	}
	if filepath.IsAbs(cleanRelative) {
		return "", fmt.Errorf("save artifacts: artifact path %q must be relative", relativePath)
	}

	targetPath := filepath.Join(cleanRoot, cleanRelative)
	relativeFromRoot, err := filepath.Rel(cleanRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("save artifacts: resolve artifact path %q: %w", relativePath, err)
	}
	if relativeFromRoot == ".." || strings.HasPrefix(relativeFromRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("save artifacts: artifact path %q escapes the artifact root", relativePath)
	}
	return targetPath, nil
}
