package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrPromptRequired   = errors.New("generation prompt is required")
	ErrSiteSlugInvalid  = errors.New("site slug is invalid")
	ErrSiteSlugConflict = errors.New("site slug is already in use")
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type draftWriter interface {
	SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error
}

type GenerateInput struct {
	Name   string
	Slug   string
	Prompt string
}

type GenerateResult struct {
	JobID string               `json:"jobId"`
	Draft siteconfig.SiteDraft `json:"draft"`
}

type Service struct {
	db     DB
	writer draftWriter
}

func NewService(db DB) *Service {
	return &Service{
		db:     db,
		writer: sites.NewPostgresWriter(db),
	}
}

func (s *Service) Generate(ctx context.Context, workspaceID string, userID string, input GenerateInput) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, ErrPromptRequired
	}

	inputContext := generationInputContext{
		NameHint: strings.TrimSpace(input.Name),
		SlugHint: strings.TrimSpace(input.Slug),
		Prompt:   prompt,
	}
	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, inputContext)
	if err != nil {
		return GenerateResult{}, err
	}

	plan := buildGenerationPlan(strings.TrimSpace(input.Name), prompt)
	slugValue, err := s.createSlug(ctx, workspaceID, strings.TrimSpace(input.Slug), plan.SiteName)
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	draft, err := buildDraftFromPlan(plan, slugValue)
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	metadataErr := s.saveSiteMetadata(ctx, workspaceID, draft.Site.ID, prompt, plan)
	jobErr := s.completeGenerationJob(ctx, jobID, draft.Site.ID, plan)
	if metadataErr != nil || jobErr != nil {
		return GenerateResult{
			JobID: jobID,
			Draft: draft,
		}, nil
	}

	return GenerateResult{
		JobID: jobID,
		Draft: draft,
	}, nil
}

type generationInputContext struct {
	NameHint string `json:"nameHint,omitempty"`
	SlugHint string `json:"slugHint,omitempty"`
	Prompt   string `json:"prompt"`
}

type generationPlan struct {
	SiteName     string                 `json:"siteName"`
	SiteGoal     string                 `json:"siteGoal"`
	ThemePreset  string                 `json:"themePreset"`
	Theme        siteconfig.ThemeConfig `json:"theme"`
	Pages        []generationPagePlan   `json:"pages"`
	AssetsNeeded []string               `json:"assetsNeeded"`
	Assumptions  []string               `json:"assumptions"`
}

type generationPagePlan struct {
	Title  string                `json:"title"`
	Slug   string                `json:"slug"`
	Goal   string                `json:"goal"`
	Blocks []generationBlockPlan `json:"blocks"`
	SEO    siteconfig.SEOConfig  `json:"seo"`
}

type generationBlockPlan struct {
	Type    string         `json:"type"`
	Purpose string         `json:"purpose"`
	Props   map[string]any `json:"props"`
}

type promptProfile struct {
	Category       string
	CategoryLabel  string
	ThemePreset    string
	PrimaryCTA     string
	ServicesTitle  string
	ServicesIntro  string
	FeatureItems   []map[string]any
	AboutHeading   string
	AboutBody      string
	GalleryHeading string
	GalleryBody    string
	ContactHeading string
	ContactBody    string
	WantsGallery   bool
	WantsWorkshops bool
}

func (s *Service) createGenerationJob(ctx context.Context, workspaceID string, userID string, input generationInputContext) (string, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode generation input context: %w", err)
	}

	var jobID string
	if err := s.db.QueryRow(ctx, `
		insert into generation_jobs (workspace_id, status, prompt, input_context, created_by)
		values ($1, 'running', $2, $3, nullif($4, '')::uuid)
		returning id::text
	`, workspaceID, input.Prompt, inputJSON, userID).Scan(&jobID); err != nil {
		return "", fmt.Errorf("create generation job: %w", err)
	}
	return jobID, nil
}

func (s *Service) completeGenerationJob(ctx context.Context, jobID string, siteID string, plan generationPlan) error {
	outputJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("encode generation output plan: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set site_id = $1::uuid,
		    status = 'completed',
		    output_plan = $2,
		    error = null,
		    updated_at = now()
		where id = $3::uuid
	`, siteID, outputJSON, jobID); err != nil {
		return fmt.Errorf("complete generation job: %w", err)
	}
	return nil
}

func (s *Service) failGenerationJob(ctx context.Context, jobID string, cause error) error {
	if jobID == "" {
		return nil
	}
	errorJSON, err := json.Marshal(map[string]any{
		"message": cause.Error(),
	})
	if err != nil {
		return fmt.Errorf("encode generation error: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set status = 'failed',
		    error = $1,
		    updated_at = now()
		where id = $2::uuid
	`, errorJSON, jobID); err != nil {
		return fmt.Errorf("mark generation job failed: %w", err)
	}
	return nil
}

func (s *Service) saveSiteMetadata(ctx context.Context, workspaceID string, siteID string, prompt string, plan generationPlan) error {
	summaryJSON, err := json.Marshal(map[string]any{
		"siteGoal":     plan.SiteGoal,
		"themePreset":  plan.ThemePreset,
		"assetsNeeded": plan.AssetsNeeded,
		"assumptions":  plan.Assumptions,
		"pageCount":    len(plan.Pages),
	})
	if err != nil {
		return fmt.Errorf("encode generation summary: %w", err)
	}

	tag, err := s.db.Exec(ctx, `
		update sites
		set generation_prompt = $1,
		    generation_summary = $2,
		    updated_at = now()
		where id = $3::uuid
		  and workspace_id = $4::uuid
	`, prompt, summaryJSON, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("save generation summary: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return sites.ErrNotFound
	}
	return nil
}

func (s *Service) createSlug(ctx context.Context, workspaceID string, requested string, name string) (string, error) {
	if value := strings.TrimSpace(requested); value != "" {
		if !slugs.IsValid(value) {
			return "", ErrSiteSlugInvalid
		}
		taken, err := s.siteSlugExists(ctx, workspaceID, value)
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrSiteSlugConflict
		}
		return value, nil
	}

	value, err := slugs.EnsureUnique(name, func(candidate string) (bool, error) {
		return s.siteSlugExists(ctx, workspaceID, candidate)
	})
	if err != nil {
		return "", fmt.Errorf("generate site slug: %w", err)
	}
	return value, nil
}

func (s *Service) siteSlugExists(ctx context.Context, workspaceID string, slugValue string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `
		select exists(
			select 1
			from sites
			where workspace_id = $1::uuid
			  and slug = $2
		)
	`, workspaceID, slugValue).Scan(&exists); err != nil {
		return false, fmt.Errorf("check site slug: %w", err)
	}
	return exists, nil
}

func buildGenerationPlan(nameHint string, prompt string) generationPlan {
	profile := profilePrompt(prompt)
	siteName := deriveSiteName(nameHint, profile)
	primaryCTAHref := "/contact"
	if profile.WantsWorkshops {
		primaryCTAHref = "/workshops"
	}

	pages := []generationPagePlan{
		homePagePlan(siteName, prompt, profile, primaryCTAHref),
	}
	if profile.WantsWorkshops {
		pages = append(pages, workshopsPagePlan(siteName, profile))
	} else {
		pages = append(pages, servicesPagePlan(siteName, profile))
	}
	if profile.WantsGallery {
		pages = append(pages, galleryPagePlan(siteName, profile))
	} else {
		pages = append(pages, aboutPagePlan(siteName, profile))
	}
	pages = append(pages, contactPagePlan(siteName, profile))

	assetsNeeded := []string{}
	if profile.WantsGallery || profile.Category == "photography" || profile.Category == "craft" {
		assetsNeeded = append(assetsNeeded, "hero-image", "supporting-image")
	}

	return generationPlan{
		SiteName:     siteName,
		SiteGoal:     siteGoalForCategory(profile.Category),
		ThemePreset:  profile.ThemePreset,
		Theme:        themePreset(profile.ThemePreset),
		Pages:        pages,
		AssetsNeeded: assetsNeeded,
		Assumptions:  assumptionsForProfile(profile),
	}
}

func buildDraftFromPlan(plan generationPlan, slugValue string) (siteconfig.SiteDraft, error) {
	siteID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate site id: %w", err)
	}

	pages := make([]siteconfig.PageDraft, 0, len(plan.Pages))
	navigation := make([]siteconfig.NavigationItem, 0, len(plan.Pages))
	siteDescription := clampSentence(firstNonEmpty(plan.Pages[0].SEO.Description, plan.SiteGoal), 180)

	for _, pagePlan := range plan.Pages {
		pageID, err := ids.New()
		if err != nil {
			return siteconfig.SiteDraft{}, fmt.Errorf("generate page id: %w", err)
		}
		blocks := make([]siteconfig.BlockInstance, 0, len(pagePlan.Blocks))
		for _, blockPlan := range pagePlan.Blocks {
			blockID, err := ids.New()
			if err != nil {
				return siteconfig.SiteDraft{}, fmt.Errorf("generate block id: %w", err)
			}
			blocks = append(blocks, siteconfig.BlockInstance{
				ID:      blockID,
				Type:    blockPlan.Type,
				Version: siteconfig.BlockVersionV1,
				Props:   blockPlan.Props,
			})
		}

		pages = append(pages, siteconfig.PageDraft{
			ID:     pageID,
			Title:  pagePlan.Title,
			Slug:   pagePlan.Slug,
			SEO:    pagePlan.SEO,
			Blocks: blocks,
		})
		navigation = append(navigation, siteconfig.NavigationItem{
			Label:  pagePlan.Title,
			PageID: pageID,
		})
	}

	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            siteID,
			Name:          plan.SiteName,
			Slug:          slugValue,
			Status:        "draft",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       clampSentence(plan.SiteName, 70),
				Description: siteDescription,
			},
		},
		Theme:      plan.Theme,
		Navigation: siteconfig.NavigationConfig{Primary: navigation},
		Pages:      pages,
	}

	if err := siteconfig.ValidateDraft(draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return draft, nil
}

func profilePrompt(prompt string) promptProfile {
	lower := strings.ToLower(prompt)
	profile := promptProfile{
		Category:      "business",
		CategoryLabel: "Small business website",
		ThemePreset:   "calm-nordic",
		PrimaryCTA:    "Get in touch",
		ServicesTitle: "What you can book or buy",
		ServicesIntro: "A concise overview of the main offers people should understand before they reach out.",
		FeatureItems: []map[string]any{
			{
				"title": "Clear offer",
				"body":  "Explain the core service in direct language without turning the homepage into a project.",
			},
			{
				"title": "Friendly process",
				"body":  "Set expectations, answer obvious questions, and make the next step feel easy.",
			},
			{
				"title": "Simple contact",
				"body":  "Point visitors toward one action that turns interest into an actual conversation.",
			},
		},
		AboutHeading:   "A small operation with a real point of view",
		AboutBody:      "Use this section to explain how the work feels, who it is for, and why customers trust you to do it well.",
		GalleryHeading: "Selected work",
		GalleryBody:    "Show a focused sample of recent work, finished pieces, or notable projects so visitors can picture the result.",
		ContactHeading: "Start the conversation",
		ContactBody:    "Make it easy for a visitor to ask a question, book a time, or request a quote without hunting for the next step.",
	}

	switch {
	case hasAny(lower, "photo", "photography", "portrait", "wedding photographer", "studio session"):
		profile.Category = "photography"
		profile.CategoryLabel = "Photography studio"
		profile.PrimaryCTA = "Book a session"
		profile.ServicesTitle = "Sessions and coverage"
		profile.ServicesIntro = "Group the main shoot types together so people can quickly see what you cover and what kind of experience you offer."
		profile.FeatureItems = []map[string]any{
			{"title": "Portrait sessions", "body": "Natural, low-pressure shoots for people who want honest photos without stiff direction."},
			{"title": "Brand imagery", "body": "A tidy set of images for websites, campaigns, launches, and day-to-day marketing."},
			{"title": "Events and occasions", "body": "Coverage that keeps the day moving while still catching the moments people actually remember."},
		}
		profile.AboutHeading = "A calm process makes better photographs"
		profile.AboutBody = "Use this space to describe your visual style, what a session feels like, and why clients leave with images that still feel like themselves."
		profile.GalleryHeading = "Recent shoots"
		profile.GalleryBody = "Break the work into a few focused examples: portraits, events, or brand sessions that show range without overwhelming the page."
		profile.WantsGallery = true
	case hasAny(lower, "florist", "flowers", "bouquet", "wedding flowers", "flower shop"):
		profile.Category = "florist"
		profile.CategoryLabel = "Florist studio"
		profile.PrimaryCTA = "Ask about an order"
		profile.ServicesTitle = "Seasonal flower work"
		profile.ServicesIntro = "Call out the formats that matter most: weddings, weekly flowers, custom orders, and one-off installations."
		profile.FeatureItems = []map[string]any{
			{"title": "Event flowers", "body": "Design for dinners, launches, celebrations, and weddings that need warmth rather than fuss."},
			{"title": "Weekly arrangements", "body": "Recurring flowers for shops, studios, offices, or homes that want the room to feel lived in."},
			{"title": "Custom orders", "body": "Smaller commissions, gift bouquets, and specific requests handled with a clear process."},
		}
		profile.WantsGallery = true
	case hasAny(lower, "yoga", "wellness", "massage", "therap", "coach", "counsel"):
		profile.Category = "wellness"
		profile.CategoryLabel = "Wellness practice"
		profile.PrimaryCTA = "Book a session"
		profile.ServicesTitle = "Sessions and support"
		profile.ServicesIntro = "Focus on the few ways people can work with you so the path from curiosity to booking stays short."
		profile.FeatureItems = []map[string]any{
			{"title": "Private sessions", "body": "One-to-one support tuned to a person, a goal, or a season of life."},
			{"title": "Group offerings", "body": "Classes, circles, or small-group formats that create rhythm without losing the human feel."},
			{"title": "Practical guidance", "body": "A grounded explanation of what happens, who it helps, and how to know if it is a fit."},
		}
		profile.WantsWorkshops = hasAny(lower, "class", "classes", "workshop", "course")
	case hasAny(lower, "design", "branding", "brand studio", "creative studio", "agency", "copywriter"):
		profile.Category = "creative"
		profile.CategoryLabel = "Creative studio"
		profile.PrimaryCTA = "Start a project"
		profile.ServicesTitle = "Services and retainers"
		profile.ServicesIntro = "Keep the offer structured around a few outcomes instead of a long inventory of tasks."
		profile.FeatureItems = []map[string]any{
			{"title": "Brand systems", "body": "Identity work that gives small businesses a clearer shape, voice, and visual rhythm."},
			{"title": "Launch pages", "body": "Focused pages for offers, openings, or campaigns that need to ship without drag."},
			{"title": "Ongoing support", "body": "A flexible way to keep copy, design, and site updates moving after the first launch."},
		}
		profile.WantsGallery = true
	case hasAny(lower, "textile", "yarn", "knit", "ceramic", "pottery", "craft", "maker", "atelier"):
		profile.Category = "craft"
		profile.CategoryLabel = "Craft studio"
		profile.PrimaryCTA = "See upcoming workshops"
		profile.ServicesTitle = "Pieces, commissions, and workshops"
		profile.ServicesIntro = "Explain what is available now, what can be commissioned, and how classes or workshops fit into the business."
		profile.FeatureItems = []map[string]any{
			{"title": "Small-batch pieces", "body": "Limited runs and seasonal releases with enough context to make each piece feel tangible."},
			{"title": "Custom work", "body": "Commission pathways for customers who want something made for a space, event, or gift."},
			{"title": "Hands-on workshops", "body": "Short classes that let people learn the material, process, and rhythm behind the work."},
		}
		profile.WantsGallery = true
		profile.WantsWorkshops = true
	case hasAny(lower, "bakery", "cafe", "coffee", "restaurant", "kitchen", "pastry"):
		profile.Category = "food"
		profile.CategoryLabel = "Local food business"
		profile.PrimaryCTA = "Plan a visit"
		profile.ServicesTitle = "Menu highlights and orders"
		profile.ServicesIntro = "Keep the story practical: what is available, when to stop by, and how to order for larger needs."
		profile.FeatureItems = []map[string]any{
			{"title": "Daily offering", "body": "A rotating line of fresh items that keeps regulars curious and first-time visitors oriented."},
			{"title": "Pre-orders", "body": "A clear way to reserve cakes, boxes, or larger orders without forcing a long back-and-forth."},
			{"title": "Events and catering", "body": "A simple summary of what kinds of gatherings you support and how to ask about them."},
		}
	}

	if hasAny(lower, "premium", "luxury", "dark", "dramatic", "bold", "editorial") {
		profile.ThemePreset = "meaner-dark"
	}
	if hasAny(lower, "playful", "fun", "bright", "colorful", "quirky", "silly") {
		profile.ThemePreset = "playful-ribbon"
	}
	if hasAny(lower, "portfolio", "gallery", "showcase", "visual") {
		profile.WantsGallery = true
	}
	if hasAny(lower, "workshop", "workshops", "class", "classes", "course", "courses") {
		profile.WantsWorkshops = true
	}

	return profile
}

func homePagePlan(siteName string, prompt string, profile promptProfile, primaryCTAHref string) generationPagePlan {
	description := clampSentence(prompt, 180)
	if description == "" {
		description = clampSentence(siteGoalForCategory(profile.Category), 180)
	}

	return generationPagePlan{
		Title: "Home",
		Slug:  "/",
		Goal:  "Introduce the business, prove relevance quickly, and move a new visitor toward the clearest next step.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(siteName, 70),
			Description: description,
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "hero",
				Purpose: "Position the business in one clear screen with a single primary call to action.",
				Props: map[string]any{
					"eyebrow":     profile.CategoryLabel,
					"headline":    homeHeadline(siteName, profile),
					"subheadline": homeSubheadline(profile),
					"primaryCta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  primaryCTAHref,
					},
					"layout": "split-left",
				},
			},
			{
				Type:    "text_section",
				Purpose: "Turn the prompt into a short promise that explains how the work feels and why it matters.",
				Props: map[string]any{
					"heading":   "A first draft shaped around the actual work",
					"body":      aboutIntro(profile),
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "List the main offers or reasons to trust the business in a scan-friendly format.",
				Props: map[string]any{
					"heading": profile.ServicesTitle,
					"intro":   profile.ServicesIntro,
					"columns": 3,
					"items":   toAnySlice(profile.FeatureItems),
				},
			},
			{
				Type:    "image_text",
				Purpose: "Reserve a visual slot and explain the experience, not just the deliverables.",
				Props: map[string]any{
					"heading":       "How the work usually feels",
					"body":          workProcessCopy(profile),
					"imagePosition": "right",
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Close the homepage with the single action the business most wants from new visitors.",
				Props: map[string]any{
					"heading": profile.ContactHeading,
					"body":    profile.ContactBody,
					"cta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  primaryCTAHref,
					},
				},
			},
		},
	}
}

func servicesPagePlan(siteName string, profile promptProfile) generationPagePlan {
	return generationPagePlan{
		Title: "Services",
		Slug:  "/services",
		Goal:  "Explain the main offer structure without forcing visitors to decode a long wall of copy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Services | "+siteName, 70),
			Description: clampSentence(profile.ServicesIntro, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Open the services page with a practical framing paragraph.",
				Props: map[string]any{
					"heading":   profile.ServicesTitle,
					"body":      profile.ServicesIntro,
					"alignment": "left",
					"width":     "wide",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "Break services into scannable sections that map to real customer questions.",
				Props: map[string]any{
					"heading": "What this includes",
					"intro":   "Use these cards to outline the typical formats, deliverables, or ways someone can work with you.",
					"columns": 3,
					"items":   toAnySlice(profile.FeatureItems),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Push the reader to contact once the offer is clear enough.",
				Props: map[string]any{
					"heading": "Need a version of this tailored to your project?",
					"body":    "Invite people to ask about timing, scope, or fit instead of making them guess how to start.",
					"cta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  "/contact",
					},
				},
			},
		},
	}
}

func workshopsPagePlan(siteName string, profile promptProfile) generationPagePlan {
	items := []map[string]any{
		{"title": "Intro sessions", "body": "A low-pressure first step for people who want to try the format before committing to more."},
		{"title": "Small-group workshops", "body": "Focused sessions with enough structure to teach a useful skill and enough space to keep them human."},
		{"title": "Private bookings", "body": "Custom sessions for teams, events, or people who need a more specific format."},
	}

	return generationPagePlan{
		Title: "Workshops",
		Slug:  "/workshops",
		Goal:  "Turn interest in classes or workshops into a concrete inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Workshops | "+siteName, 70),
			Description: clampSentence("Classes, small-group sessions, and private bookings.", 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Explain what a workshop is for and who should join.",
				Props: map[string]any{
					"heading":   "Learn it by doing it",
					"body":      "Describe the pace, materials, and level of guidance so people can quickly tell whether the class fits them.",
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "Show the formats someone can book without over-designing a schedule system yet.",
				Props: map[string]any{
					"heading": "Ways to join",
					"intro":   "Use these cards as the starting menu for your teaching formats.",
					"columns": 3,
					"items":   toAnySlice(items),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Send interested visitors toward the simplest way to ask about dates or availability.",
				Props: map[string]any{
					"heading": "Want the next date or a private booking?",
					"body":    "Invite the conversation before you build a more complex enrollment flow.",
					"cta": map[string]any{
						"label": "Ask about workshops",
						"href":  "/contact",
					},
				},
			},
		},
	}
}

func aboutPagePlan(siteName string, profile promptProfile) generationPagePlan {
	return generationPagePlan{
		Title: "About",
		Slug:  "/about",
		Goal:  "Add the human context that makes a small business feel trustworthy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("About | "+siteName, 70),
			Description: clampSentence(profile.AboutBody, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Tell the origin story without making the visitor work to find the point.",
				Props: map[string]any{
					"heading":   profile.AboutHeading,
					"body":      profile.AboutBody,
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "image_text",
				Purpose: "Pair a visual slot with a practical explanation of the process or point of view.",
				Props: map[string]any{
					"heading":       "Why people come back",
					"body":          workProcessCopy(profile),
					"imagePosition": "left",
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Bridge the story back to action.",
				Props: map[string]any{
					"heading": "If the approach fits, the next step should be simple",
					"body":    "Use the close of this page to invite a visit, a question, or a booking without changing tone.",
					"cta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  "/contact",
					},
				},
			},
		},
	}
}

func galleryPagePlan(siteName string, profile promptProfile) generationPagePlan {
	items := []map[string]any{
		{"title": "Recent highlights", "body": "A small set of examples that show range while still feeling edited and intentional."},
		{"title": "Style cues", "body": "Use captions or section text to explain the materials, mood, or process behind the work."},
		{"title": "What to ask for", "body": "Point people toward the kinds of commissions, sessions, or collaborations that make sense here."},
	}

	return generationPagePlan{
		Title: "Gallery",
		Slug:  "/gallery",
		Goal:  "Provide visual proof without depending on a full image library feature yet.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Gallery | "+siteName, 70),
			Description: clampSentence(profile.GalleryBody, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Frame the gallery with a short editorial note.",
				Props: map[string]any{
					"heading":   profile.GalleryHeading,
					"body":      profile.GalleryBody,
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "image_text",
				Purpose: "Reserve a large visual area and explain what the visitor is looking at.",
				Props: map[string]any{
					"heading":       "One representative feature can carry the page for now",
					"body":          "Use the image slot for a hero example, then tighten the surrounding copy so the page still works before asset uploads are fully built out.",
					"imagePosition": "right",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "Turn the rest of the gallery page into a structured proof section.",
				Props: map[string]any{
					"heading": "What this work tends to show",
					"intro":   "Use short cards to explain medium, style, or project types until a richer gallery block exists.",
					"columns": 3,
					"items":   toAnySlice(items),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Convert visual interest into contact.",
				Props: map[string]any{
					"heading": "Seen enough to ask about your own project?",
					"body":    "This page should end with the clearest contact action, not with a dead stop.",
					"cta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  "/contact",
					},
				},
			},
		},
	}
}

func contactPagePlan(siteName string, profile promptProfile) generationPagePlan {
	return generationPagePlan{
		Title: "Contact",
		Slug:  "/contact",
		Goal:  "Convert a ready visitor into an inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Contact | "+siteName, 70),
			Description: clampSentence(profile.ContactBody, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "hero",
				Purpose: "Lead with the contact promise instead of a cold form placeholder.",
				Props: map[string]any{
					"eyebrow":     "Contact",
					"headline":    profile.ContactHeading,
					"subheadline": profile.ContactBody,
					"primaryCta": map[string]any{
						"label": "Send an inquiry",
						"href":  "mailto:hello@example.com",
					},
					"layout": "centered",
				},
			},
			{
				Type:    "text_section",
				Purpose: "Add the practical details that reduce hesitation.",
				Props: map[string]any{
					"heading":   "What to include when you reach out",
					"body":      "A short note about timing, budget, scale, or the kind of help you need is enough to start a useful conversation.",
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Repeat the action so the page does not end ambiguously.",
				Props: map[string]any{
					"heading": "Prefer email first?",
					"body":    "Until forms are live, this can point to email, a booking tool, or another direct contact path.",
					"cta": map[string]any{
						"label": "Email hello@example.com",
						"href":  "mailto:hello@example.com",
					},
				},
			},
		},
	}
}

func themePreset(name string) siteconfig.ThemeConfig {
	switch name {
	case "meaner-dark":
		return siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background":   "#151215",
					"foreground":   "#f6f2ec",
					"surface":      "#231c24",
					"surfaceMuted": "#312736",
					"primary":      "#8fc6ff",
					"secondary":    "#8ee2d1",
					"accent":       "#ff8cad",
					"muted":        "#b78656",
					"border":       "#58415b",
					"ring":         "#f3b547",
				},
				Typography: map[string]any{
					"heading":     "Iowan Old Style",
					"body":        "Avenir Next",
					"headingFont": "Iowan Old Style",
					"bodyFont":    "Avenir Next",
					"scale":       "editorial",
				},
				Layout: map[string]any{
					"maxWidth":       "1120px",
					"contentWidth":   "720px",
					"sectionSpacing": "96px",
				},
				Shape: map[string]any{
					"radius": "28px",
					"shadow": "soft",
				},
			},
		}
	case "playful-ribbon":
		return siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background":   "#fff9f4",
					"foreground":   "#2b2324",
					"surface":      "#fff4ec",
					"surfaceMuted": "#f8e4de",
					"primary":      "#356fbd",
					"secondary":    "#7bc7bb",
					"accent":       "#f07a98",
					"muted":        "#b78656",
					"border":       "#e8cdbd",
					"ring":         "#e5a13a",
				},
				Typography: map[string]any{
					"heading":     "Avenir Next",
					"body":        "Avenir Next",
					"headingFont": "Avenir Next",
					"bodyFont":    "Avenir Next",
					"scale":       "playful",
				},
				Layout: map[string]any{
					"maxWidth":       "1120px",
					"contentWidth":   "720px",
					"sectionSpacing": "88px",
				},
				Shape: map[string]any{
					"radius": "32px",
					"shadow": "soft",
				},
			},
		}
	default:
		return siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background":   "#f6f2ec",
					"foreground":   "#2b2324",
					"surface":      "#fff9f4",
					"surfaceMuted": "#efe4d9",
					"primary":      "#356fbd",
					"secondary":    "#7bc7bb",
					"accent":       "#f07a98",
					"muted":        "#b78656",
					"border":       "#dfd1c3",
					"ring":         "#e5a13a",
				},
				Typography: map[string]any{
					"heading":     "Iowan Old Style",
					"body":        "Avenir Next",
					"headingFont": "Iowan Old Style",
					"bodyFont":    "Avenir Next",
					"scale":       "calm",
				},
				Layout: map[string]any{
					"maxWidth":       "1120px",
					"contentWidth":   "720px",
					"sectionSpacing": "96px",
				},
				Shape: map[string]any{
					"radius": "28px",
					"shadow": "soft",
				},
			},
		}
	}
}

func deriveSiteName(nameHint string, profile promptProfile) string {
	if value := cleanName(nameHint); value != "" {
		return value
	}
	switch profile.Category {
	case "photography":
		return "North Light Studio"
	case "florist":
		return "Field Note Florals"
	case "wellness":
		return "Quiet Room Practice"
	case "creative":
		return "Threadline Studio"
	case "craft":
		return "Ribbon & Reed Atelier"
	case "food":
		return "Morning Table"
	default:
		return "Small Good Studio"
	}
}

func cleanName(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}

func siteGoalForCategory(category string) string {
	switch category {
	case "photography":
		return "Turn visitors into photography inquiries and bookings."
	case "florist":
		return "Turn visitors into flower orders, event inquiries, and repeat customers."
	case "wellness":
		return "Turn visitors into session bookings and confident first conversations."
	case "creative":
		return "Turn visitors into well-qualified project inquiries."
	case "craft":
		return "Turn visitors into workshop bookings, commissions, and product interest."
	case "food":
		return "Turn visitors into visits, pre-orders, and catering inquiries."
	default:
		return "Turn visitors into clear, low-friction inquiries."
	}
}

func assumptionsForProfile(profile promptProfile) []string {
	assumptions := []string{
		"Default locale is English.",
		"Contact routes use placeholder email copy until forms or real contact details are added.",
	}
	if profile.WantsGallery {
		assumptions = append(assumptions, "Visual sections use placeholder image slots until asset uploads are available.")
	}
	if profile.WantsWorkshops {
		assumptions = append(assumptions, "Workshop and class information is presented as structured marketing content, not a booking system.")
	}
	return assumptions
}

func homeHeadline(siteName string, profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Natural photography for real people, places, and moments"
	case "florist":
		return "Flowers for gatherings, gifting, and the everyday table"
	case "wellness":
		return "A steadier practice for busy bodies and loud minds"
	case "creative":
		return "Thoughtful design for small businesses that need to look ready"
	case "craft":
		return "Handmade work with real texture, practical beauty, and room to learn"
	case "food":
		return "A local spot people want to return to"
	default:
		return siteName + " needs a clear, confident website that gets to the point"
	}
}

func homeSubheadline(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Use this draft to introduce your style, show the work, and make booking a session feel straightforward."
	case "florist":
		return "Start with a warm overview of seasonal work, custom orders, and the events or spaces you help shape."
	case "wellness":
		return "Give new visitors a calm explanation of what you offer, who it helps, and the easiest way to begin."
	case "creative":
		return "Frame the offer around outcomes, not jargon, so good-fit clients understand the value quickly."
	case "craft":
		return "Explain the mix of products, commissions, and workshops in one place without losing the handmade feel."
	case "food":
		return "Show the atmosphere, the offering, and the practical next step before the visitor bounces for logistics."
	default:
		return "This first draft keeps the message short, the structure clear, and the next step obvious."
	}
}

func aboutIntro(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Lead with the kind of moments you help people keep, then describe the relaxed experience around the camera."
	case "florist":
		return "Anchor the page in seasonality, mood, and the practical kinds of orders you actually want to receive."
	case "wellness":
		return "Explain your approach plainly: what happens, how it feels, and what someone should expect after the first session."
	case "creative":
		return "Describe the way you work with small teams so the process feels structured without sounding rigid or inflated."
	case "craft":
		return "Use the page to connect the finished pieces with the materials, techniques, and teaching behind them."
	case "food":
		return "A good food site gives enough atmosphere to make a visit sound appealing without turning basic details into a scavenger hunt."
	default:
		return "Small business sites work better when the visitor can understand the offer, the style, and the next step in under a minute."
	}
}

func workProcessCopy(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "A strong draft process usually moves from a quick inquiry to clear planning, a low-drama shoot, and a tidy handoff of the final images."
	case "florist":
		return "Keep the process readable: a short inquiry, a quick alignment on scale and mood, then a clear path to delivery or installation."
	case "wellness":
		return "The point here is clarity: who this is for, what support looks like in practice, and how people know when to reach out."
	case "creative":
		return "Good-fit projects start with scope, direction, and a clean decision-maker path, then move through concise rounds instead of endless drift."
	case "craft":
		return "People respond well when they can see the handwork and understand the rhythm behind it, whether that ends in a class, a commission, or a purchase."
	case "food":
		return "Food businesses benefit from a simple explanation of what is fresh, what changes often, and how larger or special orders are handled."
	default:
		return "Use this section to replace vague promises with a short description of how the work actually happens and what a customer gets from it."
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clampSentence(value string, limit int) string {
	text := strings.TrimSpace(value)
	if text == "" || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

func toAnySlice(items []map[string]any) []any {
	values := make([]any, 0, len(items))
	for _, item := range items {
		values = append(values, item)
	}
	return values
}

func hasAny(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
