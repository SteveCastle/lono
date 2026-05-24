package engine

import "testing"

func miniDef() *Definition {
	min := 1.0
	return &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{"day": {Type: "int", Default: float64(1), Min: &min}},
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{"health": {Type: "int", Default: float64(100)}}},
		},
		Machines: map[string]Machine{
			"arc": {Initial: "intro", States: []string{"intro", "end"}},
		},
	}
}

func TestNewInstanceSeedsDefaults(t *testing.T) {
	st, err := NewInstance(miniDef(), "run1", 42)
	if err != nil {
		t.Fatal(err)
	}
	if st.ID != "run1" || st.GameID != "g" || st.Seed != 42 {
		t.Fatalf("bad header: %+v", st)
	}
	if st.World["day"] != float64(1) {
		t.Fatalf("world default not seeded: %v", st.World["day"])
	}
	if st.Machines["arc"] != "intro" {
		t.Fatalf("machine not at initial: %v", st.Machines["arc"])
	}
}

func TestCloneIsDeep(t *testing.T) {
	st, _ := NewInstance(miniDef(), "run1", 42)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	cp := st.Clone()
	cp.World["day"] = float64(99)
	cp.Entities["player"].Attrs["health"] = float64(1)
	if st.World["day"] == float64(99) {
		t.Fatal("world not deep-copied")
	}
	if st.Entities["player"].Attrs["health"] == float64(1) {
		t.Fatal("entity not deep-copied")
	}
}
