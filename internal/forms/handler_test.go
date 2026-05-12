package forms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type fakeFormService struct {
	submitInput  SubmitInput
	listSiteID   string
	updateID     string
	updateInput  UpdateSubmissionInput
	submitResult SubmitResult
	submissions  []Submission
	updated      Submission
	err          error
}

func (f *fakeFormService) Submit(_ context.Context, input SubmitInput) (SubmitResult, error) {
	f.submitInput = input
	return f.submitResult, f.err
}

func (f *fakeFormService) ListBySite(_ context.Context, siteID string) ([]Submission, error) {
	f.listSiteID = siteID
	return f.submissions, f.err
}

func (f *fakeFormService) UpdateStatus(_ context.Context, submissionID string, input UpdateSubmissionInput) (Submission, error) {
	f.updateID = submissionID
	f.updateInput = input
	return f.updated, f.err
}

type fakeFormsAuthorizer struct{}

func (fakeFormsAuthorizer) RequireSite(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: authorization.RoleOwner}, nil
}

func (fakeFormsAuthorizer) RequireFormSubmission(context.Context, string, ...string) (authorization.Scope, error) {
	return authorization.Scope{WorkspaceID: "workspace-1", SiteID: "site-1", SubmissionID: "submission-1", Role: authorization.RoleOwner}, nil
}

type staticLimiter struct {
	allowed bool
}

func (l staticLimiter) Allow(string) bool {
	return l.allowed
}

func TestSubmitReturnsGenericAcceptedResponse(t *testing.T) {
	service := &fakeFormService{
		submitResult: SubmitResult{
			Submission:     Submission{ID: "submission-1"},
			SuccessMessage: "Thanks. Your message is on its way.",
		},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeFormsAuthorizer{},
		limiter:    staticLimiter{allowed: true},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/public/forms/site-1/block-contact/submit", strings.NewReader(`{"payload":{"email":"ada@example.com"}}`))
	req.RemoteAddr = "127.0.0.1:4567"
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-contact")
	res := httptest.NewRecorder()

	handler.submit(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected accepted status, got %d", res.Code)
	}
	if service.submitInput.SiteID != "site-1" || service.submitInput.BlockID != "block-contact" {
		t.Fatalf("expected submit target to reach service, got %#v", service.submitInput)
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Thanks. Your message is on its way." {
		t.Fatalf("expected generic success message, got %#v", payload)
	}
}

func TestSubmitReturnsValidationIssues(t *testing.T) {
	service := &fakeFormService{
		err: siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path: "payload.email",
			Code: "invalid_email",
		}}},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeFormsAuthorizer{},
		limiter:    staticLimiter{allowed: true},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/public/forms/site-1/block-contact/submit", strings.NewReader(`{"payload":{"email":"bad"}}`))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-contact")
	res := httptest.NewRecorder()

	handler.submit(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", res.Code)
	}
}

func TestSubmitRespectsRateLimit(t *testing.T) {
	handler := Handler{
		service:    &fakeFormService{},
		authorizer: fakeFormsAuthorizer{},
		limiter:    staticLimiter{allowed: false},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/public/forms/site-1/block-contact/submit", strings.NewReader(`{}`))
	req.SetPathValue("siteId", "site-1")
	req.SetPathValue("blockId", "block-contact")
	res := httptest.NewRecorder()

	handler.submit(res, req)

	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit response, got %d", res.Code)
	}
}

func TestListAndUpdateUseAuthenticatedService(t *testing.T) {
	nextStatus := "reviewed"
	service := &fakeFormService{
		submissions: []Submission{{ID: "submission-1", Status: "new", CreatedAt: time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)}},
		updated:     Submission{ID: "submission-1", Status: "reviewed"},
	}
	handler := Handler{
		service:    service,
		authorizer: fakeFormsAuthorizer{},
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sites/site-1/form-submissions", nil)
	listReq.SetPathValue("siteId", "site-1")
	listRes := httptest.NewRecorder()
	handler.list(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list success, got %d", listRes.Code)
	}
	if service.listSiteID != "site-1" {
		t.Fatalf("expected site id to reach list service, got %q", service.listSiteID)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/form-submissions/submission-1", strings.NewReader(`{"status":"reviewed"}`))
	updateReq.SetPathValue("submissionId", "submission-1")
	updateRes := httptest.NewRecorder()
	handler.update(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("expected update success, got %d", updateRes.Code)
	}
	if service.updateID != "submission-1" || service.updateInput.Status == nil || *service.updateInput.Status != nextStatus {
		t.Fatalf("expected update input to reach service, got %#v %#v", service.updateID, service.updateInput)
	}
}

func TestInMemorySubmissionRateLimiterTrimsExpiredEntries(t *testing.T) {
	limiter := newInMemorySubmissionRateLimiter(2, time.Minute)
	base := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return base }

	if !limiter.Allow("site:block:ip") || !limiter.Allow("site:block:ip") {
		t.Fatal("expected first two attempts to pass")
	}
	if limiter.Allow("site:block:ip") {
		t.Fatal("expected third attempt to be limited")
	}

	limiter.now = func() time.Time { return base.Add(2 * time.Minute) }
	if !limiter.Allow("site:block:ip") {
		t.Fatal("expected limiter to reset after the window elapsed")
	}
}

func TestWriteAuthorizationErrorMapsForbidden(t *testing.T) {
	res := httptest.NewRecorder()
	writeAuthorizationError(res, authorization.ErrForbidden)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", res.Code)
	}
}

func TestWriteFormErrorMapsSubmissionStatusValidation(t *testing.T) {
	res := httptest.NewRecorder()
	writeFormError(res, ErrSubmissionStatusInvalid)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", res.Code)
	}
}

func TestWriteFormErrorFallsBackToServerError(t *testing.T) {
	res := httptest.NewRecorder()
	writeFormError(res, errors.New("boom"))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected server error, got %d", res.Code)
	}
}
