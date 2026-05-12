package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
)

func TestCreateUploadURLHandler(t *testing.T) {
	handler := &Handler{
		service: &fakeAssetService{
			createUploadResult: CreateUploadResult{
				Asset:  Asset{ID: "asset-1", SiteID: "site-1", Kind: "image", CreatedAt: time.Now().UTC()},
				Upload: PresignedUpload{URL: "http://upload.test", Method: "PUT"},
			},
		},
		authorizer: fakeAssetAuthorizer{
			siteScope: authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: authorization.RoleEditor},
		},
	}

	body, _ := json.Marshal(map[string]any{
		"siteId":      "site-1",
		"fileName":    "hero.png",
		"contentType": "image/png",
		"sizeBytes":   2048,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "user-1"}))
	res := httptest.NewRecorder()

	handler.createUploadURL(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}
}

func TestDeleteAssetHandler(t *testing.T) {
	handler := &Handler{
		service: &fakeAssetService{},
		authorizer: fakeAssetAuthorizer{
			assetScope: authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", AssetID: "asset-1", Role: authorization.RoleEditor},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/asset-1", nil)
	req.SetPathValue("assetId", "asset-1")
	req = req.WithContext(context.Background())
	res := httptest.NewRecorder()

	handler.deleteAsset(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestRedirectPublicAssetContentHandler(t *testing.T) {
	handler := &Handler{
		service: &fakeAssetService{
			publicDownloadURL: "http://download.test/public-asset",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/public/sites/loom-light/assets/asset-1", nil)
	req.SetPathValue("siteSlug", "loom-light")
	req.SetPathValue("assetId", "asset-1")
	res := httptest.NewRecorder()

	handler.redirectPublicAssetContent(res, req)

	if res.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, res.Code)
	}
	if location := res.Header().Get("Location"); location != "http://download.test/public-asset" {
		t.Fatalf("expected redirect location, got %q", location)
	}
}

type fakeAssetService struct {
	createUploadResult CreateUploadResult
	createUploadErr    error
	downloadURL        string
	publicDownloadURL  string
}

func (s *fakeAssetService) CreateUpload(context.Context, CreateUploadInput) (CreateUploadResult, error) {
	return s.createUploadResult, s.createUploadErr
}

func (s *fakeAssetService) CompleteUpload(context.Context, string, CompleteUploadInput) (Asset, error) {
	return Asset{}, nil
}

func (s *fakeAssetService) DownloadURL(context.Context, string) (string, error) {
	return s.downloadURL, nil
}

func (s *fakeAssetService) PublicDownloadURLBySiteSlug(context.Context, string, string) (string, error) {
	return s.publicDownloadURL, nil
}

func (s *fakeAssetService) ListBySite(context.Context, string) ([]Asset, error) {
	return nil, nil
}

func (s *fakeAssetService) Update(context.Context, string, UpdateAssetInput) (Asset, error) {
	return Asset{}, nil
}

func (s *fakeAssetService) Delete(context.Context, string) error {
	return nil
}

type fakeAssetAuthorizer struct {
	siteScope  authorization.Scope
	assetScope authorization.Scope
	err        error
}

func (a fakeAssetAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return a.siteScope, a.err
}

func (a fakeAssetAuthorizer) RequireAsset(context.Context, string, ...string) (authorization.Scope, error) {
	return a.assetScope, a.err
}
