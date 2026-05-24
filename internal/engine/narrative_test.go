package engine

import "testing"

func beatDef() *Definition {
	tru := true
	fls := false
	return &Definition{
		ID: "g", Version: 1,
		World:    map[string]VarSpec{"day": {Type: "int", Default: float64(1)}},
		Machines: map[string]Machine{"arc": {Initial: "intro", States: []string{"intro", "bar"}}},
		Beats: map[string]Beat{
			"always_intro": {Text: "You arrive.", MachineState: &MachineStateRef{Machine: "arc", State: "intro"}},
			"day2_plus":    {Text: "A new day.", Guard: &Guard{Target: "world.day", Op: "gte", Value: float64(2)}},
			"once_beat":    {Text: "Once.", Once: &tru},
			"repeat_beat":  {Text: "Again.", Once: &fls},
		},
	}
}

func TestActiveBeats(t *testing.T) {
	def := beatDef()
	st, _ := NewInstance(def, "r", 1) // arc=intro, day=1

	ids := activeBeatIDs(def, st)
	if !ids["always_intro"] {
		t.Fatal("intro-bound beat should be active in intro")
	}
	if ids["day2_plus"] {
		t.Fatal("day2_plus should be inactive on day 1")
	}
	if !ids["once_beat"] || !ids["repeat_beat"] {
		t.Fatal("unbound beats should be active")
	}

	// Advance day; move out of intro.
	st.World["day"] = float64(2)
	st.Machines["arc"] = "bar"
	ids = activeBeatIDs(def, st)
	if ids["always_intro"] {
		t.Fatal("intro beat should be inactive in bar")
	}
	if !ids["day2_plus"] {
		t.Fatal("day2_plus should be active on day 2")
	}

	// Deliver the once beat; it should drop out. The repeatable one stays.
	st.DeliveredBeats = []string{"once_beat", "repeat_beat"}
	ids = activeBeatIDs(def, st)
	if ids["once_beat"] {
		t.Fatal("delivered once beat must not be active")
	}
	if !ids["repeat_beat"] {
		t.Fatal("delivered repeatable beat stays active")
	}
}

// activeBeatIDs is a test helper turning ActiveBeats into a set.
func activeBeatIDs(def *Definition, st *State) map[string]bool {
	out := map[string]bool{}
	for _, b := range ActiveBeats(def, st) {
		out[b.ID] = true
	}
	return out
}

func TestEndingsReached(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		Machines: map[string]Machine{
			"arc": {Initial: "intro", States: []string{"intro", "ending_good"},
				StateMeta: map[string]StateMeta{
					"ending_good": {Terminal: true, Ending: true, Description: "You win."},
				}},
			"side": {Initial: "a", States: []string{"a"}},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	if len(EndingsReached(def, st)) != 0 {
		t.Fatal("no ending at start")
	}
	st.Machines["arc"] = "ending_good"
	endings := EndingsReached(def, st)
	if len(endings) != 1 || endings[0].Machine != "arc" || endings[0].State != "ending_good" || endings[0].Description != "You win." {
		t.Fatalf("ending not reported: %+v", endings)
	}
}
