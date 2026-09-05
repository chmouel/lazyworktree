package utils

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// wordPattern enforces the invariants callers rely on: lowercase ASCII only,
// no hyphens (names are split on a single hyphen) and a sensible length so
// generated worktree directory names stay manageable.
var wordPattern = regexp.MustCompile(`^[a-z]{3,12}$`)

func TestWordListsSatisfyInvariants(t *testing.T) {
	lists := map[string][]string{
		"randomAdjectives": randomAdjectives,
		"randomNouns":      randomNouns,
	}

	for name, list := range lists {
		if len(list) < 100 {
			t.Errorf("%s has %d entries, want at least 100 for a varied pool", name, len(list))
		}

		seen := make(map[string]bool, len(list))
		for _, word := range list {
			if !wordPattern.MatchString(word) {
				t.Errorf("%s contains %q, want a lowercase ASCII word of 3-12 characters", name, word)
			}
			if seen[word] {
				t.Errorf("%s contains duplicate entry %q", name, word)
			}
			seen[word] = true
		}

		if !slices.IsSorted(list) {
			t.Errorf("%s is not sorted alphabetically", name)
		}
	}
}

func TestWordListsAreDisjoint(t *testing.T) {
	adjectives := make(map[string]bool, len(randomAdjectives))
	for _, word := range randomAdjectives {
		adjectives[word] = true
	}
	for _, word := range randomNouns {
		if adjectives[word] {
			t.Errorf("%q appears in both word lists, which allows names like %q", word, word+"-"+word)
		}
	}
}

func TestRandomBranchNameFormat(t *testing.T) {
	adjectives := make(map[string]bool, len(randomAdjectives))
	for _, word := range randomAdjectives {
		adjectives[word] = true
	}
	nouns := make(map[string]bool, len(randomNouns))
	for _, word := range randomNouns {
		nouns[word] = true
	}

	for range 200 {
		name := RandomBranchName()
		parts := strings.Split(name, "-")
		if len(parts) != 2 {
			t.Fatalf("expected 'adjective-noun' with a single hyphen, got %q", name)
		}
		if !adjectives[parts[0]] {
			t.Fatalf("%q is not a known adjective (from %q)", parts[0], name)
		}
		if !nouns[parts[1]] {
			t.Fatalf("%q is not a known noun (from %q)", parts[1], name)
		}
	}
}

func TestRandomBranchNameVariety(t *testing.T) {
	const iterations = 500

	seen := make(map[string]bool, iterations)
	for range iterations {
		seen[RandomBranchName()] = true
	}

	// With ~59,000 combinations, 500 draws should almost always be distinct.
	// A low threshold still catches a truncated or accidentally tiny list
	// without making the test flaky.
	if len(seen) < iterations/2 {
		t.Errorf("got %d distinct names from %d draws, want a far more varied pool", len(seen), iterations)
	}
}

func TestRandomWordEdgeCases(t *testing.T) {
	if got := randomWord(nil); got != "" {
		t.Errorf("randomWord(nil) = %q, want empty string", got)
	}
	if got := randomWord([]string{}); got != "" {
		t.Errorf("randomWord(empty) = %q, want empty string", got)
	}
	if got := randomWord([]string{"solitary"}); got != "solitary" {
		t.Errorf("randomWord(single) = %q, want %q", got, "solitary")
	}
}
