package engine

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// loadManor loads the manor.json golden definition.
func loadManor(t *testing.T) *Definition {
	t.Helper()
	b, err := os.ReadFile("../../testdata/manor.json")
	if err != nil {
		t.Fatal(err)
	}
	var def Definition
	if err := json.Unmarshal(b, &def); err != nil {
		t.Fatal(err)
	}
	return &def
}

// TestManorValid confirms the manor definition has no validation errors.
func TestManorValid(t *testing.T) {
	def := loadManor(t)
	if errs := ValidateDefinition(def); len(errs) != 0 {
		t.Fatalf("manor definition should be valid; errors: %v", errs)
	}
}

// TestManorLore verifies that the manor golden contains the expected lore entries
// and that the discover op correctly tracks them on an instance.
func TestManorLore(t *testing.T) {
	def := loadManor(t)

	// Confirm the two expected lore entries are present and valid.
	if _, ok := def.Lore["manor_history"]; !ok {
		t.Fatal("manor_history lore entry missing")
	}
	if _, ok := def.Lore["locket_provenance"]; !ok {
		t.Fatal("locket_provenance lore entry missing")
	}

	mh := def.Lore["manor_history"]
	if mh.Title == "" || mh.Text == "" {
		t.Fatalf("manor_history missing title or text: %+v", mh)
	}
	if mh.When != "Year 312" {
		t.Errorf("manor_history when: got %q, want Year 312", mh.When)
	}
	if !strings.Contains(strings.Join(mh.Tags, ","), "manor") {
		t.Errorf("manor_history tags should include 'manor': %v", mh.Tags)
	}

	lp := def.Lore["locket_provenance"]
	if lp.Subject != "player" {
		t.Errorf("locket_provenance subject: got %q, want player", lp.Subject)
	}

	// Start an instance and discover manor_history.
	st, err := StartInstance(def, "manor_lore_run", 1)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if len(st.DiscoveredLore) != 0 {
		t.Fatal("DiscoveredLore should start empty")
	}

	st2, _, err := ApplyOps(def, st, []Effect{
		{Op: "discover", Lore: "manor_history"},
	})
	if err != nil {
		t.Fatalf("discover manor_history: %v", err)
	}
	if len(st2.DiscoveredLore) != 1 || st2.DiscoveredLore[0] != "manor_history" {
		t.Fatalf("DiscoveredLore = %v, want [manor_history]", st2.DiscoveredLore)
	}

	// Discover locket_provenance as well.
	st3, _, err := ApplyOps(def, st2, []Effect{
		{Op: "discover", Lore: "locket_provenance"},
	})
	if err != nil {
		t.Fatalf("discover locket_provenance: %v", err)
	}
	if len(st3.DiscoveredLore) != 2 {
		t.Fatalf("expected 2 discovered, got %d: %v", len(st3.DiscoveredLore), st3.DiscoveredLore)
	}

	// Idempotent: re-discover manor_history — still 2.
	st4, _, err := ApplyOps(def, st3, []Effect{
		{Op: "discover", Lore: "manor_history"},
	})
	if err != nil {
		t.Fatalf("idempotent discover: %v", err)
	}
	if len(st4.DiscoveredLore) != 2 {
		t.Fatalf("idempotent discover should leave count at 2, got %d", len(st4.DiscoveredLore))
	}
}

// sortedStrings returns a sorted copy of a []any slice as []string.
func sortedStrings(t *testing.T, raw any) []string {
	t.Helper()
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T: %v", raw, raw)
	}
	out := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("element %d is %T, not string", i, v)
		}
		out[i] = s
	}
	sort.Strings(out)
	return out
}

// TestManorTravel exercises starting an instance, spatial queries, the move op
// (with and without via), and verifies that descriptions are seeded.
func TestManorTravel(t *testing.T) {
	def := loadManor(t)

	st, err := StartInstance(def, "manor_run", 1)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}

	// --- Initial positions ---
	player := st.Entities["player"]
	if player == nil {
		t.Fatal("player entity missing")
	}
	if player.Attrs["location"] != "study" {
		t.Fatalf("player starts at %v, want study", player.Attrs["location"])
	}

	// --- Location descriptions seeded ---
	if st.Entities["study"].Description == "" {
		t.Fatal("study entity should have a seeded description")
	}
	if st.Entities["hall"].Description == "" {
		t.Fatal("hall entity should have a seeded description")
	}
	if st.Entities["garden"].Description == "" {
		t.Fatal("garden entity should have a seeded description")
	}

	// --- exits_here from study → ["hall"] ---
	exitsSpec := def.Derived["exits_here"]
	got, err := computeDerived(def, st, exitsSpec, "")
	if err != nil {
		t.Fatalf("exits_here from study: %v", err)
	}
	exitIDs := sortedStrings(t, got)
	if len(exitIDs) != 1 || exitIDs[0] != "hall" {
		t.Fatalf("exits_here from study = %v, want [hall]", exitIDs)
	}

	// --- ApplyOps: move player study→hall via exit ---
	st2, _, err := ApplyOps(def, st, []Effect{
		{Op: "move", Entity: "player", To: "hall", Via: "exit"},
	})
	if err != nil {
		t.Fatalf("move player to hall via exit: %v", err)
	}
	if st2.Entities["player"].Attrs["location"] != "hall" {
		t.Fatalf("player.location after move: got %v, want hall",
			st2.Entities["player"].Attrs["location"])
	}

	// --- exits_here from hall → ["garden","study"] ---
	got2, err := computeDerived(def, st2, exitsSpec, "")
	if err != nil {
		t.Fatalf("exits_here from hall: %v", err)
	}
	exitIDs2 := sortedStrings(t, got2)
	if len(exitIDs2) != 2 || exitIDs2[0] != "garden" || exitIDs2[1] != "study" {
		t.Fatalf("exits_here from hall = %v, want [garden study]", exitIDs2)
	}

	// --- Move with no exit (study→garden directly) must be rejected ---
	_, _, errNoExit := ApplyOps(def, st, []Effect{
		{Op: "move", Entity: "player", To: "garden", Via: "exit"},
	})
	if errNoExit == nil {
		t.Fatal("expected error: no exit study→garden")
	}

	// --- here query: both player and player share hall after move ---
	hereSpec := def.Derived["here"]
	gotHere, err := computeDerived(def, st2, hereSpec, "")
	if err != nil {
		t.Fatalf("here from hall: %v", err)
	}
	hereIDs := sortedStrings(t, gotHere)
	// Only the player is a person; they are at hall.
	if len(hereIDs) != 1 || hereIDs[0] != "player" {
		t.Fatalf("here from hall = %v, want [player]", hereIDs)
	}
}
