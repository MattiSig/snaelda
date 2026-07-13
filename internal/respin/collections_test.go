package respin

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func servicesN(n int) []ExtractService {
	out := make([]ExtractService, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ExtractService{Name: "Service " + itoaSmall(i+1)})
	}
	return out
}

func peopleN(n int) []ExtractPerson {
	out := make([]ExtractPerson, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ExtractPerson{Name: "Person " + itoaSmall(i+1), Role: "Role"})
	}
	return out
}

func TestShouldPromoteServices(t *testing.T) {
	cases := []struct {
		name     string
		services []ExtractService
		want     bool
	}{
		{"large list promotes", servicesN(4), true},
		{"three plain services stay static", servicesN(3), false},
		{"two with substance promote", []ExtractService{
			{Name: "Drain cleaning", Description: "We clear it", Price: "9.900 kr."},
			{Name: "Camera inspection", Description: "See the pipe", Price: "14.900 kr."},
		}, true},
		{"two without price stay static", []ExtractService{
			{Name: "Drain cleaning", Description: "We clear it"},
			{Name: "Camera inspection", Description: "See the pipe"},
		}, false},
		{"single service stays static", servicesN(1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldPromoteServices(tc.services); got != tc.want {
				t.Fatalf("shouldPromoteServices = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldPromotePeople(t *testing.T) {
	if shouldPromotePeople(peopleN(5)) {
		t.Fatalf("5 people should stay in the static team block")
	}
	if !shouldPromotePeople(peopleN(6)) {
		t.Fatalf("6 people should promote to a collection")
	}
}

func TestSynthesizeSeedCollectionsServices(t *testing.T) {
	fields := ExtractedFields{
		Services: []ExtractService{
			{Name: "Þrif á niðurföllum", Description: "Við hreinsum stífluna", Price: "9.900 kr."},
			{Name: "Myndavélaskoðun", Description: "Skoðum rörið", Price: "14.900 kr."},
			{Name: "Neyðarþjónusta", Price: "Frá 12.900 kr."},
			{Name: "Viðhald", Description: "Reglulegt viðhald"},
		},
	}
	res := synthesizeSeedCollections(fields, "is")
	if !res.PromotedServices {
		t.Fatalf("expected services promoted")
	}
	if len(res.Collections) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(res.Collections))
	}
	col := res.Collections[0]
	if col.Slug != "thjonusta" {
		t.Fatalf("expected Icelandic transliterated slug thjonusta, got %q", col.Slug)
	}
	if !col.Settings.ExposeDetailURLs {
		t.Fatalf("expected collection to expose detail URLs")
	}
	if len(col.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(col.Entries))
	}
	for _, e := range col.Entries {
		if e.Status != siteconfig.EntryStatusPublished {
			t.Fatalf("entry %q not published: %q", e.Slug, e.Status)
		}
		if e.Fields["title"] == nil {
			t.Fatalf("entry %q missing title", e.Slug)
		}
	}
	// The whole collection must be a valid siteconfig shape.
	if err := siteconfig.ValidateCollection(col); err != nil {
		t.Fatalf("synthesized collection invalid: %v", err)
	}
}

func TestSynthesizeSeedCollectionsPeople(t *testing.T) {
	fields := ExtractedFields{People: peopleN(6)}
	res := synthesizeSeedCollections(fields, "en")
	if !res.PromotedPeople {
		t.Fatalf("expected people promoted")
	}
	if len(res.Collections) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(res.Collections))
	}
	col := res.Collections[0]
	if col.Slug != "team" {
		t.Fatalf("expected slug team, got %q", col.Slug)
	}
	if len(col.Entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(col.Entries))
	}
	if err := siteconfig.ValidateCollection(col); err != nil {
		t.Fatalf("synthesized collection invalid: %v", err)
	}
}

func TestSynthesizeSeedCollectionsDedupesEntrySlugs(t *testing.T) {
	fields := ExtractedFields{
		Services: []ExtractService{
			{Name: "Þrif", Description: "a", Price: "1 kr."},
			{Name: "Þrif", Description: "b", Price: "2 kr."},
		},
	}
	res := synthesizeSeedCollections(fields, "is")
	if !res.PromotedServices || len(res.Collections) != 1 {
		t.Fatalf("expected one promoted services collection")
	}
	seen := map[string]bool{}
	for _, e := range res.Collections[0].Entries {
		if seen[e.Slug] {
			t.Fatalf("duplicate entry slug %q", e.Slug)
		}
		seen[e.Slug] = true
	}
	if err := siteconfig.ValidateCollection(res.Collections[0]); err != nil {
		t.Fatalf("collection invalid: %v", err)
	}
}

func TestSynthesizeSeedCollectionsNoneWhenSmall(t *testing.T) {
	res := synthesizeSeedCollections(ExtractedFields{
		Services: servicesN(2),
		People:   peopleN(3),
	}, "is")
	if res.PromotedServices || res.PromotedPeople || len(res.Collections) != 0 {
		t.Fatalf("expected no collections for small lists, got %+v", res)
	}
}
