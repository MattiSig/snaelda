package siteconfig

type SiteDraft struct {
	Site       DraftSite        `json:"site"`
	Theme      ThemeConfig      `json:"theme"`
	Navigation NavigationConfig `json:"navigation"`
	Pages      []PageDraft      `json:"pages"`
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
	Theme         ThemeConfig      `json:"theme"`
	Navigation    NavigationConfig `json:"navigation"`
	Pages         []PageDraft      `json:"pages"`
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
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Slug     string          `json:"slug"`
	SEO      SEOConfig       `json:"seo,omitempty"`
	Blocks   []BlockInstance `json:"blocks"`
	Settings map[string]any  `json:"settings,omitempty"`
}

type SEOConfig struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type BlockInstance struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Version  string         `json:"version"`
	Props    map[string]any `json:"props"`
	Settings BlockSettings  `json:"settings,omitempty"`
}

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
}

type NavigationItem struct {
	Label  string `json:"label"`
	PageID string `json:"pageId,omitempty"`
	Href   string `json:"href,omitempty"`
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
