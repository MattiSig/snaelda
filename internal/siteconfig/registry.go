package siteconfig

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrBlockTypeUnknown    = errors.New("block type is unknown")
	ErrBlockVersionUnknown = errors.New("block version is unknown")
)

type BlockCategory string

const (
	BlockCategoryHero       BlockCategory = "hero"
	BlockCategoryContent    BlockCategory = "content"
	BlockCategoryMedia      BlockCategory = "media"
	BlockCategoryConversion BlockCategory = "conversion"
)

type BlockDefinition struct {
	Type          string                                                                    `json:"type"`
	Version       string                                                                    `json:"version"`
	DisplayName   string                                                                    `json:"displayName"`
	Category      BlockCategory                                                             `json:"category"`
	Tagline       string                                                                    `json:"tagline,omitempty"`
	DefaultProps  map[string]any                                                            `json:"defaultProps,omitempty"`
	EditorSchema  []EditorField                                                             `json:"editorSchema,omitempty"`
	PropSchema    map[string]any                                                            `json:"-"`
	MigrateProps  func(previousVersion string, previousProps map[string]any) map[string]any `json:"-"`
	ValidateProps func(path string, props map[string]any, c *collector)                     `json:"-"`
}

type EditorField struct {
	Name        string        `json:"name"`
	Label       string        `json:"label"`
	Control     string        `json:"control"`
	ValueType   string        `json:"valueType,omitempty"`
	Description string        `json:"description,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Options     []string      `json:"options,omitempty"`
	Fields      []EditorField `json:"fields,omitempty"`
	ItemFields  []EditorField `json:"itemFields,omitempty"`
}

type BlockRegistry struct {
	byType map[string]map[string]BlockDefinition
}

func NewBlockRegistry(definitions ...BlockDefinition) (*BlockRegistry, error) {
	registry := &BlockRegistry{byType: map[string]map[string]BlockDefinition{}}
	for _, definition := range definitions {
		if definition.Type == "" {
			return nil, fmt.Errorf("register block: type is required")
		}
		if definition.Version == "" {
			return nil, fmt.Errorf("register block %s: version is required", definition.Type)
		}
		if definition.ValidateProps == nil {
			return nil, fmt.Errorf("register block %s@%s: props validator is required", definition.Type, definition.Version)
		}
		if registry.byType[definition.Type] == nil {
			registry.byType[definition.Type] = map[string]BlockDefinition{}
		}
		if _, exists := registry.byType[definition.Type][definition.Version]; exists {
			return nil, fmt.Errorf("register block %s@%s: duplicate definition", definition.Type, definition.Version)
		}
		registry.byType[definition.Type][definition.Version] = definition
	}
	return registry, nil
}

func DefaultBlockRegistry() *BlockRegistry {
	registry, err := NewBlockRegistry(
		collectionDetailBlockDefinition(),
		collectionIndexBlockDefinition(),
		collectionListBlockDefinition(),
		contactFormBlockDefinition(),
		ctaBandBlockDefinition(),
		faqBlockDefinition(),
		featuresGridBlockDefinition(),
		footerBlockDefinition(),
		galleryBlockDefinition(),
		heroBlockDefinition(),
		heroBlockDefinitionV1(),
		imageTextBlockDefinition(),
		pricingPackagesBlockDefinition(),
		statsBlockDefinition(),
		teamProfileCardsBlockDefinition(),
		testimonialsBlockDefinition(),
		textSectionBlockDefinition(),
	)
	if err != nil {
		panic(err)
	}
	return registry
}

func (r *BlockRegistry) Lookup(blockType string, version string) (BlockDefinition, error) {
	if r == nil {
		r = DefaultBlockRegistry()
	}
	versions := r.byType[blockType]
	if versions == nil {
		return BlockDefinition{}, ErrBlockTypeUnknown
	}
	definition, ok := versions[version]
	if !ok {
		return BlockDefinition{}, ErrBlockVersionUnknown
	}
	return definition, nil
}

// Latest returns the highest-versioned definition registered for a block type.
// It is what stamps new blocks: stored blocks keep resolving their recorded
// version through Lookup, while freshly created blocks always take the newest.
func (r *BlockRegistry) Latest(blockType string) (BlockDefinition, error) {
	if r == nil {
		r = DefaultBlockRegistry()
	}
	versions := r.byType[blockType]
	if len(versions) == 0 {
		return BlockDefinition{}, ErrBlockTypeUnknown
	}
	var latest BlockDefinition
	for version, definition := range versions {
		if latest.Version == "" || compareBlockVersions(version, latest.Version) > 0 {
			latest = definition
		}
	}
	return latest, nil
}

// LatestBlockVersion returns the current version to stamp on a new block of the
// given type, falling back to BlockVersionV1 for unknown types so callers keep
// their existing unknown-type error paths.
func LatestBlockVersion(blockType string) string {
	definition, err := DefaultBlockRegistry().Latest(blockType)
	if err != nil {
		return BlockVersionV1
	}
	return definition.Version
}

// compareBlockVersions compares two dotted numeric versions ("1.10.0" > "1.9.0").
// Non-numeric segments fall back to string comparison.
func compareBlockVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		an, aErr := strconv.Atoi(av)
		bn, bErr := strconv.Atoi(bv)
		if aErr == nil && bErr == nil {
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
			continue
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func (r *BlockRegistry) ValidateProps(blockType string, version string, path string, props map[string]any) error {
	definition, err := r.Lookup(blockType, version)
	if err != nil {
		return err
	}
	var c collector
	definition.ValidateProps(path, props, &c)
	return c.err()
}

func (r *BlockRegistry) Definitions() []BlockDefinition {
	if r == nil {
		r = DefaultBlockRegistry()
	}

	// One definition per type — the latest version. Older versions stay
	// registered only so stored blocks resolve; catalogs, editor schemas, and
	// the frontend contract all describe the current shape.
	definitions := make([]BlockDefinition, 0, len(r.byType))
	for blockType := range r.byType {
		definition, err := r.Latest(blockType)
		if err != nil {
			continue
		}
		definitions = append(definitions, definition)
	}

	sort.SliceStable(definitions, func(i int, j int) bool {
		return definitions[i].Type < definitions[j].Type
	})

	return definitions
}
