package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGenerateCreatesDraftAndTracksCompletedJob(t *testing.T) {
	store := newFakeGenerationStore()
	store.slugs["north-light-studio"] = true

	service := Service{
		db:     store,
		writer: store,
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if result.JobID == "" {
		t.Fatal("expected generation job id")
	}
	if result.Draft.Site.Slug != "north-light-studio-2" {
		t.Fatalf("expected unique slug, got %q", result.Draft.Site.Slug)
	}
	if len(result.Draft.Pages) != 4 {
		t.Fatalf("expected four generated pages, got %d", len(result.Draft.Pages))
	}
	if result.Draft.Pages[2].Slug != "/gallery" {
		t.Fatalf("expected gallery page, got %#v", result.Draft.Pages[2])
	}
	if err := siteconfig.ValidateDraft(result.Draft); err != nil {
		t.Fatalf("expected valid draft, got %v", err)
	}

	job := store.jobs[result.JobID]
	if job.Status != "completed" {
		t.Fatalf("expected completed job, got %#v", job)
	}
	if job.SiteID != result.Draft.Site.ID {
		t.Fatalf("expected site id to be recorded on job, got %#v", job)
	}
	if store.prompts[result.Draft.Site.ID] == "" {
		t.Fatalf("expected prompt to be saved, got %#v", store.prompts)
	}
	if store.summary[result.Draft.Site.ID]["themePreset"] != "editorial-studio" {
		t.Fatalf("expected theme preset summary, got %#v", store.summary[result.Draft.Site.ID])
	}
}

func TestGenerateRejectsEmptyPrompt(t *testing.T) {
	service := Service{
		db:     newFakeGenerationStore(),
		writer: newFakeGenerationStore(),
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{})
	if !errors.Is(err, ErrPromptRequired) {
		t.Fatalf("expected prompt required error, got %v", err)
	}
}

func TestGenerateRejectsConflictingRequestedSlug(t *testing.T) {
	store := newFakeGenerationStore()
	store.slugs["quiet-room-practice"] = true

	service := Service{
		db:     store,
		writer: store,
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "Quiet Room Practice",
		Slug:   "quiet-room-practice",
		Prompt: "A yoga studio website with classes and bookings.",
	})
	if !errors.Is(err, ErrSiteSlugConflict) {
		t.Fatalf("expected slug conflict, got %v", err)
	}

	failedCount := 0
	for _, job := range store.jobs {
		if job.Status == "failed" {
			failedCount++
		}
	}
	if failedCount != 1 {
		t.Fatalf("expected one failed job, got %#v", store.jobs)
	}
}

func TestGenerateRetriesPlannerAfterValidationFailure(t *testing.T) {
	store := newFakeGenerationStore()
	store.saveDraftErrors = []error{
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
	}
	feedbacks := []generationPlanFeedback{}

	service := Service{
		db:     store,
		writer: store,
		planner: func(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
			feedbacks = append(feedbacks, feedback)
			return buildGenerationPlan(input.NameHint, input.Prompt, input.PreferredLanguage), nil
		},
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if result.Draft.Site.ID == "" {
		t.Fatalf("expected saved draft after retry, got %#v", result.Draft.Site)
	}
	if len(feedbacks) != 2 {
		t.Fatalf("expected planner to run twice, got %#v", feedbacks)
	}
	if feedbacks[0].Attempt != 1 || len(feedbacks[0].ValidationIssues) != 0 {
		t.Fatalf("expected first attempt to have no validation feedback, got %#v", feedbacks[0])
	}
	if feedbacks[1].Attempt != 2 || len(feedbacks[1].ValidationIssues) != 1 {
		t.Fatalf("expected second attempt to receive validation issues, got %#v", feedbacks[1])
	}
	if feedbacks[1].ValidationIssues[0].Code != "required" {
		t.Fatalf("expected validation issue code to be forwarded, got %#v", feedbacks[1].ValidationIssues)
	}
	if store.saveDraftCalls != 2 {
		t.Fatalf("expected draft save to retry once, got %d calls", store.saveDraftCalls)
	}
	if store.summary[result.Draft.Site.ID]["validationRetryCount"] != float64(1) {
		t.Fatalf("expected summary to track validation retry count, got %#v", store.summary[result.Draft.Site.ID])
	}
}

func TestGenerateUsesExtendedMVPBlocksWhenPromptCallsForThem(t *testing.T) {
	store := newFakeGenerationStore()

	service := Service{
		db:     store,
		writer: store,
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A photography studio website with a gallery, pricing, testimonials, team bios, and FAQ for new clients.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	blockTypes := map[string]bool{}
	for _, page := range result.Draft.Pages {
		for _, block := range page.Blocks {
			blockTypes[block.Type] = true
		}
	}

	for _, blockType := range []string{
		"gallery",
		"pricing_packages",
		"testimonials",
		"team_profile_cards",
		"faq",
		"contact_form",
		"footer",
	} {
		if !blockTypes[blockType] {
			t.Fatalf("expected generated draft to include %s, got %#v", blockType, blockTypes)
		}
	}
}

func TestRepromptSiteReplacesDraftAndCapturesUndoRevision(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	result, err := service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it warmer, add pricing, and include a proper contact form.",
	})
	if err != nil {
		t.Fatalf("reprompt site: %v", err)
	}

	if result.Draft.Site.ID != initial.Draft.Site.ID {
		t.Fatalf("expected site identity to stay stable, got %#v", result.Draft.Site)
	}
	if len(store.revisions) != 2 {
		t.Fatalf("expected before/after revisions, got %#v", store.revisions)
	}
	if store.revisions[0].Draft.Site.ID != initial.Draft.Site.ID {
		t.Fatalf("expected captured draft revision to match initial draft, got %#v", store.revisions[0])
	}
	if len(store.repromptHistory) != 1 || store.repromptHistory[0].Scope != "site" {
		t.Fatalf("expected site reprompt history entry, got %#v", store.repromptHistory)
	}
	if store.prompts[result.Draft.Site.ID] != "Make it warmer, add pricing, and include a proper contact form." {
		t.Fatalf("expected prompt metadata to update, got %#v", store.prompts)
	}
}

func TestRepromptPageReplacesOnlySelectedPage(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	targetPage := initial.Draft.Pages[1]
	unchangedPage := initial.Draft.Pages[0]

	result, err := service.RepromptPage(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, targetPage.ID, RepromptInput{
		Prompt: "Turn this page into a pricing overview with clearer packages.",
	})
	if err != nil {
		t.Fatalf("reprompt page: %v", err)
	}

	if result.Draft.Pages[0].ID != unchangedPage.ID {
		t.Fatalf("expected non-target pages to stay in place, got %#v", result.Draft.Pages)
	}
	updatedPage := result.Draft.Pages[1]
	if updatedPage.ID != targetPage.ID || updatedPage.Slug != targetPage.Slug {
		t.Fatalf("expected targeted page identity to stay stable, got %#v", updatedPage)
	}
	if len(store.revisions) != 2 || store.revisions[0].PageID != targetPage.ID {
		t.Fatalf("expected page-scoped draft revision, got %#v", store.revisions)
	}
	if len(store.repromptHistory) != 1 || store.repromptHistory[0].TargetID != targetPage.ID {
		t.Fatalf("expected page reprompt history entry, got %#v", store.repromptHistory)
	}
}

type fakeBlockSuggester struct {
	request  BlockSuggestRequest
	response BlockSuggestResponse
	err      error
}

func (f *fakeBlockSuggester) SuggestBlockProps(_ context.Context, request BlockSuggestRequest) (BlockSuggestResponse, error) {
	f.request = request
	return f.response, f.err
}

type fakeChangeSetPlanner struct {
	request  PageChangeSetRequest
	response PageChangeSetResponse
	err      error
}

func (f *fakeChangeSetPlanner) PlanPageChanges(_ context.Context, request PageChangeSetRequest) (PageChangeSetResponse, error) {
	f.request = request
	return f.response, f.err
}

type dynamicChangeSetPlanner struct {
	requests []PageChangeSetRequest
}

func (p *dynamicChangeSetPlanner) PlanPageChanges(_ context.Context, request PageChangeSetRequest) (PageChangeSetResponse, error) {
	p.requests = append(p.requests, request)
	operations := make([]PageChangeSetOperation, 0, len(request.Page.Blocks))
	for index, block := range request.Page.Blocks {
		action := PageChangeSetActionKeep
		purpose := ""
		if index == 0 {
			action = PageChangeSetActionEdit
			purpose = "Refresh the lead section to match the latest direction."
		}
		operations = append(operations, PageChangeSetOperation{
			Action:  action,
			BlockID: block.BlockID,
			Purpose: purpose,
		})
	}
	return PageChangeSetResponse{
		Operations:    operations,
		ChangeSummary: "Refined the page without replacing untouched sections.",
	}, nil
}

type fakeClarifyingPlanner struct {
	request   ClarifyingQuestionsRequest
	questions []ClarifyingQuestion
	err       error
}

func (f *fakeClarifyingPlanner) BuildClarifyingQuestions(_ context.Context, request ClarifyingQuestionsRequest) ([]ClarifyingQuestion, error) {
	f.request = request
	return f.questions, f.err
}

type fakeDecomposedPlanner struct {
	outlineRequest OutlineRequest
	outline        OutlineResult
	outlineErr     error

	layoutRequests []PageLayoutRequest
	layout         PageLayoutResult
	layoutErr      error

	contentRequests []PageContentRequest
	content         PageContentResult
	contentErr      error
}

func (f *fakeDecomposedPlanner) BuildOutline(_ context.Context, request OutlineRequest) (OutlineResult, error) {
	f.outlineRequest = request
	return f.outline, f.outlineErr
}

func (f *fakeDecomposedPlanner) BuildPageLayout(_ context.Context, request PageLayoutRequest) (PageLayoutResult, error) {
	f.layoutRequests = append(f.layoutRequests, request)
	return f.layout, f.layoutErr
}

func (f *fakeDecomposedPlanner) BuildPageContent(_ context.Context, request PageContentRequest) (PageContentResult, error) {
	f.contentRequests = append(f.contentRequests, request)
	return f.content, f.contentErr
}

func TestBuildInterviewQuestionsRequiresPrompt(t *testing.T) {
	service := Service{}
	if _, err := service.BuildInterviewQuestions(context.Background(), GenerateInput{}); !errors.Is(err, ErrPromptRequired) {
		t.Fatalf("expected ErrPromptRequired, got %v", err)
	}
}

func TestBuildInterviewQuestionsReturnsEmptyWhenPlannerMissing(t *testing.T) {
	service := Service{}
	questions, err := service.BuildInterviewQuestions(context.Background(), GenerateInput{Prompt: "A photo studio site."})
	if err != nil {
		t.Fatalf("expected no error when planner missing, got %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("expected zero questions when planner missing, got %#v", questions)
	}
}

func TestBuildInterviewQuestionsCapsAtMax(t *testing.T) {
	planner := &fakeClarifyingPlanner{
		questions: []ClarifyingQuestion{
			{ID: "q1", Prompt: "?", Kind: ClarifyingQuestionKindSingle},
			{ID: "q2", Prompt: "?", Kind: ClarifyingQuestionKindSingle},
			{ID: "q3", Prompt: "?", Kind: ClarifyingQuestionKindSingle},
			{ID: "q4", Prompt: "?", Kind: ClarifyingQuestionKindSingle},
		},
	}
	service := Service{clarifyingPlanner: planner}
	questions, err := service.BuildInterviewQuestions(context.Background(), GenerateInput{Prompt: "A photo studio site."})
	if err != nil {
		t.Fatalf("interview: %v", err)
	}
	if len(questions) != MaxClarifyingQuestions {
		t.Fatalf("expected interview to cap at %d questions, got %d", MaxClarifyingQuestions, len(questions))
	}
}

type recordingBlockSuggester struct {
	requests []BlockSuggestRequest
	response BlockSuggestResponse
}

func (r *recordingBlockSuggester) SuggestBlockProps(_ context.Context, request BlockSuggestRequest) (BlockSuggestResponse, error) {
	r.requests = append(r.requests, request)
	return r.response, nil
}

type transformBlockSuggester struct {
	requests []BlockSuggestRequest
}

func (s *transformBlockSuggester) SuggestBlockProps(_ context.Context, request BlockSuggestRequest) (BlockSuggestResponse, error) {
	s.requests = append(s.requests, request)

	props := deepCloneProps(request.Block.Props)
	for _, key := range []string{"headline", "heading", "subheadline", "body", "eyebrow"} {
		value, ok := props[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		props[key] = value + " Refined."
		return BlockSuggestResponse{
			Props:         props,
			ChangeSummary: "Refined the lead section.",
		}, nil
	}

	props["headline"] = "Refined section"
	return BlockSuggestResponse{
		Props:         props,
		ChangeSummary: "Refined the lead section.",
	}, nil
}

func TestRepromptPageUsesWholePageLayoutAndRegeneratesBlockIDs(t *testing.T) {
	store := newFakeGenerationStore()
	planner := &fakeDecomposedPlanner{
		layout: PageLayoutResult{
			Blocks: []PageLayoutBlock{
				{Type: "hero", Purpose: "Rewrite the opener.", ContentBrief: "Sharper promise.", VariantHint: "standard"},
				{Type: "text_section", Purpose: "Support the new direction.", ContentBrief: "Short proof paragraph.", VariantHint: "default"},
			},
		},
		content: PageContentResult{
			Blocks: []PageContentBlock{
				{
					Type: "hero",
					Props: map[string]any{
						"variant":     "standard",
						"headline":    "Rewritten headline",
						"subheadline": "Reflects the page reprompt.",
						"layout":      "centered",
					},
				},
				{
					Type: "text_section",
					Props: map[string]any{
						"heading":   "What changed",
						"body":      "The whole page was regenerated from one selected layout.",
						"alignment": "left",
						"width":     "default",
					},
				},
			},
		},
	}
	service := Service{
		db:                store,
		reader:            store,
		writer:            store,
		decomposedPlanner: planner,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	homePage := initial.Draft.Pages[0]
	if len(homePage.Blocks) < 2 {
		t.Fatalf("expected at least two blocks on the home page, got %d", len(homePage.Blocks))
	}
	heroID := homePage.Blocks[0].ID
	secondID := homePage.Blocks[1].ID

	result, err := service.RepromptPage(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, homePage.ID, RepromptInput{
		Prompt: "Punch up the hero headline.",
	})
	if err != nil {
		t.Fatalf("reprompt page: %v", err)
	}

	updatedPage := result.Draft.Pages[0]
	if len(updatedPage.Blocks) != 2 {
		t.Fatalf("expected page layout/content to produce 2 blocks, got %d", len(updatedPage.Blocks))
	}
	if updatedPage.Blocks[0].ID == heroID || updatedPage.Blocks[1].ID == secondID {
		t.Fatalf("expected whole-page rewrite to regenerate block ids, got %#v from %#v", updatedPage.Blocks, homePage.Blocks)
	}
	if updatedPage.Blocks[0].Props["headline"] != "Rewritten headline" {
		t.Fatalf("expected hero props to be rewritten, got %#v", updatedPage.Blocks[0].Props)
	}
	if len(planner.layoutRequests) != 1 {
		t.Fatalf("expected one page layout request, got %d", len(planner.layoutRequests))
	}
	if len(planner.contentRequests) != 1 {
		t.Fatalf("expected one page content request, got %d", len(planner.contentRequests))
	}
	if len(planner.contentRequests[0].Layout) != 2 || planner.contentRequests[0].Layout[0].Type != "hero" {
		t.Fatalf("expected content request to receive selected layout, got %#v", planner.contentRequests[0].Layout)
	}
	if planner.layoutRequests[0].Page.Slug != homePage.Slug {
		t.Fatalf("expected layout request to target current page, got %#v", planner.layoutRequests[0].Page)
	}
}

func TestRepromptPageUsesChangeSetAndPreservesUntouchedBlockIDs(t *testing.T) {
	store := newFakeGenerationStore()
	changeSetPlanner := &dynamicChangeSetPlanner{}
	suggester := &transformBlockSuggester{}
	decomposedPlanner := &fakeDecomposedPlanner{}
	service := Service{
		db:                   store,
		reader:               store,
		writer:               store,
		suggester:            suggester,
		pageChangeSetPlanner: changeSetPlanner,
		decomposedPlanner:    decomposedPlanner,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	targetPage := initial.Draft.Pages[0]
	if len(targetPage.Blocks) < 2 {
		t.Fatalf("expected at least two blocks on the target page, got %d", len(targetPage.Blocks))
	}
	originalFirstHeadline, _ := targetPage.Blocks[0].Props["headline"].(string)

	result, err := service.RepromptPage(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, targetPage.ID, RepromptInput{
		Prompt: "Make the opening stronger while keeping the rest intact.",
	})
	if err != nil {
		t.Fatalf("reprompt page: %v", err)
	}

	updatedPage := result.Draft.Pages[0]
	if len(updatedPage.Blocks) != len(targetPage.Blocks) {
		t.Fatalf("expected change-set reprompt to keep block count stable, got %d from %d", len(updatedPage.Blocks), len(targetPage.Blocks))
	}
	if updatedPage.Blocks[0].ID != targetPage.Blocks[0].ID {
		t.Fatalf("expected edited block id to stay stable, got %#v", updatedPage.Blocks[0])
	}
	if updatedPage.Blocks[0].Props["headline"] == originalFirstHeadline {
		t.Fatalf("expected edited block copy to change, got %#v", updatedPage.Blocks[0].Props)
	}
	for index := 1; index < len(targetPage.Blocks); index++ {
		if updatedPage.Blocks[index].ID != targetPage.Blocks[index].ID {
			t.Fatalf("expected untouched block %d to keep its id, got %#v", index, updatedPage.Blocks[index])
		}
	}
	if len(changeSetPlanner.requests) != 1 {
		t.Fatalf("expected one change-set planning request, got %d", len(changeSetPlanner.requests))
	}
	if len(suggester.requests) != 1 {
		t.Fatalf("expected one rewritten block, got %d", len(suggester.requests))
	}
	if len(decomposedPlanner.layoutRequests) != 0 || len(decomposedPlanner.contentRequests) != 0 {
		t.Fatalf("expected whole-page generation path to be skipped, got %#v %#v", decomposedPlanner.layoutRequests, decomposedPlanner.contentRequests)
	}
	if len(store.repromptHistory) != 1 || store.repromptHistory[0].ChangeSummary != "Refined the page without replacing untouched sections." {
		t.Fatalf("expected change-set summary to reach history, got %#v", store.repromptHistory)
	}
}

func TestRepromptPageFallsBackWhenChangeSetEmpty(t *testing.T) {
	store := newFakeGenerationStore()
	planner := &fakeChangeSetPlanner{response: PageChangeSetResponse{}}
	suggester := &recordingBlockSuggester{
		response: BlockSuggestResponse{Props: map[string]any{"headline": "x"}},
	}
	service := Service{
		db:                   store,
		reader:               store,
		writer:               store,
		suggester:            suggester,
		pageChangeSetPlanner: planner,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := service.RepromptPage(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, initial.Draft.Pages[1].ID, RepromptInput{
		Prompt: "Make this page about pricing.",
	}); err != nil {
		t.Fatalf("reprompt page: %v", err)
	}
	if len(suggester.requests) != 0 {
		t.Fatalf("expected fallback path to skip per-block rewrites, got %d", len(suggester.requests))
	}
}

func TestRepromptSiteRewritesMatchingPagesInsteadOfCopyingBlocksVerbatim(t *testing.T) {
	store := newFakeGenerationStore()
	changeSetPlanner := &dynamicChangeSetPlanner{}
	suggester := &transformBlockSuggester{}
	decomposedPlanner := &fakeDecomposedPlanner{}
	service := Service{
		db:                   store,
		reader:               store,
		writer:               store,
		suggester:            suggester,
		pageChangeSetPlanner: changeSetPlanner,
		decomposedPlanner:    decomposedPlanner,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	outlinePages := make([]OutlinePage, 0, len(initial.Draft.Pages))
	initialPageIDs := make(map[string]string, len(initial.Draft.Pages))
	initialHeadlines := make(map[string]string, len(initial.Draft.Pages))
	for _, page := range initial.Draft.Pages {
		outlinePages = append(outlinePages, OutlinePage{
			Title: page.Title,
			Slug:  page.Slug,
			Goal:  "Refresh the existing copy for this page.",
			SEO:   page.SEO,
		})
		initialPageIDs[page.Slug] = page.ID
		if len(page.Blocks) > 0 {
			initialHeadlines[page.Slug], _ = page.Blocks[0].Props["headline"].(string)
		}
	}
	decomposedPlanner.outline = OutlineResult{
		SiteName:       initial.Draft.Site.Name,
		SiteGoal:       initial.Draft.Site.SEO.Description,
		ThemeSelection: siteconfig.DetectThemeSelection(initial.Draft.Theme),
		Pages:          outlinePages,
	}

	result, err := service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make the whole site feel more premium without throwing away the current structure.",
	})
	if err != nil {
		t.Fatalf("reprompt site: %v", err)
	}

	if len(changeSetPlanner.requests) != len(initial.Draft.Pages) {
		t.Fatalf("expected one change-set plan per existing page, got %d", len(changeSetPlanner.requests))
	}
	if len(decomposedPlanner.layoutRequests) != 0 || len(decomposedPlanner.contentRequests) != 0 {
		t.Fatalf("expected existing pages to avoid full page regeneration, got %#v %#v", decomposedPlanner.layoutRequests, decomposedPlanner.contentRequests)
	}
	if len(suggester.requests) != len(initial.Draft.Pages) {
		t.Fatalf("expected one block rewrite per page, got %d", len(suggester.requests))
	}

	for _, page := range result.Draft.Pages {
		if page.ID != initialPageIDs[page.Slug] {
			t.Fatalf("expected preserved page identity for %s, got %#v", page.Slug, page)
		}
		if len(page.Blocks) == 0 {
			t.Fatalf("expected revised page blocks for %s", page.Slug)
		}
		if page.Blocks[0].ID == "" {
			t.Fatalf("expected revised page to keep a real first block id for %s", page.Slug)
		}
		if before := initialHeadlines[page.Slug]; before != "" && page.Blocks[0].Props["headline"] == before {
			t.Fatalf("expected first block copy to change for %s, got %#v", page.Slug, page.Blocks[0].Props)
		}
	}
}

func TestSuggestBlockRewritesPropsAndRecordsHistory(t *testing.T) {
	store := newFakeGenerationStore()
	suggester := &fakeBlockSuggester{
		response: BlockSuggestResponse{
			Props: map[string]any{
				"variant":     "standard",
				"headline":    "Calm photography for real moments",
				"subheadline": "A tighter take on the hero copy.",
				"eyebrow":     "Photography studio",
				"layout":      "split-left",
			},
			ChangeSummary: "Tightened the hero headline.",
		},
	}
	service := Service{
		db:        store,
		reader:    store,
		writer:    store,
		suggester: suggester,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	targetPage := initial.Draft.Pages[0]
	targetBlock := targetPage.Blocks[0]
	if targetBlock.Type != "hero" {
		t.Fatalf("expected first block to be the hero; got %s", targetBlock.Type)
	}

	result, err := service.SuggestBlock(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, targetBlock.ID, BlockSuggestInput{
		Action: BlockSuggestActionTighten,
	})
	if err != nil {
		t.Fatalf("suggest block: %v", err)
	}

	updatedBlock := result.Draft.Pages[0].Blocks[0]
	if updatedBlock.ID != targetBlock.ID {
		t.Fatalf("expected block id to stay stable, got %#v", updatedBlock)
	}
	if updatedBlock.Type != targetBlock.Type {
		t.Fatalf("expected block type to stay stable, got %s", updatedBlock.Type)
	}
	if updatedBlock.Props["headline"] != "Calm photography for real moments" {
		t.Fatalf("expected suggested headline to be applied, got %#v", updatedBlock.Props)
	}
	if suggester.request.Block.ID != targetBlock.ID {
		t.Fatalf("expected suggester to receive the target block, got %#v", suggester.request)
	}
	if suggester.request.Action != BlockSuggestActionTighten {
		t.Fatalf("expected action to reach suggester, got %#v", suggester.request)
	}
	if suggester.request.Definition.Type != "hero" {
		t.Fatalf("expected hero definition to reach suggester, got %#v", suggester.request.Definition)
	}
	if len(store.revisions) != 2 {
		t.Fatalf("expected before/after draft revisions, got %#v", store.revisions)
	}
	if store.revisions[0].Scope != "block" || store.revisions[0].PageID != targetPage.ID {
		t.Fatalf("expected first revision to be block-scoped on the target page, got %#v", store.revisions[0])
	}
	if len(store.repromptHistory) != 1 {
		t.Fatalf("expected reprompt history entry, got %#v", store.repromptHistory)
	}
	entry := store.repromptHistory[0]
	if entry.Scope != "block" {
		t.Fatalf("expected block-scoped reprompt history, got %#v", entry)
	}
	if entry.TargetID != targetBlock.ID {
		t.Fatalf("expected reprompt history target to be the block id, got %#v", entry)
	}
	if entry.ChangeSummary != "Tightened the hero headline." {
		t.Fatalf("expected model summary to land in history, got %#v", entry)
	}
}

func TestSuggestBlockRequiresConfiguredSuggester(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{db: store, reader: store, writer: store}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.SuggestBlock(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, initial.Draft.Pages[0].Blocks[0].ID, BlockSuggestInput{Action: BlockSuggestActionTighten})
	if !errors.Is(err, ErrBlockSuggestUnavailable) {
		t.Fatalf("expected ErrBlockSuggestUnavailable, got %v", err)
	}
}

func TestSuggestBlockRejectsUnknownAction(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{db: store, reader: store, writer: store, suggester: &fakeBlockSuggester{}}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.SuggestBlock(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, initial.Draft.Pages[0].Blocks[0].ID, BlockSuggestInput{Action: "explode"})
	if !errors.Is(err, ErrBlockSuggestActionUnknown) {
		t.Fatalf("expected ErrBlockSuggestActionUnknown, got %v", err)
	}
}

func TestSuggestBlockRequiresToneForToneAction(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{db: store, reader: store, writer: store, suggester: &fakeBlockSuggester{}}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.SuggestBlock(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, initial.Draft.Pages[0].Blocks[0].ID, BlockSuggestInput{Action: BlockSuggestActionTone})
	if !errors.Is(err, ErrBlockSuggestToneRequired) {
		t.Fatalf("expected ErrBlockSuggestToneRequired, got %v", err)
	}
}

func TestSuggestBlockReturnsNotFoundForMissingBlock(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{db: store, reader: store, writer: store, suggester: &fakeBlockSuggester{response: BlockSuggestResponse{Props: map[string]any{"headline": "x"}}}}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.SuggestBlock(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, "block-does-not-exist", BlockSuggestInput{Action: BlockSuggestActionTighten})
	if !errors.Is(err, ErrBlockSuggestNotFound) {
		t.Fatalf("expected ErrBlockSuggestNotFound, got %v", err)
	}
}

func TestBuildPageRepromptPlanPrefersModelPlanOverTemplateFallback(t *testing.T) {
	service := Service{
		planner: func(_ context.Context, _ generationInputContext, _ generationPlanFeedback) (generationPlan, error) {
			return generationPlan{
				Pages: []generationPagePlan{{
					Title: "AI Pricing",
					Slug:  "/pricing",
					SEO: siteconfig.SEOConfig{
						Title:       "AI pricing",
						Description: "Model-authored pricing page",
					},
					Blocks: []generationBlockPlan{{
						Type:    "pricing_packages",
						Purpose: "Present clear package options",
						Props: map[string]any{
							"heading": "Straightforward packages",
							"plans": []any{
								map[string]any{
									"name":        "Starter",
									"price":       "$500",
									"description": "Short engagement",
									"features":    []any{map[string]any{"text": "One main deliverable"}},
								},
							},
						},
					}},
				}},
			}, nil
		},
	}

	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:   "site-1",
			Name: "North Light Studio",
			Slug: "north-light-studio",
			SEO:  siteconfig.SEOConfig{Description: "Studio site"},
		},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-1",
			Title: "Services",
			Slug:  "/services",
		}},
	}

	plan, err := service.buildPageRepromptPlan(context.Background(), draft, draft.Pages[0], "Turn this into a pricing page.")
	if err != nil {
		t.Fatalf("build page reprompt plan: %v", err)
	}
	if plan.Title != "AI Pricing" {
		t.Fatalf("expected model-authored title to win, got %#v", plan)
	}
	if len(plan.Blocks) != 1 || plan.Blocks[0].Type != "pricing_packages" {
		t.Fatalf("expected model-authored blocks to win, got %#v", plan.Blocks)
	}
	if plan.Slug != "/services" {
		t.Fatalf("expected page slug identity to stay stable, got %#v", plan)
	}
}

func TestUndoLastDraftRevisionRestoresPreviousDraft(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it darker and add pricing.",
	})
	if err != nil {
		t.Fatalf("reprompt site: %v", err)
	}

	restored, err := service.UndoLastDraftRevision(context.Background(), "workspace-1", initial.Draft.Site.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	if restored.Site.ID != initial.Draft.Site.ID || restored.Site.Slug != initial.Draft.Site.Slug {
		t.Fatalf("expected restored draft identity to match initial draft, got %#v", restored.Site)
	}
	if len(store.revisions) != 2 {
		t.Fatalf("expected immutable revisions to remain after undo, got %#v", store.revisions)
	}
	if store.prompts[initial.Draft.Site.ID] != "A calm portfolio site for a photography studio that needs a gallery." {
		t.Fatalf("expected prompt metadata to restore, got %#v", store.prompts)
	}
	if len(store.repromptHistory) != 1 || store.repromptHistory[0].UndoneAt == nil {
		t.Fatalf("expected reprompt history entry to be marked undone, got %#v", store.repromptHistory)
	}
}

func TestGenerateFailsAfterValidationRetryExhausted(t *testing.T) {
	store := newFakeGenerationStore()
	store.saveDraftErrors = []error{
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
	}
	planCalls := 0

	service := Service{
		db:     store,
		writer: store,
		planner: func(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
			planCalls++
			return buildGenerationPlan(input.NameHint, input.Prompt, input.PreferredLanguage), nil
		},
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error after retry exhaustion, got %v", err)
	}
	if planCalls != 2 {
		t.Fatalf("expected two planner attempts, got %d", planCalls)
	}
	if store.saveDraftCalls != 2 {
		t.Fatalf("expected two draft save attempts, got %d", store.saveDraftCalls)
	}
	for _, job := range store.jobs {
		if job.Status != "failed" {
			continue
		}
		issues, ok := job.Error["issues"].([]any)
		if !ok || len(issues) != 1 {
			t.Fatalf("expected failed job to include validation issues, got %#v", job.Error)
		}
		return
	}
	t.Fatalf("expected failed generation job, got %#v", store.jobs)
}

func TestGenerateFailsWhenMetadataPersistenceFails(t *testing.T) {
	store := newFakeGenerationStore()
	store.siteUpdateErr = errors.New("write generation summary")
	service := Service{
		db:     store,
		writer: store,
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err == nil || !strings.Contains(err.Error(), "write generation summary") {
		t.Fatalf("expected metadata persistence error, got %v", err)
	}
	if store.jobs["job-1"].Status != "failed" {
		t.Fatalf("expected job to be marked failed, got %#v", store.jobs["job-1"])
	}
}

func TestRepromptSiteFailsWhenJobCompletionFails(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	store.completeJobErr = errors.New("update generation job")
	_, err = service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it warmer and add pricing.",
	})
	if err == nil || !strings.Contains(err.Error(), "update generation job") {
		t.Fatalf("expected job completion error, got %v", err)
	}
	if store.jobs["job-2"].Status != "failed" {
		t.Fatalf("expected second job to be marked failed, got %#v", store.jobs["job-2"])
	}
}

func TestRepairGenerationPlanRepairsSafeIssues(t *testing.T) {
	pages := make([]generationPagePlan, 0, siteconfig.MaxPagesPerSite+2)
	for index := 0; index < siteconfig.MaxPagesPerSite+2; index++ {
		pages = append(pages, generationPagePlan{
			Title: fmt.Sprintf("Page %d", index+1),
			Slug:  "/duplicate",
			Goal:  "<script>alert(1)</script>Explain the offer clearly.",
			Blocks: []generationBlockPlan{
				{
					Type: "hero",
					Props: map[string]any{
						"headline":    fmt.Sprintf("<b>Headline %d</b>", index+1),
						"subheadline": "<p>Structured copy without raw markup.</p>",
						"primaryCta": map[string]any{
							"label": "<span>Get started</span>",
							"href":  "javascript:alert(1)",
						},
						"layout": "unknown",
					},
				},
				{
					Type: "script_embed",
					Props: map[string]any{
						"code": "<script>alert(1)</script>",
					},
				},
			},
		})
	}

	repaired := repairGenerationPlan(generationPlan{
		SiteName:    "<strong>North Light Studio</strong>",
		SiteGoal:    "<script>bad()</script>Turn visitors into confident inquiries.",
		ThemePreset: "unknown-theme",
		Theme: siteconfig.ThemeConfig{
			Version: "theme.v0",
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "warm",
				},
			},
		},
		Pages:        pages,
		AssetsNeeded: []string{"hero-image", "hero-image", "javascript:alert(1)", "supporting-image"},
		Assumptions: []string{
			"<p>Default locale is English.</p>",
			"<p>Default locale is English.</p>",
		},
	}, "")

	if repaired.SiteName != "North Light Studio" {
		t.Fatalf("expected sanitized site name, got %q", repaired.SiteName)
	}
	if repaired.SiteGoal != "Turn visitors into confident inquiries." {
		t.Fatalf("expected sanitized site goal, got %q", repaired.SiteGoal)
	}
	if repaired.ThemePreset != siteconfig.ThemePaletteCleanLocal {
		t.Fatalf("expected fallback theme preset, got %q", repaired.ThemePreset)
	}
	if len(repaired.AssetsNeeded) != 2 {
		t.Fatalf("expected repaired assets list, got %#v", repaired.AssetsNeeded)
	}
	if len(repaired.Assumptions) != 1 || repaired.Assumptions[0] != "Default locale is English." {
		t.Fatalf("expected deduplicated assumptions, got %#v", repaired.Assumptions)
	}
	if len(repaired.Pages) != siteconfig.MaxPagesPerSite {
		t.Fatalf("expected repaired page count to be capped, got %d", len(repaired.Pages))
	}
	if repaired.Pages[0].Slug != "/" {
		t.Fatalf("expected homepage slug, got %q", repaired.Pages[0].Slug)
	}
	seenSlugs := map[string]bool{}
	for index, page := range repaired.Pages {
		if seenSlugs[page.Slug] {
			t.Fatalf("expected unique slugs, got duplicate %q", page.Slug)
		}
		seenSlugs[page.Slug] = true
		if strings.Contains(page.Goal, "<") {
			t.Fatalf("expected sanitized page goal, got %q", page.Goal)
		}
		if len(page.Blocks) == 0 {
			t.Fatalf("expected repaired blocks on page %d", index)
		}
		for _, block := range page.Blocks {
			if block.Type == "script_embed" {
				t.Fatalf("expected unsupported block to be removed, got %#v", block)
			}
			if strings.Contains(fmt.Sprint(block.Props), "<") {
				t.Fatalf("expected sanitized props, got %#v", block.Props)
			}
		}
	}

	draft, err := buildDraftFromPlan(repaired, "north-light-studio", "en", siteconfig.BrandConfig{})
	if err != nil {
		t.Fatalf("expected repaired plan to build, got %v", err)
	}
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("expected repaired draft to validate, got %v", err)
	}
}

type fakeGenerationStore struct {
	drafts          map[string]siteconfig.SiteDraft
	jobs            map[string]fakeGenerationJob
	slugs           map[string]bool
	prompts         map[string]string
	summary         map[string]map[string]any
	summaryJSON     map[string][]byte
	revisions       []draftRevisionRecord
	repromptHistory []RepromptHistoryEntry
	saveDraftErrors []error
	saveDraftCalls  int
	completeJobErr  error
	siteUpdateErr   error
}

type fakeGenerationJob struct {
	ID     string
	SiteID string
	Status string
	Output generationPlan
	Error  map[string]any
}

func newFakeGenerationStore() *fakeGenerationStore {
	return &fakeGenerationStore{
		drafts:      map[string]siteconfig.SiteDraft{},
		jobs:        map[string]fakeGenerationJob{},
		slugs:       map[string]bool{},
		prompts:     map[string]string{},
		summary:     map[string]map[string]any{},
		summaryJSON: map[string][]byte{},
	}
}

func (s *fakeGenerationStore) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	s.saveDraftCalls++
	if len(s.saveDraftErrors) > 0 {
		err := s.saveDraftErrors[0]
		s.saveDraftErrors = s.saveDraftErrors[1:]
		if err != nil {
			return err
		}
	}
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return err
	}
	s.drafts[draft.Site.ID] = draft
	s.slugs[draft.Site.Slug] = true
	return nil
}

func (s *fakeGenerationStore) LoadDraft(_ context.Context, siteID string) (siteconfig.SiteDraft, error) {
	draft, ok := s.drafts[siteID]
	if !ok {
		return siteconfig.SiteDraft{}, sites.ErrNotFound
	}
	return draft, nil
}

func (s *fakeGenerationStore) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "from guest_sessions"):
		return &fakeGenerationRows{rows: [][]any{}}, nil
	case strings.Contains(sql, "from reprompt_history"):
		rows := make([][]any, 0, len(s.repromptHistory))
		for _, entry := range s.repromptHistory {
			rows = append(rows, []any{
				entry.ID,
				entry.Scope,
				entry.TargetID,
				entry.Prompt,
				entry.ChangeSummary,
				entry.PreviousRevision,
				entry.ResultRevision,
				entry.JobID,
				entry.CreatedAt,
				entry.UndoneAt,
			})
		}
		return &fakeGenerationRows{rows: rows}, nil
	default:
		return &fakeGenerationRows{err: errors.New("not implemented")}, nil
	}
}

func (s *fakeGenerationStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into generation_jobs"):
		jobID := "job-1"
		if len(s.jobs) > 0 {
			jobID = "job-2"
		}
		s.jobs[jobID] = fakeGenerationJob{ID: jobID, Status: "running"}
		return fakeGenerationRow{values: []any{jobID}}
	case strings.Contains(sql, "returning workspace_id::text"):
		if s.completeJobErr != nil {
			return fakeGenerationRow{err: s.completeJobErr}
		}
		job := s.jobs[args[2].(string)]
		job.SiteID = args[0].(string)
		job.Status = "completed"
		if err := json.Unmarshal(args[1].([]byte), &job.Output); err != nil {
			return fakeGenerationRow{err: err}
		}
		s.jobs[job.ID] = job
		return fakeGenerationRow{values: []any{"workspace-1"}}
	case strings.Contains(sql, "select subscription_live"):
		return fakeGenerationRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from billing_entitlements"):
		return fakeGenerationRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from billing_subscriptions"):
		return fakeGenerationRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from generation_jobs") && strings.Contains(sql, "count(*)"):
		return fakeGenerationRow{values: []any{0}}
	case strings.Contains(sql, "select exists("):
		return fakeGenerationRow{values: []any{s.slugs[args[1].(string)]}}
	case strings.Contains(sql, "select coalesce(generation_prompt, '')"):
		siteID := args[0].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return fakeGenerationRow{err: pgx.ErrNoRows}
		}
		return fakeGenerationRow{values: []any{s.prompts[siteID], s.summaryJSON[siteID]}}
	case strings.Contains(sql, "select id::text") && strings.Contains(sql, "from draft_revisions") && !strings.Contains(sql, "draft,"):
		if len(s.revisions) == 0 {
			return fakeGenerationRow{err: pgx.ErrNoRows}
		}
		return fakeGenerationRow{values: []any{s.revisions[len(s.revisions)-1].ID}}
	case strings.Contains(sql, "from draft_revisions"):
		revisionID := args[0].(string)
		for _, revision := range s.revisions {
			if revision.ID != revisionID {
				continue
			}
			draftJSON, err := json.Marshal(revision.Draft)
			if err != nil {
				return fakeGenerationRow{err: err}
			}
			return fakeGenerationRow{values: []any{
				revision.ID,
				revision.Scope,
				revision.PageID,
				revision.Prompt,
				draftJSON,
				revision.GenerationPrompt,
				revision.GenerationSummaryJSON,
				revision.CreatedAt,
			}}
		}
		return fakeGenerationRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from reprompt_history"):
		repromptID := args[0].(string)
		for _, entry := range s.repromptHistory {
			if entry.ID != repromptID {
				continue
			}
			return fakeGenerationRow{values: []any{
				entry.ID,
				entry.Scope,
				entry.TargetID,
				entry.Prompt,
				entry.ChangeSummary,
				entry.PreviousRevision,
				entry.ResultRevision,
				entry.JobID,
				entry.CreatedAt,
				entry.UndoneAt,
			}}
		}
		return fakeGenerationRow{err: pgx.ErrNoRows}
	default:
		return fakeGenerationRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeGenerationStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "state = 'running'"):
		job := s.jobs[args[2].(string)]
		job.Status = "running"
		s.jobs[job.ID] = job
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "status = 'completed'"):
		if s.completeJobErr != nil {
			return pgconn.CommandTag{}, s.completeJobErr
		}
		job := s.jobs[args[2].(string)]
		job.SiteID = args[0].(string)
		job.Status = "completed"
		if err := json.Unmarshal(args[1].([]byte), &job.Output); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.jobs[job.ID] = job
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "status = 'failed'"):
		job := s.jobs[args[2].(string)]
		job.Status = "failed"
		if err := json.Unmarshal(args[0].([]byte), &job.Error); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.jobs[job.ID] = job
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update guest_sessions"):
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case strings.Contains(sql, "update sites"):
		if s.siteUpdateErr != nil {
			return pgconn.CommandTag{}, s.siteUpdateErr
		}
		siteID := args[2].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		s.prompts[siteID] = args[0].(string)
		var summary map[string]any
		if err := json.Unmarshal(args[1].([]byte), &summary); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.summary[siteID] = summary
		s.summaryJSON[siteID] = append([]byte(nil), args[1].([]byte)...)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "insert into draft_revisions"):
		draftJSON := args[6].([]byte)
		var draft siteconfig.SiteDraft
		if err := json.Unmarshal(draftJSON, &draft); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.revisions = append(s.revisions, draftRevisionRecord{
			ID:                    args[0].(string),
			Scope:                 args[3].(string),
			PageID:                args[4].(string),
			Prompt:                args[5].(string),
			Draft:                 draft,
			GenerationPrompt:      args[7].(string),
			GenerationSummaryJSON: append([]byte(nil), args[8].([]byte)...),
			CreatedAt:             time.Now().UTC(),
		})
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(sql, "insert into reprompt_history"):
		entry := RepromptHistoryEntry{
			ID:               args[0].(string),
			Scope:            args[3].(string),
			TargetID:         args[4].(string),
			Prompt:           args[5].(string),
			PreviousRevision: args[6].(string),
			ResultRevision:   args[7].(string),
			JobID:            args[8].(string),
			ChangeSummary:    args[9].(string),
			CreatedAt:        time.Now().UTC(),
		}
		s.repromptHistory = append([]RepromptHistoryEntry{entry}, s.repromptHistory...)
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(sql, "update reprompt_history"):
		repromptID := args[0].(string)
		now := time.Now().UTC()
		for index := range s.repromptHistory {
			if s.repromptHistory[index].ID != repromptID {
				continue
			}
			s.repromptHistory[index].UndoneAt = &now
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case strings.Contains(sql, "delete from draft_revisions"):
		if len(s.revisions) > 0 {
			s.revisions = s.revisions[:len(s.revisions)-1]
		}
		return pgconn.NewCommandTag("DELETE 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

func (s *fakeGenerationStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return &fakeGenerationTx{store: s}, nil
}

type fakeGenerationTx struct {
	store *fakeGenerationStore
}

func (tx *fakeGenerationTx) Begin(context.Context) (pgx.Tx, error) {
	return tx, nil
}

func (tx *fakeGenerationTx) Commit(context.Context) error {
	return nil
}

func (tx *fakeGenerationTx) Rollback(context.Context) error {
	return nil
}

func (tx *fakeGenerationTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}

func (tx *fakeGenerationTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeGenerationTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeGenerationTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}

func (tx *fakeGenerationTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.store.Exec(ctx, sql, args...)
}

func (tx *fakeGenerationTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return tx.store.Query(ctx, sql, args...)
}

func (tx *fakeGenerationTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.store.QueryRow(ctx, sql, args...)
}

func (tx *fakeGenerationTx) Conn() *pgx.Conn {
	return nil
}

type fakeGenerationRow struct {
	values []any
	err    error
}

func (r fakeGenerationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *bool:
			*target = value.(bool)
		case *int:
			*target = value.(int)
		case *[]byte:
			*target = append([]byte(nil), value.([]byte)...)
		case *time.Time:
			*target = value.(time.Time)
		case **time.Time:
			if value == nil {
				*target = nil
				continue
			}
			timestamp := value.(*time.Time)
			if timestamp == nil {
				*target = nil
				continue
			}
			copyValue := *timestamp
			*target = &copyValue
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

type fakeGenerationRows struct {
	rows  [][]any
	index int
	err   error
}

func (r *fakeGenerationRows) Close() {}

func (r *fakeGenerationRows) Err() error { return r.err }

func (r *fakeGenerationRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *fakeGenerationRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *fakeGenerationRows) Next() bool {
	if r.err != nil {
		return false
	}
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}

func (r *fakeGenerationRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("scan called without row")
	}
	return fakeGenerationRow{values: r.rows[r.index-1]}.Scan(dest...)
}

func (r *fakeGenerationRows) Values() ([]any, error) {
	if r.index == 0 || r.index > len(r.rows) {
		return nil, errors.New("values called without row")
	}
	return r.rows[r.index-1], nil
}

func (r *fakeGenerationRows) RawValues() [][]byte { return nil }

func (r *fakeGenerationRows) Conn() *pgx.Conn { return nil }
