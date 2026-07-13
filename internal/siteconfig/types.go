package siteconfig

type SiteDraft struct {
	Revision    int64            `json:"revision"`
	Site        DraftSite        `json:"site"`
	Brand       BrandConfig      `json:"brand"`
	Theme       ThemeConfig      `json:"theme"`
	Navigation  NavigationConfig `json:"navigation"`
	Pages       []PageDraft      `json:"pages"`
	Collections []Collection     `json:"collections,omitempty"`
}

// BrandConfig captures the user-provided business identity that the theme
// system and brand-aware blocks (Header, Footer, sometimes Hero) read from at
// render time. Brand is a sibling to theme, not a child of it: brand is the
// source of truth, theme is the derived rendering layer.
type BrandConfig struct {
	BusinessName string     `json:"businessName"`
	Logo         *BrandLogo `json:"logo,omitempty"`
	PrimaryColor string     `json:"primaryColor"`
}

type BrandLogo struct {
	AssetID string `json:"assetId"`
	Alt     string `json:"alt"`
	// Size scales the logo in the site header: "small" (default), "medium", or
	// "large" — wide lockup logos need more presence than a compact mark.
	Size string `json:"size,omitempty"`
	// HideName suppresses the visible business-name text beside the header
	// logo, for lockup logos that already carry the name. The name stays in
	// the DOM visually hidden so assistive tech and crawlers still read it.
	HideName bool `json:"hideName,omitempty"`
}

type DraftSite struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Status        string    `json:"status"`
	DefaultLocale string    `json:"defaultLocale,omitempty"`
	SEO           SEOConfig `json:"seo,omitempty"`
}

type PublishedSnapshot struct {
	SchemaVersion string           `json:"schemaVersion"`
	Site          PublishedSite    `json:"site"`
	Brand         BrandConfig      `json:"brand"`
	Theme         ThemeConfig      `json:"theme"`
	Navigation    NavigationConfig `json:"navigation"`
	Pages         []PageDraft      `json:"pages"`
	Collections   []Collection     `json:"collections,omitempty"`
	ImageCredits  []ImageCredit    `json:"imageCredits,omitempty"`
}

// ImageCredit records the attribution data the renderer should surface on
// public pages when a published version references a backend-imported
// starter image (Pexels, Unsplash, etc.).
type ImageCredit struct {
	Provider  string `json:"provider"`
	Author    string `json:"author,omitempty"`
	AuthorURL string `json:"authorUrl,omitempty"`
	SourceURL string `json:"sourceUrl,omitempty"`
	License   string `json:"license,omitempty"`
}

type PublishedSite struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DefaultLocale string    `json:"defaultLocale"`
	SEO           SEOConfig `json:"seo"`
}

type PageDraft struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Slug         string          `json:"slug"`
	Status       string          `json:"status,omitempty"`
	Type         string          `json:"type,omitempty"`
	CollectionID string          `json:"collectionId,omitempty"`
	SEO          SEOConfig       `json:"seo,omitempty"`
	Blocks       []BlockInstance `json:"blocks"`
	Settings     map[string]any  `json:"settings,omitempty"`
}

const (
	PageTypeStatic           = "static"
	PageTypeCollectionIndex  = "collection_index"
	PageTypeCollectionDetail = "collection_detail"

	PageStatusDraft     = "draft"
	PageStatusPublished = "published"
)

type SEOConfig struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type BlockInstance struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Version  string                  `json:"version"`
	Props    map[string]any          `json:"props"`
	Settings BlockSettings           `json:"settings,omitempty"`
	Bindings map[string]BlockBinding `json:"bindings,omitempty"`
}

// BlockBinding pulls the bound block prop value from the active collection
// entry at render time, replacing the literal value in Props. Bindings are
// only valid inside collection_detail page templates.
type BlockBinding struct {
	Source string `json:"source"`
	Field  string `json:"field"`
}

const BlockBindingSourceEntry = "entry"

type BlockSettings struct {
	Hidden   bool   `json:"hidden,omitempty"`
	AnchorID string `json:"anchorId,omitempty"`
}

type ThemeConfig struct {
	Version string      `json:"version"`
	Tokens  ThemeTokens `json:"tokens"`
}

type ThemeTokens struct {
	Colors     map[string]string `json:"colors"`
	Typography map[string]any    `json:"typography"`
	Layout     map[string]any    `json:"layout"`
	Shape      map[string]any    `json:"shape"`
}

type NavigationConfig struct {
	Primary []NavigationItem `json:"primary"`
	Footer  []NavigationItem `json:"footer,omitempty"`
}

type NavigationItem struct {
	Label  string `json:"label"`
	PageID string `json:"pageId,omitempty"`
	Href   string `json:"href,omitempty"`
}

type FooterContact struct {
	Address string   `json:"address,omitempty"`
	Phone   string   `json:"phone,omitempty"`
	Email   string   `json:"email,omitempty"`
	Hours   []string `json:"hours,omitempty"`
}

type FormDefinition struct {
	Fields            []FormField `json:"fields"`
	SuccessMessage    string      `json:"successMessage,omitempty"`
	NotificationEmail string      `json:"notificationEmail,omitempty"`
}

type FormField struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}
