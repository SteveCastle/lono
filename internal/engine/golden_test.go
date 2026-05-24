package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func loadHeist(t *testing.T) *Definition {
	t.Helper()
	b, err := os.ReadFile("../../testdata/heist.json")
	if err != nil {
		t.Fatal(err)
	}
	var def Definition
	if err := json.Unmarshal(b, &def); err != nil {
		t.Fatal(err)
	}
	return &def
}

func TestHeistDefinitionValid(t *testing.T) {
	if errs := ValidateDefinition(loadHeist(t)); len(errs) != 0 {
		t.Fatalf("heist should be valid: %v", errs)
	}
}

func TestHeistPlaythrough(t *testing.T) {
	def := loadHeist(t)
	st, err := StartInstance(def, "run", 7)
	if err != nil {
		t.Fatal(err)
	}
	// Setup applied.
	if st.Entities["player"].Inventory["lockpick"] != 2 {
		t.Fatalf("setup inventory wrong: %v", st.Entities["player"].Inventory)
	}

	st, _, err = PerformAction(def, st, "arc", "break_in", nil)
	if err != nil {
		t.Fatalf("break_in: %v", err)
	}
	if st.Machines["arc"] != "inside" || st.Entities["player"].Attrs["location"] != "vault" {
		t.Fatalf("break_in did not advance correctly: %+v", st.Machines)
	}

	st, _, err = PerformAction(def, st, "arc", "grab_loot", map[string]any{"bags": float64(2)})
	if err != nil {
		t.Fatalf("grab_loot: %v", err)
	}
	loot := st.World["loot"].(float64)
	if loot < 1 || loot > 6 {
		t.Fatalf("loot after 1d6 out of range: %v", loot)
	}

	st, _, err = PerformAction(def, st, "arc", "escape", nil)
	if err != nil {
		t.Fatalf("escape: %v", err)
	}
	if st.Machines["arc"] != "escaped" {
		t.Fatal("did not escape")
	}
	if findRelationship(st, "trust", "aria", "player").Attrs["value"] != float64(5) {
		t.Fatal("trust not adjusted on escape")
	}
}

func TestHeistDeterministic(t *testing.T) {
	def := loadHeist(t)
	run := func() float64 {
		st, _ := StartInstance(def, "r", 7)
		st, _, _ = PerformAction(def, st, "arc", "break_in", nil)
		st, _, _ = PerformAction(def, st, "arc", "grab_loot", map[string]any{"bags": float64(1)})
		return st.World["loot"].(float64)
	}
	if run() != run() {
		t.Fatal("same seed must produce the same loot roll")
	}
}
