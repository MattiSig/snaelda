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

// CollectSnapshotAssetIDs returns asset identifiers referenced anywhere in a
// published snapshot — both block props and collection-entry field values.
// Entry assets must be allowlisted alongside block assets so the public asset
// route serves images bound into collection_detail templates and rendered in
// collection_list / collection_index blocks.
func CollectSnapshotAssetIDs(pages []PageDraft, collections []Collection) map[string]struct{} {
	references := CollectAssetIDs(pages)
	for _, collection := range collections {
		for _, entry := range collection.Entries {
			collectAssetIDsFromValue(entry.Fields, references)
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
