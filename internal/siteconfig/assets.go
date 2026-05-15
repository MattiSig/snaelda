package siteconfig

// CollectAssetIDs returns the set of asset identifiers referenced by blocks in
// the supplied pages. The walk inspects every prop value recursively for an
// "assetId" string, which is the contract every block image field uses.
func CollectAssetIDs(pages []PageDraft) map[string]struct{} {
	references := map[string]struct{}{}
	for _, page := range pages {
		for _, block := range page.Blocks {
			collectAssetIDsFromValue(block.Props, references)
		}
	}
	return references
}

func collectAssetIDsFromValue(value any, references map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		if assetID, ok := typed["assetId"].(string); ok && assetID != "" {
			references[assetID] = struct{}{}
		}
		for _, nested := range typed {
			collectAssetIDsFromValue(nested, references)
		}
	case []any:
		for _, nested := range typed {
			collectAssetIDsFromValue(nested, references)
		}
	}
}
