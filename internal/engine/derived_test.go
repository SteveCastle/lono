package engine

import "testing"

func socialState() (*Definition, *State) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{"character": {Attributes: map[string]VarSpec{}}},
		RelationshipTypes: map[string]RelType{
			"romance": {From: "character", To: "character",
				Attributes: map[string]VarSpec{"attraction": {Type: "int", Default: float64(0)}}},
		},
		Derived: map[string]DerivedSpec{
			"admirers": {Over: "relationships",
				Where:  WhereSpec{Type: "romance", To: "player", Attrs: []AttrPred{{Attr: "attraction", Op: "gte", Value: float64(80)}}},
				Reduce: "count"},
			"any_admirer": {Over: "relationships",
				Where:  WhereSpec{Type: "romance", To: "player", Attrs: []AttrPred{{Attr: "attraction", Op: "gte", Value: float64(80)}}},
				Reduce: "any"},
			"top_admirer": {Over: "relationships",
				Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "argmax:attraction"},
			"total_attraction": {Over: "relationships",
				Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "sum:attraction"},
			"my_romances": {Over: "relationships",
				Where: WhereSpec{Type: "romance", From: "$self"}, Reduce: "count"},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{}, Inventory: map[string]int{}}
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{}, Inventory: map[string]int{}}
	st.Entities["mara"] = &Entity{Type: "character", Attrs: map[string]any{}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{
		{Type: "romance", From: "aria", To: "player", Attrs: map[string]any{"attraction": float64(90)}},
		{Type: "romance", From: "mara", To: "player", Attrs: map[string]any{"attraction": float64(40)}},
	}
	return def, st
}

func TestComputeDerived(t *testing.T) {
	def, st := socialState()
	check := func(name string, self string, want any) {
		got, err := computeDerived(def, st, def.Derived[name], self)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s = %v (want %v)", name, got, want)
		}
	}
	check("admirers", "", float64(1)) // only aria >= 80
	check("any_admirer", "", true)
	check("total_attraction", "", float64(130))
	check("top_admirer", "", "aria")         // argmax returns the matching rel's From id
	check("my_romances", "aria", float64(1)) // aria has 1 outgoing romance
	check("my_romances", "mara", float64(1))
	check("my_romances", "player", float64(0))
}

func TestComputeDerivedEntities(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{"character": {Attributes: map[string]VarSpec{"alive": {Type: "bool", Default: true}}}},
		Derived: map[string]DerivedSpec{
			"living": {Over: "entities", Where: WhereSpec{Type: "character", Attrs: []AttrPred{{Attr: "alive", Op: "eq", Value: true}}}, Reduce: "count"},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["a"] = &Entity{Type: "character", Attrs: map[string]any{"alive": true}, Inventory: map[string]int{}}
	st.Entities["b"] = &Entity{Type: "character", Attrs: map[string]any{"alive": false}, Inventory: map[string]int{}}
	got, err := computeDerived(def, st, def.Derived["living"], "")
	if err != nil {
		t.Fatal(err)
	}
	if got != float64(1) {
		t.Fatalf("living = %v want 1", got)
	}
}

func TestDerivedView(t *testing.T) {
	def, st := socialState()
	view := BuildDerivedView(def, st)
	if view.Global["admirers"] != float64(1) {
		t.Fatalf("global admirers = %v", view.Global["admirers"])
	}
	if _, ok := view.Global["my_romances"]; ok {
		t.Fatal("per-entity derived must not appear in Global")
	}
	if view.ByEntity["aria"]["my_romances"] != float64(1) {
		t.Fatalf("aria my_romances = %v", view.ByEntity["aria"]["my_romances"])
	}
}

func TestComputeDerivedEmptyReducers(t *testing.T) {
	def, st := socialState()
	st.Relationships = nil // no matches
	for name, want := range map[string]any{
		"admirers": float64(0), "any_admirer": false, "total_attraction": float64(0), "top_admirer": "",
	} {
		got, err := computeDerived(def, st, def.Derived[name], "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Fatalf("empty %s = %v want %v", name, got, want)
		}
	}
}

func TestComputeDerivedMinMaxArg(t *testing.T) {
	def, st := socialState()
	def.Derived["max_attr"] = DerivedSpec{Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "max:attraction"}
	def.Derived["min_attr"] = DerivedSpec{Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "min:attraction"}
	def.Derived["least_admirer"] = DerivedSpec{Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "argmin:attraction"}
	check := func(name string, want any) {
		got, err := computeDerived(def, st, def.Derived[name], "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s=%v want %v", name, got, want)
		}
	}
	check("max_attr", float64(90))
	check("min_attr", float64(40))
	check("least_admirer", "mara")
	st.Relationships = nil
	check("max_attr", float64(0))
	check("min_attr", float64(0))
	check("least_admirer", "")
}

// TestDerivedListReducer verifies the new "list" reducer.
func TestDerivedListReducer(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"alive": {Type: "bool", Default: true},
				"score": {Type: "int", Default: float64(0)},
			}},
		},
		Derived: map[string]DerivedSpec{
			"living_list": {Over: "entities", Where: WhereSpec{Type: "character",
				Attrs: []AttrPred{{Attr: "alive", Op: "eq", Value: true}}},
				Reduce: "list"},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["a"] = &Entity{Type: "character", Attrs: map[string]any{"alive": true, "score": float64(1)}, Inventory: map[string]int{}}
	st.Entities["b"] = &Entity{Type: "character", Attrs: map[string]any{"alive": false, "score": float64(2)}, Inventory: map[string]int{}}
	st.Entities["c"] = &Entity{Type: "character", Attrs: map[string]any{"alive": true, "score": float64(3)}, Inventory: map[string]int{}}

	got, err := computeDerived(def, st, def.Derived["living_list"], "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("list result is %T, want []any", got)
	}
	// Entities are iterated in sorted order: a, c (b is dead).
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "c" {
		t.Fatalf("list = %v, want [a c]", arr)
	}
}

// TestDerivedPathOperandInAttr tests $path resolution in attr-predicate values.
func TestDerivedPathOperandInAttr(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"person": {Attributes: map[string]VarSpec{
				"location": {Type: "string"},
			}},
		},
		Derived: map[string]DerivedSpec{
			"here": {
				Over: "entities",
				Where: WhereSpec{
					Type: "person",
					Attrs: []AttrPred{{
						Attr:  "location",
						Op:    "eq",
						Value: map[string]any{"$path": "entity.player.location"},
					}},
				},
				Reduce: "list",
			},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["player"] = &Entity{Type: "person", Attrs: map[string]any{"location": "study"}, Inventory: map[string]int{}}
	st.Entities["npc"] = &Entity{Type: "person", Attrs: map[string]any{"location": "study"}, Inventory: map[string]int{}}
	st.Entities["other"] = &Entity{Type: "person", Attrs: map[string]any{"location": "hall"}, Inventory: map[string]int{}}

	got, err := computeDerived(def, st, def.Derived["here"], "")
	if err != nil {
		t.Fatalf("here: %v", err)
	}
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("here result is %T, want []any", got)
	}
	// player and npc are at study; sorted → [npc, player].
	if len(arr) != 2 {
		t.Fatalf("here = %v, want 2 entries", arr)
	}
	if arr[0] != "npc" || arr[1] != "player" {
		t.Fatalf("here = %v, want [npc player]", arr)
	}
}

// TestDerivedExitsFrom tests from-anchored counterpart rule: from:$path → return to.
func TestDerivedExitsFrom(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"location": {Attributes: map[string]VarSpec{"name": {Type: "string"}}},
			"person":   {Attributes: map[string]VarSpec{"location": {Type: "string"}}},
		},
		RelationshipTypes: map[string]RelType{
			"exit": {From: "location", To: "location", Directed: true},
		},
		Derived: map[string]DerivedSpec{
			"exits_here": {
				Over: "relationships",
				Where: WhereSpec{
					Type: "exit",
					From: map[string]any{"$path": "entity.player.location"},
				},
				Reduce: "list",
			},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["study"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Study"}, Inventory: map[string]int{}}
	st.Entities["hall"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Hall"}, Inventory: map[string]int{}}
	st.Entities["garden"] = &Entity{Type: "location", Attrs: map[string]any{"name": "Garden"}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "person", Attrs: map[string]any{"location": "study"}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{
		{Type: "exit", From: "study", To: "hall", Attrs: map[string]any{}},
		{Type: "exit", From: "hall", To: "study", Attrs: map[string]any{}},
		{Type: "exit", From: "hall", To: "garden", Attrs: map[string]any{}},
		{Type: "exit", From: "garden", To: "hall", Attrs: map[string]any{}},
	}

	// exits_here: from=player.location(="study") → counterpart = To → returns ["hall"]
	got, err := computeDerived(def, st, def.Derived["exits_here"], "")
	if err != nil {
		t.Fatalf("exits_here from study: %v", err)
	}
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("exits_here result is %T, want []any", got)
	}
	if len(arr) != 1 || arr[0] != "hall" {
		t.Fatalf("exits_here from study = %v, want [hall]", arr)
	}
}

// TestDerivedToAnchoredArgmaxStillReturnsFrom verifies the v3 counterpart rule:
// to-anchored → return from (existing behaviour kept green).
func TestDerivedToAnchoredArgmaxStillReturnsFrom(t *testing.T) {
	def, st := socialState()
	// top_admirer has To:"player" (to-anchored) → argmax should return From id.
	got, err := computeDerived(def, st, def.Derived["top_admirer"], "")
	if err != nil {
		t.Fatalf("top_admirer: %v", err)
	}
	if got != "aria" {
		t.Fatalf("top_admirer = %v, want aria (to-anchored should return from)", got)
	}
}
