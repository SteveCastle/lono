package engine

import (
	"encoding/json"
	"testing"
)

// TestLoreRoundTrip verifies that a Definition with LoreEntry values survives a
// JSON round-trip with all fields intact.
func TestLoreRoundTrip(t *testing.T) {
	def := &Definition{
		ID:      "g",
		Version: 1,
		Lore: map[string]LoreEntry{
			"founding": {
				Title:   "The Founding",
				Text:    "The manor was built in Year 1 by the Ashford family.",
				Tags:    []string{"history", "manor"},
				Subject: "manor",
				When:    "Year 1",
				Intent:  "background",
			},
			"portrait": {
				Title: "The Portrait",
				Text:  "A stern woman stares from the oil painting.",
			},
		},
	}

	b, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Definition
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	entry := got.Lore["founding"]
	if entry.Title != "The Founding" {
		t.Errorf("title: got %q", entry.Title)
	}
	if entry.Text != "The manor was built in Year 1 by the Ashford family." {
		t.Errorf("text: got %q", entry.Text)
	}
	if len(entry.Tags) != 2 || entry.Tags[0] != "history" {
		t.Errorf("tags: got %v", entry.Tags)
	}
	if entry.Subject != "manor" {
		t.Errorf("subject: got %q", entry.Subject)
	}
	if entry.When != "Year 1" {
		t.Errorf("when: got %q", entry.When)
	}

	if got.Lore["portrait"].Title != "The Portrait" {
		t.Error("portrait title missing")
	}
}

// TestLoreValidation verifies that ValidateDefinition rejects lore entries
// missing a title or text.
func TestLoreValidation(t *testing.T) {
	good := &Definition{
		ID:      "g",
		Version: 1,
		Lore: map[string]LoreEntry{
			"ok": {Title: "Title", Text: "Text"},
		},
	}
	if errs := ValidateDefinition(good); len(errs) != 0 {
		t.Fatalf("expected valid, got %v", errs)
	}

	bad := &Definition{
		ID:      "b",
		Version: 1,
		Lore: map[string]LoreEntry{
			"no_title": {Text: "some text"},
			"no_text":  {Title: "some title"},
		},
	}
	errs := ValidateDefinition(bad)
	if len(errs) < 2 {
		t.Fatalf("expected >=2 validation errors, got %d: %v", len(errs), errs)
	}
	// Check paths
	found := map[string]bool{}
	for _, e := range errs {
		found[e.Path] = true
	}
	if !found["lore.no_title.title"] {
		t.Error("expected lore.no_title.title error")
	}
	if !found["lore.no_text.text"] {
		t.Error("expected lore.no_text.text error")
	}
}
