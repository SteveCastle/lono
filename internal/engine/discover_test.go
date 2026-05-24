package engine

import (
	"strings"
	"testing"
)

func defWithLore() *Definition {
	return &Definition{
		ID:      "g",
		Version: 1,
		Lore: map[string]LoreEntry{
			"founding": {Title: "The Founding", Text: "Built in Year 312."},
			"locket":   {Title: "The Locket", Text: "A silver locket.", Subject: "player"},
		},
	}
}

// TestDiscoverAppends verifies that the discover op adds the lore id to DiscoveredLore.
func TestDiscoverAppends(t *testing.T) {
	def := defWithLore()
	st, _ := NewInstance(def, "r", 1)

	if len(st.DiscoveredLore) != 0 {
		t.Fatal("DiscoveredLore should start empty")
	}

	st2, _, err := ApplyOps(def, st, []Effect{
		{Op: "discover", Lore: "founding"},
	})
	if err != nil {
		t.Fatalf("discover founding: %v", err)
	}
	if len(st2.DiscoveredLore) != 1 || st2.DiscoveredLore[0] != "founding" {
		t.Fatalf("DiscoveredLore = %v, want [founding]", st2.DiscoveredLore)
	}
}

// TestDiscoverIdempotent verifies that discovering the same lore twice does not
// produce duplicate entries.
func TestDiscoverIdempotent(t *testing.T) {
	def := defWithLore()
	st, _ := NewInstance(def, "r", 1)

	st2, _, err := ApplyOps(def, st, []Effect{
		{Op: "discover", Lore: "founding"},
		{Op: "discover", Lore: "founding"},
	})
	if err != nil {
		t.Fatalf("discover twice: %v", err)
	}
	if len(st2.DiscoveredLore) != 1 {
		t.Fatalf("expected 1 entry after double discover, got %d: %v", len(st2.DiscoveredLore), st2.DiscoveredLore)
	}
}

// TestDiscoverUnknownLore verifies that discovering an unknown lore id returns an error.
func TestDiscoverUnknownLore(t *testing.T) {
	def := defWithLore()
	st, _ := NewInstance(def, "r", 1)

	_, _, err := ApplyOps(def, st, []Effect{
		{Op: "discover", Lore: "no_such_entry"},
	})
	if err == nil {
		t.Fatal("expected error for unknown lore id")
	}
	if !strings.Contains(err.Error(), "no_such_entry") {
		t.Fatalf("error should mention the id, got: %v", err)
	}
}

// TestDiscoverMultiple verifies that multiple distinct lore entries each appear once.
func TestDiscoverMultiple(t *testing.T) {
	def := defWithLore()
	st, _ := NewInstance(def, "r", 1)

	st2, _, err := ApplyOps(def, st, []Effect{
		{Op: "discover", Lore: "founding"},
		{Op: "discover", Lore: "locket"},
	})
	if err != nil {
		t.Fatalf("discover two: %v", err)
	}
	if len(st2.DiscoveredLore) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(st2.DiscoveredLore), st2.DiscoveredLore)
	}
}

// TestDiscoverValidation verifies that validateEffect rejects a discover with no lore id.
func TestDiscoverValidation(t *testing.T) {
	errs := validateEffect("test", Effect{Op: "discover", Lore: ""})
	if len(errs) == 0 {
		t.Fatal("expected validation error for empty lore id")
	}
}
