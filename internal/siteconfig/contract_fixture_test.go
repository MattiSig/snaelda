package siteconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildBlockRegistryContractFixtureProducesValidRegistryExamples(t *testing.T) {
	fixture := BuildBlockRegistryContractFixture()

	if err := ValidateDraft(fixture.Draft); err != nil {
		t.Fatalf("validate fixture draft: %v", err)
	}

	registry := DefaultBlockRegistry()
	blocksByType := map[string]BlockInstance{}
	for _, block := range fixture.Draft.Pages[0].Blocks {
		blocksByType[block.Type] = block
	}

	if len(fixture.BlockRegistry) != len(blocksByType) {
		t.Fatalf("expected fixture to include %d blocks, got %d", len(fixture.BlockRegistry), len(blocksByType))
	}

	for _, definition := range fixture.BlockRegistry {
		block, ok := blocksByType[definition.Type]
		if !ok {
			t.Fatalf("expected fixture block for %s", definition.Type)
		}
		if block.Version != definition.Version {
			t.Fatalf("expected fixture block %s to use version %s, got %s", definition.Type, definition.Version, block.Version)
		}
		if err := registry.ValidateProps(definition.Type, definition.Version, "props", cloneProps(block.Props)); err != nil {
			t.Fatalf("validate fixture props for %s@%s: %v", definition.Type, definition.Version, err)
		}
	}

	requiredTypes := []string{
		"hero",
		"text_section",
		"image_text",
		"features_grid",
		"gallery",
		"testimonials",
		"pricing_packages",
		"cta_band",
		"contact_form",
		"faq",
		"team_profile_cards",
		"footer",
		"stats",
		"collection_list",
		"collection_index",
		"collection_detail",
	}
	for _, requiredType := range requiredTypes {
		if _, ok := blocksByType[requiredType]; !ok {
			t.Fatalf("expected fixture block for spec-required type %s", requiredType)
		}
	}
}

func TestBlockRegistryContractFixtureMatchesGoldenFile(t *testing.T) {
	fixture := BuildBlockRegistryContractFixture()
	got, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile(filepath.Join("testdata", "block_registry_contract.json"))
	if err != nil {
		t.Fatalf("read fixture file: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("block registry fixture is out of sync with testdata/block_registry_contract.json")
	}
}
