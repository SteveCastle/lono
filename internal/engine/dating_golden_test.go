package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func loadDating(t *testing.T) *Definition {
	t.Helper()
	b, err := os.ReadFile("../../testdata/dating.json")
	if err != nil {
		t.Fatal(err)
	}
	var def Definition
	if err := json.Unmarshal(b, &def); err != nil {
		t.Fatal(err)
	}
	return &def
}

func TestDatingDefinitionValid(t *testing.T) {
	if errs := ValidateDefinition(loadDating(t)); len(errs) != 0 {
		t.Fatalf("dating def should be valid: %v", errs)
	}
}

func TestDatingPlaythrough(t *testing.T) {
	def := loadDating(t)
	st, err := StartInstance(def, "run", 7)
	if err != nil {
		t.Fatal(err)
	}
	// Setup created the romance with its attached machine at "acquaintances".
	r := findRelationship(st, "romance", "aria", "player")
	if r == nil || r.Machines["romance_stage"] != "acquaintances" {
		t.Fatalf("attached romance machine not initialized: %+v", r)
	}
	// The arrival beat is active at "arrive".
	if !beatActive(def, st, "arrival") {
		t.Fatal("arrival beat should be active")
	}

	// Can't enter the gallery undressed.
	if _, _, err := PerformAction(def, st, "arc", "enter", nil); err == nil {
		t.Fatal("enter should require an outfit")
	}
	// Dress, then enter.
	st, _, err = ApplyOps(def, st, []Effect{{Op: "equip", Entity: "player", Slot: "torso", Item: "gallery_suit"}})
	if err != nil {
		t.Fatal(err)
	}
	st, _, err = PerformAction(def, st, "arc", "enter", nil)
	if err != nil {
		t.Fatalf("enter after dressing: %v", err)
	}

	// Advance the romance: flirt then charm -> affection 60, machine "smitten".
	host := &HostRef{Kind: "relationship", From: "aria", To: "player"}
	st, _, err = PerformHostAction(def, st, "romance_stage", "flirt", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err = PerformHostAction(def, st, "romance_stage", "charm", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	r = findRelationship(st, "romance", "aria", "player")
	if r.Attrs["affection"] != float64(60) || r.Machines["romance_stage"] != "smitten" {
		t.Fatalf("romance not advanced: affection=%v stage=%v", r.Attrs["affection"], r.Machines["romance_stage"])
	}
	// derived player_admirers should now be 1, and the smitten beat active.
	if v, _ := computeDerived(def, st, def.Derived["player_admirers"], ""); v != float64(1) {
		t.Fatalf("player_admirers = %v want 1", v)
	}
	if !beatActive(def, st, "smitten_beat") {
		t.Fatal("smitten beat should be active at affection 60")
	}

	// Leave together (guard: >=1 admirer) -> terminal ending reported.
	st, _, err = PerformAction(def, st, "arc", "leave_together", nil)
	if err != nil {
		t.Fatalf("leave_together: %v", err)
	}
	endings := EndingsReached(def, st)
	if len(endings) != 1 || endings[0].State != "ending_together" {
		t.Fatalf("expected ending_together, got %+v", endings)
	}
}

// beatActive reports whether a beat id is currently active.
func beatActive(def *Definition, st *State, id string) bool {
	for _, b := range ActiveBeats(def, st) {
		if b.ID == id {
			return true
		}
	}
	return false
}
