package engine

import "testing"

func defWithActions() *Definition {
	def := defForEffects()
	def.Machines = map[string]Machine{
		"arc": {Initial: "intro", States: []string{"intro", "rising"},
			Transitions: []Transition{
				{ID: "open", From: StateSet{"intro"}, To: "rising",
					Guard: &Guard{Target: "entity.player.health", Op: "gt", Value: float64(0)}},
				{ID: "blocked", From: StateSet{"intro"}, To: "rising",
					Guard: &Guard{Target: "world.alarm", Op: "eq", Value: true}},
				{ID: "with_param", From: StateSet{"intro"}, To: "rising",
					Params: map[string]VarSpec{"amt": {Type: "int"}},
					Guard:  &Guard{Target: "param.amt", Op: "gt", Value: float64(0)}},
				{ID: "elsewhere", From: StateSet{"rising"}, To: "intro"},
			}},
	}
	return def
}

func TestAvailableActions(t *testing.T) {
	def := defWithActions()
	st, _ := NewInstance(def, "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}

	actions, err := AvailableActions(def, st)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ActionInfo{}
	for _, a := range actions {
		got[a.Action] = a
	}
	if _, ok := got["elsewhere"]; ok {
		t.Fatal("transition from another state must not be listed")
	}
	if !got["open"].Enabled {
		t.Fatal("open should be enabled (health>0)")
	}
	if got["blocked"].Enabled {
		t.Fatalf("blocked should be disabled, reason=%q", got["blocked"].Reason)
	}
	if !got["with_param"].Enabled || !got["with_param"].RequiresParams {
		t.Fatal("param-gated action should be listed as enabled+requiresParams")
	}
}

func TestAvailableActionsAttached(t *testing.T) {
	def := defWithTrust()
	def.Machines = map[string]Machine{
		"romance": {Attach: &AttachSpec{To: "relationshipType:trust"},
			Initial: "friends", States: []string{"friends", "dating"},
			Transitions: []Transition{{ID: "start_dating", From: StateSet{"friends"}, To: "dating",
				Guard: &Guard{Target: "this.value", Op: "gte", Value: float64(50)}}}},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{{Type: "trust", From: "aria", To: "player",
		Attrs: map[string]any{"value": float64(60)}, Machines: map[string]string{"romance": "friends"}}}

	actions, err := AvailableActions(def, st)
	if err != nil {
		t.Fatal(err)
	}
	var found *ActionInfo
	for i := range actions {
		if actions[i].Action == "start_dating" {
			found = &actions[i]
		}
	}
	if found == nil {
		t.Fatal("attached action not listed")
	}
	if found.Host == nil || found.Host.Kind != "relationship" || found.Host.From != "aria" || found.Host.To != "player" {
		t.Fatalf("attached action missing host: %+v", found.Host)
	}
	if !found.Enabled {
		t.Fatalf("should be enabled (value 60 >= 50): %+v", found)
	}
}

// feasibilityDef builds a map-based game whose move transitions' guards all pass
// from "study", so enabledness is decided purely by whether the move effect can
// actually execute (exit existence / locked).
func feasibilityDef() (*Definition, *State) {
	def := &Definition{
		ID: "feas", Version: 1,
		EntityTypes: map[string]EntityType{
			"location": {Attributes: map[string]VarSpec{"name": {Type: "string"}}},
			"person": {Attributes: map[string]VarSpec{
				"location": {Type: "ref", RefType: "location"},
			}},
		},
		RelationshipTypes: map[string]RelType{
			"exit": {From: "location", To: "location", Directed: true,
				Attributes: map[string]VarSpec{"locked": {Type: "bool", Default: false}}},
		},
		Machines: map[string]Machine{
			"arc": {Initial: "exploring", States: []string{"exploring"},
				Transitions: []Transition{
					// guard passes (in study), and study→hall edge exists → feasible
					{ID: "go_hall", From: StateSet{"exploring"}, To: "exploring",
						Guard:   &Guard{Target: "entity.player.location", Op: "eq", Value: "study"},
						Effects: []Effect{{Op: "move", Entity: "player", To: "hall", Via: "exit"}}},
					// guard passes (in study), but no study→garden edge → infeasible
					{ID: "go_garden", From: StateSet{"exploring"}, To: "exploring",
						Guard:   &Guard{Target: "entity.player.location", Op: "eq", Value: "study"},
						Effects: []Effect{{Op: "move", Entity: "player", To: "garden", Via: "exit"}}},
					// guard passes, edge exists but is locked → infeasible
					{ID: "go_cellar", From: StateSet{"exploring"}, To: "exploring",
						Guard:   &Guard{Target: "entity.player.location", Op: "eq", Value: "study"},
						Effects: []Effect{{Op: "move", Entity: "player", To: "cellar", Via: "exit"}}},
				}},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["study"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Study"}, Inventory: map[string]int{}}
	st.Entities["hall"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Hall"}, Inventory: map[string]int{}}
	st.Entities["garden"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Garden"}, Inventory: map[string]int{}}
	st.Entities["cellar"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Cellar"}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "person", Attrs: map[string]any{"location": "study"}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{
		{Type: "exit", From: "study", To: "hall", Attrs: map[string]any{"locked": false}},
		{Type: "exit", From: "study", To: "cellar", Attrs: map[string]any{"locked": true}},
	}
	return def, st
}

// TestActionEnabledRequiresFeasibleEffects is the G1 fix: an action whose guard
// passes but whose effects cannot execute (no exit, or a locked exit) must be
// reported enabled:false, not enabled:true-then-fail-on-do.
func TestActionEnabledRequiresFeasibleEffects(t *testing.T) {
	def, st := feasibilityDef()
	actions, err := AvailableActions(def, st)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ActionInfo{}
	for _, a := range actions {
		got[a.Action] = a
	}
	if !got["go_hall"].Enabled {
		t.Fatalf("go_hall should be enabled (study→hall exists): %+v", got["go_hall"])
	}
	if got["go_garden"].Enabled {
		t.Fatal("go_garden should be disabled (no study→garden exit)")
	}
	if got["go_garden"].Reason == "" {
		t.Fatal("disabled go_garden should carry the effect error as its reason")
	}
	if got["go_cellar"].Enabled {
		t.Fatalf("go_cellar should be disabled (study→cellar is locked): reason=%q", got["go_cellar"].Reason)
	}
	// And confirm the real state was not mutated by the dry-run.
	if st.Entities["player"].Attrs["location"] != "study" {
		t.Fatalf("feasibility dry-run leaked state: player at %v", st.Entities["player"].Attrs["location"])
	}
}

// TestActionEnabledParamActionNotDryRun confirms param-taking actions are still
// listed enabled+requiresParams (we can't dry-run them without params).
func TestActionEnabledParamActionNotDryRun(t *testing.T) {
	def, st := feasibilityDef()
	arc := def.Machines["arc"]
	arc.Transitions = append(arc.Transitions, Transition{
		ID: "teleport", From: StateSet{"exploring"}, To: "exploring",
		Params:  map[string]VarSpec{"dest": {Type: "string"}},
		Guard:   &Guard{Target: "param.dest", Op: "neq", Value: ""},
		Effects: []Effect{{Op: "move", Entity: "player", To: "garden", Via: "exit"}},
	})
	def.Machines["arc"] = arc

	actions, err := AvailableActions(def, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.Action == "teleport" {
			if !a.Enabled || !a.RequiresParams {
				t.Fatalf("param action should stay enabled+requiresParams, not dry-run-disabled: %+v", a)
			}
			return
		}
	}
	t.Fatal("teleport action not listed")
}

// AvailableActions must always return a JSON array (never nil), so consumers can
// treat "no actions" (e.g. a terminal/ending state) as [] rather than null.
func TestAvailableActionsNeverNil(t *testing.T) {
	def := &Definition{ID: "g", Version: 1, Machines: map[string]Machine{
		"arc": {Initial: "end", States: []string{"end"}},
	}}
	st, _ := NewInstance(def, "r", 1)
	acts, err := AvailableActions(def, st)
	if err != nil {
		t.Fatal(err)
	}
	if acts == nil {
		t.Fatal("AvailableActions returned nil; want empty slice")
	}
	if len(acts) != 0 {
		t.Fatalf("want 0 actions in a terminal state, got %d", len(acts))
	}
}
