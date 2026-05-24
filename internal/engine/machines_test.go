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
