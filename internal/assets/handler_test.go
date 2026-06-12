package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/jackc/pgx/v5"
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

func TestCreateUploadURLHandlerReturnsPlanLimitExceeded(t *testing.T) {
	handler := &Handler{
		billingDB: billingAccessStoreStub{
			entitlement: billing.Entitlement{
				WorkspaceID:            "workspace-1",
				Plan:                   "basic",
				Status:                 "active",
				SubscriptionLive:       true,
				AssetStorageLimitBytes: int64Ptr(100),
			},
			assetBytes: 90,
		},
		service: &fakeAssetService{},
		authorizer: fakeAssetAuthorizer{
			siteScope: authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: authorization.RoleEditor},
		},
	}

	body, _ := json.Marshal(map[string]any{
		"siteId":      "site-1",
		"fileName":    "hero.png",
		"contentType": "image/png",
		"sizeBytes":   20,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: "user-1"}))
	res := httptest.NewRecorder()

	handler.createUploadURL(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
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

func (s *fakeAssetService) PublicDownloadURLByHostname(context.Context, string, string) (string, error) {
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

type billingAccessStoreStub struct {
	entitlement     billing.Entitlement
	activeSiteCount int
	assetBytes      int64
	periodStart     *time.Time
	periodEnd       *time.Time
}

func (s billingAccessStoreStub) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "from billing_entitlements"):
		return assetRowStub{values: []any{
			s.entitlement.WorkspaceID,
			s.entitlement.Plan,
			s.entitlement.Status,
			s.entitlement.SubscriptionLive,
			s.entitlement.CustomDomainsEnabled,
			s.entitlement.ActiveSiteLimit,
			s.entitlement.MonthlyPromptLimit,
			s.entitlement.AssetStorageLimitBytes,
			s.entitlement.CollectionLimit,
			s.entitlement.CollectionEntryLimit,
			time.Now().UTC(),
		}}
	case strings.Contains(sql, "from sites"):
		return assetRowStub{values: []any{s.activeSiteCount}}
	case strings.Contains(sql, "from assets"):
		return assetRowStub{values: []any{s.assetBytes}}
	case strings.Contains(sql, "from billing_subscriptions"):
		return assetRowStub{err: pgx.ErrNoRows}
	default:
		return assetRowStub{err: pgx.ErrNoRows}
	}
}

type assetRowStub struct {
	values []any
	err    error
}

func (r assetRowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index := range dest {
		switch target := dest[index].(type) {
		case *string:
			*target = r.values[index].(string)
		case *bool:
			*target = r.values[index].(bool)
		case *int:
			*target = r.values[index].(int)
		case *int64:
			*target = r.values[index].(int64)
		case **int:
			*target = r.values[index].(*int)
		case **int64:
			*target = r.values[index].(*int64)
		case *time.Time:
			*target = r.values[index].(time.Time)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func int64Ptr(value int64) *int64 {
	return &value
}
