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
