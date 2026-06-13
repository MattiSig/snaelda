package collections

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// SchemaMigrationRequiredError wraps the structured diff that surfaces when a
// schema patch is rejected because it would lose or invalidate entry data.
// Handlers serialize the diff so the editor can render a migration prompt.
type SchemaMigrationRequiredError struct {
	CollectionID string
	Diff         SchemaDiff
	Unmapped     []SchemaChange
}

func (e *SchemaMigrationRequiredError) Error() string {
	return ErrSchemaMigrationRequired.Error()
}

func (e *SchemaMigrationRequiredError) Is(target error) bool {
	return errors.Is(target, ErrSchemaMigrationRequired)
}

// SchemaMigrationIncompleteError wraps the unmapped destructive changes that
// a migrate call did not acknowledge.
type SchemaMigrationIncompleteError struct {
	CollectionID string
	Diff         SchemaDiff
	Unmapped     []SchemaChange
}

func (e *SchemaMigrationIncompleteError) Error() string {
	return ErrSchemaMigrationIncomplete.Error()
}

func (e *SchemaMigrationIncompleteError) Is(target error) bool {
	return errors.Is(target, ErrSchemaMigrationIncomplete)
}

// SchemaChangeKind classifies how a single field changed between the current
// collection schema and a proposed one.
type SchemaChangeKind string

const (
	SchemaChangeAdded    SchemaChangeKind = "added"
	SchemaChangeRemoved  SchemaChangeKind = "removed"
	SchemaChangeRenamed  SchemaChangeKind = "renamed"
	SchemaChangeRetyped  SchemaChangeKind = "retyped"
	SchemaChangeModified SchemaChangeKind = "modified"
)

// SchemaChange describes one field-level difference between two schemas.
type SchemaChange struct {
	Kind     SchemaChangeKind             `json:"kind"`
	OldField *siteconfig.FieldDefinition  `json:"oldField,omitempty"`
	NewField *siteconfig.FieldDefinition  `json:"newField,omitempty"`
}

// SchemaDiff captures the full classification of changes between the current
// and proposed schemas, together with a flag for whether the change is
// destructive (would lose or invalidate existing entry values).
type SchemaDiff struct {
	Changes     []SchemaChange `json:"changes"`
	Destructive bool           `json:"destructive"`
}

// FieldMapping is one explicit user/operator acknowledgement of a destructive
// schema change. The mapping tells the migration engine how to carry forward
// the values stored on existing entries.
type FieldMapping struct {
	// Action describes what to do for this entry-side data.
	// One of "rename", "retype_clear", or "drop".
	Action string `json:"action"`
	// OldKey is the field key in the current schema this mapping applies to.
	OldKey string `json:"oldKey,omitempty"`
	// NewKey is the field key in the proposed schema. Required for "rename".
	NewKey string `json:"newKey,omitempty"`
}

// MigrationPlan summarizes what would happen if a migration were applied. It
// is intended for preview/UI consumption: the diff plus the per-entry
// transformations and any unmet acknowledgements.
type MigrationPlan struct {
	Diff             SchemaDiff           `json:"diff"`
	Mappings         []FieldMapping       `json:"mappings"`
	EntriesAffected  int                  `json:"entriesAffected"`
	UnmappedChanges  []SchemaChange       `json:"unmappedChanges,omitempty"`
	NewSchemaVersion int                  `json:"newSchemaVersion"`
}

// DiffSchemas computes a stable structured diff between the current and
// proposed schemas. Renames are surfaced only when explicit `renames` mappings
// (newKey -> oldKey) bind a removed key to an added key.
//
// Renames must be supplied separately because the platform cannot
// automatically infer them: removing field "title" and adding "name" could
// either be a rename (the user wants to keep values) or a destructive replace.
func DiffSchemas(current []siteconfig.FieldDefinition, proposed []siteconfig.FieldDefinition, renames map[string]string) SchemaDiff {
	currentByKey := make(map[string]siteconfig.FieldDefinition, len(current))
	for _, field := range current {
		currentByKey[field.Key] = field
	}
	proposedByKey := make(map[string]siteconfig.FieldDefinition, len(proposed))
	for _, field := range proposed {
		proposedByKey[field.Key] = field
	}

	// Apply renames: newKey -> oldKey means the user explicitly bound the
	// proposed "newKey" to the existing "oldKey".
	renameSourcesByNewKey := map[string]string{}
	consumedOldKeys := map[string]bool{}
	for newKey, oldKey := range renames {
		newKey = strings.TrimSpace(newKey)
		oldKey = strings.TrimSpace(oldKey)
		if newKey == "" || oldKey == "" {
			continue
		}
		if _, ok := currentByKey[oldKey]; !ok {
			continue
		}
		if _, ok := proposedByKey[newKey]; !ok {
			continue
		}
		renameSourcesByNewKey[newKey] = oldKey
		consumedOldKeys[oldKey] = true
	}

	diff := SchemaDiff{}

	// Iterate proposed in declaration order so output is deterministic with
	// respect to the user's edit.
	for _, next := range proposed {
		previous, kept := currentByKey[next.Key]
		if kept {
			if previous.Type != next.Type {
				cp := previous
				np := next
				diff.Changes = append(diff.Changes, SchemaChange{
					Kind:     SchemaChangeRetyped,
					OldField: &cp,
					NewField: &np,
				})
				diff.Destructive = true
				continue
			}
			if fieldDefinitionsModified(previous, next) {
				cp := previous
				np := next
				change := SchemaChange{
					Kind:     SchemaChangeModified,
					OldField: &cp,
					NewField: &np,
				}
				diff.Changes = append(diff.Changes, change)
				if requiresMigrationModification(previous, next) {
					diff.Destructive = true
				}
			}
			continue
		}
		if oldKey, renamed := renameSourcesByNewKey[next.Key]; renamed {
			from := currentByKey[oldKey]
			np := next
			cp := from
			diff.Changes = append(diff.Changes, SchemaChange{
				Kind:     SchemaChangeRenamed,
				OldField: &cp,
				NewField: &np,
			})
			diff.Destructive = true
			if from.Type != next.Type {
				diff.Destructive = true
			}
			continue
		}
		// Brand-new field that no existing entry can have a value for.
		np := next
		diff.Changes = append(diff.Changes, SchemaChange{
			Kind:     SchemaChangeAdded,
			NewField: &np,
		})
	}

	// Removed fields are anything in current that isn't consumed by a rename
	// and doesn't appear under its original key in the proposed schema.
	removedKeys := []string{}
	for key, previous := range currentByKey {
		if _, kept := proposedByKey[key]; kept {
			continue
		}
		if consumedOldKeys[key] {
			continue
		}
		removedKeys = append(removedKeys, previous.Key)
	}
	sort.Strings(removedKeys)
	for _, key := range removedKeys {
		previous := currentByKey[key]
		cp := previous
		diff.Changes = append(diff.Changes, SchemaChange{
			Kind:     SchemaChangeRemoved,
			OldField: &cp,
		})
		diff.Destructive = true
	}

	return diff
}

func fieldDefinitionsModified(a, b siteconfig.FieldDefinition) bool {
	if a.Label != b.Label {
		return true
	}
	if a.Required != b.Required {
		return true
	}
	if a.Description != b.Description {
		return true
	}
	if !stringSlicesEqual(a.Options, b.Options) {
		return true
	}
	if !fieldValidationsEqual(a.Validation, b.Validation) {
		return true
	}
	return false
}

func requiresMigrationModification(prev, next siteconfig.FieldDefinition) bool {
	// Turning an existing optional field into required can invalidate
	// pre-existing entries with empty values for that field.
	if !prev.Required && next.Required {
		return true
	}
	// Removing an enum option from the declared set invalidates any entry
	// whose stored value referenced the dropped option.
	if (prev.Type == siteconfig.FieldTypeEnum || prev.Type == siteconfig.FieldTypeEnumMulti) && enumOptionsNarrowed(prev.Options, next.Options) {
		return true
	}
	return false
}

func enumOptionsNarrowed(prev, next []string) bool {
	allowed := make(map[string]bool, len(next))
	for _, value := range next {
		allowed[value] = true
	}
	for _, value := range prev {
		if !allowed[value] {
			return true
		}
	}
	return false
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index, value := range a {
		if b[index] != value {
			return false
		}
	}
	return true
}

func fieldValidationsEqual(a, b *siteconfig.FieldValidation) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !intPtrEqual(a.MinLength, b.MinLength) {
		return false
	}
	if !intPtrEqual(a.MaxLength, b.MaxLength) {
		return false
	}
	if !float64PtrEqual(a.Min, b.Min) {
		return false
	}
	if !float64PtrEqual(a.Max, b.Max) {
		return false
	}
	return true
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func float64PtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// classifiedMappings reduces FieldMapping inputs into the rename map (newKey
// -> oldKey) plus sets of acknowledged drops/retypes.
type classifiedMappings struct {
	renames       map[string]string
	drops         map[string]bool
	retypeAcks    map[string]bool
}

func classifyMappings(mappings []FieldMapping) (classifiedMappings, error) {
	result := classifiedMappings{
		renames:    map[string]string{},
		drops:      map[string]bool{},
		retypeAcks: map[string]bool{},
	}
	for index, mapping := range mappings {
		action := strings.TrimSpace(mapping.Action)
		oldKey := strings.TrimSpace(mapping.OldKey)
		newKey := strings.TrimSpace(mapping.NewKey)
		switch action {
		case "rename":
			if oldKey == "" || newKey == "" {
				return result, fmt.Errorf("mapping[%d] rename requires oldKey and newKey", index)
			}
			if existing, ok := result.renames[newKey]; ok && existing != oldKey {
				return result, fmt.Errorf("mapping[%d] rename %q is already mapped to %q", index, newKey, existing)
			}
			result.renames[newKey] = oldKey
		case "drop":
			if oldKey == "" {
				return result, fmt.Errorf("mapping[%d] drop requires oldKey", index)
			}
			result.drops[oldKey] = true
		case "retype_clear":
			if oldKey == "" {
				return result, fmt.Errorf("mapping[%d] retype_clear requires oldKey", index)
			}
			result.retypeAcks[oldKey] = true
		default:
			return result, fmt.Errorf("mapping[%d] action %q is not supported", index, action)
		}
	}
	return result, nil
}

// applyMigrationToEntries transforms entry field maps based on the diff and
// the supplied mappings, returning a new slice. It does NOT validate against
// the proposed schema — callers should run siteconfig.ValidateDraft after
// substituting the migrated collection.
func applyMigrationToEntries(entries []siteconfig.CollectionEntry, diff SchemaDiff, classified classifiedMappings) []siteconfig.CollectionEntry {
	migrated := make([]siteconfig.CollectionEntry, len(entries))
	renamesByOldKey := map[string]string{}
	for newKey, oldKey := range classified.renames {
		renamesByOldKey[oldKey] = newKey
	}
	retypedKeys := map[string]bool{}
	removedKeys := map[string]bool{}
	for _, change := range diff.Changes {
		switch change.Kind {
		case SchemaChangeRetyped:
			if change.OldField != nil {
				retypedKeys[change.OldField.Key] = true
			}
		case SchemaChangeRemoved:
			if change.OldField != nil {
				removedKeys[change.OldField.Key] = true
			}
		}
	}
	for i, entry := range entries {
		nextFields := make(map[string]any, len(entry.Fields))
		for key, value := range entry.Fields {
			if removedKeys[key] {
				continue
			}
			if retypedKeys[key] {
				// Acknowledged retype clears the old value.
				continue
			}
			if newKey, ok := renamesByOldKey[key]; ok {
				nextFields[newKey] = value
				continue
			}
			nextFields[key] = value
		}
		entry.Fields = nextFields
		migrated[i] = entry
	}
	return migrated
}

// destructiveChanges filters a diff down to changes that would invalidate or
// drop stored entry data, in stable order.
func destructiveChanges(diff SchemaDiff) []SchemaChange {
	var out []SchemaChange
	for _, change := range diff.Changes {
		switch change.Kind {
		case SchemaChangeRetyped, SchemaChangeRemoved, SchemaChangeRenamed:
			out = append(out, change)
		case SchemaChangeModified:
			if change.OldField != nil && change.NewField != nil && requiresMigrationModification(*change.OldField, *change.NewField) {
				out = append(out, change)
			}
		}
	}
	return out
}

// requireMappings checks that every destructive change in the diff is
// acknowledged by the supplied mappings; it returns the list of changes that
// remain unmapped.
func requireMappings(diff SchemaDiff, classified classifiedMappings) []SchemaChange {
	var unmapped []SchemaChange
	for _, change := range diff.Changes {
		switch change.Kind {
		case SchemaChangeRetyped:
			if change.OldField == nil {
				continue
			}
			if !classified.retypeAcks[change.OldField.Key] {
				unmapped = append(unmapped, change)
			}
		case SchemaChangeRemoved:
			if change.OldField == nil {
				continue
			}
			if !classified.drops[change.OldField.Key] {
				unmapped = append(unmapped, change)
			}
		case SchemaChangeRenamed:
			// Already covered by the supplied rename entry; reaching this
			// branch means DiffSchemas surfaced a rename so the mapping was
			// honored by construction.
		}
	}
	return unmapped
}
