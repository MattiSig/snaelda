package siteconfig

// CollectAssetIDs returns the set of asset identifiers referenced anywhere in
// the supplied site content. Brand logos are included alongside block and entry
// assets because both validation and public delivery must treat them as
// first-class site asset references.
func CollectAssetIDs(brand BrandConfig, pages []PageDraft, collections []Collection) map[string]struct{} {
	references := map[string]struct{}{}
	if brand.Logo != nil && brand.Logo.AssetID != "" {
		references[brand.Logo.AssetID] = struct{}{}
	}
	for _, page := range pages {
		for _, block := range page.Blocks {
			collectAssetIDsFromValue(block.Props, references)
		}
	}
	for _, collection := range collections {
		for _, entry := range collection.Entries {
			collectAssetIDsFromValue(entry.Fields, references)
		}
	}
	return references
}

// CollectSnapshotAssetIDs returns asset identifiers referenced anywhere in a
// published snapshot. Entry assets must be allowlisted alongside block assets
// so the public asset route serves images bound into collection_detail
// templates and rendered in collection_list / collection_index blocks.
func CollectSnapshotAssetIDs(snapshot PublishedSnapshot) map[string]struct{} {
	return CollectAssetIDs(snapshot.Brand, snapshot.Pages, snapshot.Collections)
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
