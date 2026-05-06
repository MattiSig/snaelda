package siteconfig

import (
	"errors"
	"fmt"
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
	Type          string
	Version       string
	DisplayName   string
	Category      BlockCategory
	DefaultProps  map[string]any
	EditorSchema  []EditorField
	ValidateProps func(path string, props map[string]any, c *collector)
}

type EditorField struct {
	Name    string   `json:"name"`
	Label   string   `json:"label"`
	Control string   `json:"control"`
	Options []string `json:"options,omitempty"`
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
		heroBlockDefinition(),
		textSectionBlockDefinition(),
		imageTextBlockDefinition(),
		featuresGridBlockDefinition(),
		ctaBandBlockDefinition(),
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

func (r *BlockRegistry) ValidateProps(blockType string, version string, path string, props map[string]any) error {
	definition, err := r.Lookup(blockType, version)
	if err != nil {
		return err
	}
	var c collector
	definition.ValidateProps(path, props, &c)
	return c.err()
}
