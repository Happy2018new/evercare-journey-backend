package main

import "testing"

func TestCuratedCategoriesCoverEveryAttraction(t *testing.T) {
	if len(curatedCategoryNames) != len(attractions) {
		t.Fatalf("curated categories=%d, attractions=%d", len(curatedCategoryNames), len(attractions))
	}

	seenSlugs := make(map[string]struct{}, len(attractions))
	for _, attraction := range attractions {
		if _, exists := seenSlugs[attraction.Slug]; exists {
			t.Fatalf("duplicate attraction slug %q", attraction.Slug)
		}
		seenSlugs[attraction.Slug] = struct{}{}

		categoryName, exists := curatedCategoryNames[attraction.Slug]
		if !exists {
			t.Errorf("missing curated category for %q", attraction.Slug)
			continue
		}
		if err := validateCuratedCategoryName(categoryName); err != nil {
			t.Errorf("invalid curated category for %q: %v", attraction.Slug, err)
		}
	}
}
