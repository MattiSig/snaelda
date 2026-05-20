package collections

import "errors"

var (
	ErrCollectionNotFound      = errors.New("collection was not found")
	ErrEntryNotFound           = errors.New("collection entry was not found")
	ErrCollectionInUse         = errors.New("collection cannot be deleted while pages still bind to it")
	ErrCollectionSlugConflict  = errors.New("collection slug is already in use")
	ErrEntrySlugConflict       = errors.New("entry slug is already in use")
	ErrCollectionLabelRequired = errors.New("collection labels are required")
	ErrCollectionSlugInvalid   = errors.New("collection slug must be lowercase words separated by hyphens")
	ErrEntryOrderInvalid       = errors.New("entry reorder must include every entry exactly once")
	ErrNoCollectionChanges     = errors.New("collection update requires at least one change")
	ErrNoEntryChanges          = errors.New("entry update requires at least one change")
)
