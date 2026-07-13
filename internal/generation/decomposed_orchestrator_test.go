package generation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// retryingContentPlanner fails BuildPageContent for the first failFirst calls,
// then succeeds, recording every request so the test can assert the retry
// threaded the failure back as feedback.
type retryingContentPlanner struct {
	layout          PageLayoutResult
	content         PageContentResult
	failFirst       int
	contentRequests []PageContentRequest
}

func (p *retryingContentPlanner) BuildOutline(context.Context, OutlineRequest) (OutlineResult, error) {
	return OutlineResult{}, nil
}

func (p *retryingContentPlanner) BuildPageLayout(context.Context, PageLayoutRequest) (PageLayoutResult, error) {
	return p.layout, nil
}

func (p *retryingContentPlanner) BuildPageContent(_ context.Context, request PageContentRequest) (PageContentResult, error) {
	p.contentRequests = append(p.contentRequests, request)
	if len(p.contentRequests) <= p.failFirst {
		return PageContentResult{}, errors.New("page content block 1 type mismatch: got \"gallery\" want \"image_text\"")
	}
	return p.content, nil
}

func TestBuildPagePlanFromLayoutRetriesWithFeedback(t *testing.T) {
	planner := &retryingContentPlanner{
		layout: PageLayoutResult{Blocks: []PageLayoutBlock{
			{Type: "hero", Purpose: "Open.", ContentBrief: "Opener."},
			{Type: "footer", Purpose: "Close.", ContentBrief: "Contact."},
		}},
		content: PageContentResult{Blocks: []PageContentBlock{
			{Type: "hero", Props: map[string]any{"headline": "Hi"}},
			{Type: "footer", Props: map[string]any{"copyright": "Co"}},
		}},
		failFirst: 1,
	}
	service := &Service{decomposedPlanner: planner}

	plan, err := service.buildPagePlanFromLayout(
		context.Background(),
		"Acme",
		"Goal",
		"brief",
		"en",
		siteconfig.BrandConfig{},
		OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce."},
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("expected retry to recover, got %v", err)
	}
	if len(plan.Blocks) != 2 {
		t.Fatalf("expected 2 blocks after retry, got %d", len(plan.Blocks))
	}
	if len(planner.contentRequests) != 2 {
		t.Fatalf("expected exactly one retry (2 calls), got %d", len(planner.contentRequests))
	}
	if planner.contentRequests[0].Feedback != "" {
		t.Fatalf("expected first attempt to carry no feedback, got %q", planner.contentRequests[0].Feedback)
	}
	if !strings.Contains(planner.contentRequests[1].Feedback, "type mismatch") {
		t.Fatalf("expected retry to thread the failure as feedback, got %q", planner.contentRequests[1].Feedback)
	}
}

func TestBuildPagePlanFromLayoutFailsAfterRetryExhausted(t *testing.T) {
	planner := &retryingContentPlanner{
		layout: PageLayoutResult{Blocks: []PageLayoutBlock{
			{Type: "hero", Purpose: "Open.", ContentBrief: "Opener."},
		}},
		failFirst: 5, // always fails
	}
	service := &Service{decomposedPlanner: planner}

	_, err := service.buildPagePlanFromLayout(
		context.Background(),
		"Acme",
		"Goal",
		"brief",
		"en",
		siteconfig.BrandConfig{},
		OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce."},
		nil,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error when every attempt fails")
	}
	if len(planner.contentRequests) != maxPageContentAttempts {
		t.Fatalf("expected %d attempts, got %d", maxPageContentAttempts, len(planner.contentRequests))
	}
}

// sourceHeroRecordingPlanner records the layout and content requests so a test
// can assert the source hero rode into both per-page calls.
type sourceHeroRecordingPlanner struct {
	layoutRequest  PageLayoutRequest
	contentRequest PageContentRequest
}

func (p *sourceHeroRecordingPlanner) BuildOutline(context.Context, OutlineRequest) (OutlineResult, error) {
	return OutlineResult{}, nil
}

func (p *sourceHeroRecordingPlanner) BuildPageLayout(_ context.Context, request PageLayoutRequest) (PageLayoutResult, error) {
	p.layoutRequest = request
	return PageLayoutResult{Blocks: []PageLayoutBlock{{Type: "hero", Purpose: "Open.", ContentBrief: "Opener."}}}, nil
}

func (p *sourceHeroRecordingPlanner) BuildPageContent(_ context.Context, request PageContentRequest) (PageContentResult, error) {
	p.contentRequest = request
	return PageContentResult{Blocks: []PageContentBlock{{Type: "hero", Props: map[string]any{"headline": "Hi"}}}}, nil
}

func TestBuildPagePlanFromLayoutAttachesSourceHero(t *testing.T) {
	planner := &sourceHeroRecordingPlanner{}
	service := &Service{decomposedPlanner: planner}
	hero := &SourceHero{Headline: "Clogged Drain? We Fix Them 24/7", CTALabel: "Call now", TextOnly: true}

	if _, err := service.buildPagePlanFromLayout(
		context.Background(),
		"My Sewer Guys",
		"Goal",
		"brief",
		"en",
		siteconfig.BrandConfig{},
		OutlinePage{Title: "Home", Slug: "/", Goal: "Introduce."},
		nil,
		nil,
		hero,
	); err != nil {
		t.Fatalf("build page plan: %v", err)
	}

	if planner.layoutRequest.SourceHero == nil || planner.layoutRequest.SourceHero.Headline != hero.Headline {
		t.Fatalf("layout request missing source hero: %+v", planner.layoutRequest.SourceHero)
	}
	if planner.contentRequest.SourceHero == nil || planner.contentRequest.SourceHero.Headline != hero.Headline {
		t.Fatalf("content request missing source hero: %+v", planner.contentRequest.SourceHero)
	}
}

func TestIsHomeSlug(t *testing.T) {
	for _, slug := range []string{"/", "", "/home", "home", "/index", "  / "} {
		if !isHomeSlug(slug) {
			t.Errorf("isHomeSlug(%q) = false, want true", slug)
		}
	}
	for _, slug := range []string{"/about", "/services", "/contact", "/faq"} {
		if isHomeSlug(slug) {
			t.Errorf("isHomeSlug(%q) = true, want false", slug)
		}
	}
}
