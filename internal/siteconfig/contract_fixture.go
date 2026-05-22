package siteconfig

import "fmt"

type BlockRegistryContractFixture struct {
	BlockRegistry []BlockDefinition `json:"blockRegistry"`
	Draft         SiteDraft         `json:"draft"`
}

func BuildBlockRegistryContractFixture() BlockRegistryContractFixture {
	definitions := DefaultBlockRegistry().Definitions()
	blocks := make([]BlockInstance, 0, len(definitions))
	for index, definition := range definitions {
		blocks = append(blocks, BlockInstance{
			ID:      fmt.Sprintf("block_%02d_%s", index+1, definition.Type),
			Type:    definition.Type,
			Version: definition.Version,
			Props:   cloneProps(definition.DefaultProps),
		})
	}

	return BlockRegistryContractFixture{
		BlockRegistry: definitions,
		Draft: SiteDraft{
			Site: DraftSite{
				ID:            "site_registry_contract",
				Name:          "Registry Contract Studio",
				Slug:          "registry-contract-studio",
				Status:        "draft",
				DefaultLocale: "en",
				SEO: SEOConfig{
					Title:       "Registry Contract Studio",
					Description: "Shared block-registry fixture proving Go validation and React rendering stay aligned.",
				},
			},
			Brand: BrandConfig{
				BusinessName: "Registry Contract Studio",
				PrimaryColor: "#8ee2d1",
			},
			Theme: ThemeConfig{
				Version: ThemeVersionV1,
				Tokens: ThemeTokens{
					Colors: map[string]string{
						"background":   "#151215",
						"foreground":   "#f3ead8",
						"surface":      "#231c24",
						"surfaceMuted": "#302333",
						"primary":      "#8ee2d1",
						"secondary":    "#8fc6ff",
						"accent":       "#ff8cad",
						"border":       "#5a3e57",
						"muted":        "#caa778",
						"ring":         "#f3b547",
					},
					Typography: map[string]any{
						"headingFont": "Iowan Old Style",
						"bodyFont":    "Avenir Next",
					},
					Layout: map[string]any{
						"sectionPaddingX": "24px",
						"sectionPaddingY": "96px",
						"contentWidth":    "720px",
					},
					Shape: map[string]any{
						"radius": "28px",
					},
				},
			},
			Navigation: NavigationConfig{
				Primary: []NavigationItem{
					{Label: "Home", PageID: "page_home"},
				},
			},
			Pages: []PageDraft{
				{
					ID:    "page_home",
					Title: "Home",
					Slug:  "/",
					SEO: SEOConfig{
						Title:       "Registry Contract Studio",
						Description: "Fixture page containing one valid instance of every Go-owned prototype block.",
					},
					Blocks: blocks,
				},
			},
		},
	}
}

func cloneProps(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneProps(typed)
	case []any:
		cloned := make([]any, 0, len(typed))
		for _, item := range typed {
			cloned = append(cloned, cloneValue(item))
		}
		return cloned
	default:
		return typed
	}
}
