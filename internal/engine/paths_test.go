package engine

import "testing"

func stateWithData() *State {
	st, _ := NewInstance(miniDef(), "r", 1)
	st.World["day"] = float64(3)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(80)}, Inventory: map[string]int{"gold": 50}}
	st.Relationships = []*Relationship{{Type: "trust", From: "aria", To: "player", Attrs: map[string]any{"value": float64(5)}}}
	st.Machines["arc"] = "intro"
	return st
}

func TestResolvePath(t *testing.T) {
	st := stateWithData()
	ctx := &evalCtx{params: map[string]any{"amount": float64(7)}}
	cases := []struct {
		path string
		want any
	}{
		{"world.day", float64(3)},
		{"entity.player.health", float64(80)},
		{"inventory.player.gold", float64(50)},
		{"inventory.player.potion", float64(0)},
		{"rel.trust.aria.player.value", float64(5)},
		{"machine.arc.state", "intro"},
		{"param.amount", float64(7)},
	}
	for _, c := range cases {
		got, err := resolvePath(st, ctx, c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if got != c.want {
			t.Fatalf("%s = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestResolvePathMissingEntity(t *testing.T) {
	st := stateWithData()
	if _, err := resolvePath(st, &evalCtx{}, "entity.ghost.health"); err == nil {
		t.Fatal("expected error for missing entity")
	}
}

func TestResolveThisPaths(t *testing.T) {
	st := stateWithData() // player.health=80, gold=50; rel trust aria->player value=5
	// relationship host
	rel := findRelationship(st, "trust", "aria", "player")
	relCtx := &evalCtx{host: &hostRef{kind: "relationship", rel: rel}}
	if v, err := resolvePath(st, relCtx, "this.value"); err != nil || v != float64(5) {
		t.Fatalf("this.value = %v, %v", v, err)
	}
	if v, err := resolvePath(st, relCtx, "this.from"); err != nil || v != "aria" {
		t.Fatalf("this.from = %v, %v", v, err)
	}
	if v, err := resolvePath(st, relCtx, "this.to.health"); err != nil || v != float64(80) {
		t.Fatalf("this.to.health = %v, %v", v, err)
	}
	// entity host
	entCtx := &evalCtx{host: &hostRef{kind: "entity", id: "player", ent: st.Entities["player"]}}
	if v, err := resolvePath(st, entCtx, "this.health"); err != nil || v != float64(80) {
		t.Fatalf("this.health = %v, %v", v, err)
	}
	if v, err := resolvePath(st, entCtx, "this.id"); err != nil || v != "player" {
		t.Fatalf("this.id = %v, %v", v, err)
	}
	if v, err := resolvePath(st, entCtx, "this.inventory.gold"); err != nil || v != float64(50) {
		t.Fatalf("this.inventory.gold = %v, %v", v, err)
	}
	// no host -> error
	if _, err := resolvePath(st, &evalCtx{}, "this.health"); err == nil {
		t.Fatal("this.* without host should error")
	}
}

func TestResolveDerivedPaths(t *testing.T) {
	def, st := socialState()
	ctx := &evalCtx{def: def}

	got, err := resolvePath(st, ctx, "derived.admirers")
	if err != nil || got != float64(1) {
		t.Fatalf("derived.admirers = %v, %v", got, err)
	}
	got, err = resolvePath(st, ctx, "derived.top_admirer")
	if err != nil || got != "aria" {
		t.Fatalf("derived.top_admirer = %v, %v", got, err)
	}
	got, err = resolvePath(st, ctx, "entity.aria.derived.my_romances")
	if err != nil || got != float64(1) {
		t.Fatalf("entity.aria.derived.my_romances = %v, %v", got, err)
	}
	// A $self-derived read globally is an error.
	if _, err := resolvePath(st, ctx, "derived.my_romances"); err == nil {
		t.Fatal("expected error reading a $self derived value globally")
	}
	// Unknown derived name errors.
	if _, err := resolvePath(st, ctx, "derived.nope"); err == nil {
		t.Fatal("expected error for unknown derived")
	}
}

func TestResolveEquipmentPaths(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{"character": {
			Attributes: map[string]VarSpec{},
			Slots:      map[string]SlotSpec{"torso": {Accepts: []string{"dress"}}},
		}},
		ItemTypes: map[string]ItemType{"silk_dress": {Category: "dress", Equippable: true, Attributes: map[string]any{"style": float64(8)}}},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["p"] = &Entity{Type: "character", Attrs: map[string]any{}, Inventory: map[string]int{}, Equipped: map[string]string{"torso": "silk_dress"}}
	ctx := &evalCtx{def: def}

	if v, err := resolvePath(st, ctx, "equipped.p.torso"); err != nil || v != "silk_dress" {
		t.Fatalf("equipped = %v, %v", v, err)
	}
	if v, err := resolvePath(st, ctx, "worn.p.torso.style"); err != nil || v != float64(8) {
		t.Fatalf("worn.style = %v, %v", v, err)
	}
	if v, err := resolvePath(st, ctx, "itemtype.silk_dress.style"); err != nil || v != float64(8) {
		t.Fatalf("itemtype.style = %v, %v", v, err)
	}
	// empty slot: equipped is "", worn errors (absent)
	if v, _ := resolvePath(st, ctx, "equipped.p.head"); v != "" {
		t.Fatalf("empty slot equipped should be empty, got %v", v)
	}
	if _, err := resolvePath(st, ctx, "worn.p.head.style"); err == nil {
		t.Fatal("worn on empty slot should error")
	}
	// exists semantics: occupied slot true, empty slot false
	if !pathExists(st, ctx, "equipped.p.torso") {
		t.Fatal("occupied slot should exist")
	}
	if pathExists(st, ctx, "equipped.p.head") {
		t.Fatal("empty slot should not exist")
	}
	// this.equipped for an entity host
	hctx := &evalCtx{def: def, host: &hostRef{kind: "entity", id: "p", ent: st.Entities["p"]}}
	if v, err := resolvePath(st, hctx, "this.equipped.torso"); err != nil || v != "silk_dress" {
		t.Fatalf("this.equipped = %v, %v", v, err)
	}
}
