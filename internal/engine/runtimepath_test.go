package engine

import (
	"testing"
)

// minDef returns a Definition sufficient for runtime path tests.
func minDef() *Definition {
	minF := float64(0)
	maxF := float64(100)
	return &Definition{
		ID:      "test",
		Version: 1,
		World: map[string]VarSpec{
			"day": {Type: "int", Default: float64(1)},
		},
		EntityTypes: map[string]EntityType{
			"character": {
				Attributes: map[string]VarSpec{
					"health": {Type: "int", Default: float64(100), Min: &minF, Max: &maxF},
				},
				Slots: map[string]SlotSpec{
					"hand": {Accepts: []string{"weapon"}},
				},
			},
		},
		ItemTypes: map[string]ItemType{
			"sword": {Equippable: true, Category: "weapon"},
		},
		RelationshipTypes: map[string]RelType{
			"ally": {
				From:     "character",
				To:       "character",
				Directed: false,
				Attributes: map[string]VarSpec{
					"trust": {Type: "float", Default: float64(0)},
				},
			},
		},
		Machines: map[string]Machine{
			"arc": {
				Initial: "intro",
				States:  []string{"intro", "end"},
			},
			"bond": {
				Attach:  &AttachSpec{To: "entityType:character"},
				Initial: "neutral",
				States:  []string{"neutral", "allied"},
			},
			"link": {
				Attach:  &AttachSpec{To: "relationshipType:ally"},
				Initial: "weak",
				States:  []string{"weak", "strong"},
			},
		},
	}
}

// minState returns a State consistent with minDef.
func minState() *State {
	return &State{
		World:    map[string]any{"day": float64(1)},
		Machines: map[string]string{"arc": "intro"},
		Entities: map[string]*Entity{
			"player": {
				Type:      "character",
				Attrs:     map[string]any{"health": float64(50)},
				Inventory: map[string]int{"sword": 2},
				Equipped:  map[string]string{},
				Machines:  map[string]string{"bond": "neutral"},
			},
			"npc": {
				Type:      "character",
				Attrs:     map[string]any{"health": float64(80)},
				Inventory: map[string]int{},
				Equipped:  map[string]string{},
				Machines:  map[string]string{"bond": "neutral"},
			},
		},
		Relationships: []*Relationship{
			{
				Type:     "ally",
				From:     "player",
				To:       "npc",
				Attrs:    map[string]any{"trust": float64(10)},
				Machines: map[string]string{"link": "weak"},
			},
		},
	}
}

// ---- CompileRuntimeWrite tests ----

func TestCompileRuntimeWrite_WorldSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "world/day", float64(3), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set" || ops[0].Target != "world.day" || ops[0].Value != float64(3) {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_EntityAttrSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "entities/player/attrs/health", float64(75), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set" || ops[0].Target != "entity.player.health" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_InventoryDeltaUp(t *testing.T) {
	def, st := minDef(), minState() // player has 2 swords
	ops, err := CompileRuntimeWrite(def, st, "entities/player/inventory/sword", float64(5), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "add_item" || ops[0].Count != 3 {
		t.Fatalf("expected add_item count=3, got: %+v", ops)
	}
}

func TestCompileRuntimeWrite_InventoryDeltaDown(t *testing.T) {
	def, st := minDef(), minState() // player has 2 swords
	ops, err := CompileRuntimeWrite(def, st, "entities/player/inventory/sword", float64(1), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "remove_item" || ops[0].Count != 1 {
		t.Fatalf("expected remove_item count=1, got: %+v", ops)
	}
}

func TestCompileRuntimeWrite_InventoryNoOp(t *testing.T) {
	def, st := minDef(), minState() // player has 2 swords
	ops, err := CompileRuntimeWrite(def, st, "entities/player/inventory/sword", float64(2), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected no-op, got: %+v", ops)
	}
}

func TestCompileRuntimeWrite_EquippedSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "entities/player/equipped/hand", "sword", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "equip" || ops[0].Entity != "player" || ops[0].Slot != "hand" || ops[0].Item != "sword" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_EntityMachineSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "entities/player/machines/bond", "allied", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set_attached_state" || ops[0].Machine != "bond" || ops[0].Entity != "player" || ops[0].State != "allied" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_RelationshipAttrSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "relationships/ally/player/npc/attrs/trust", float64(50), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set_relationship" || ops[0].RelType != "ally" || ops[0].From != "player" || ops[0].To != "npc" {
		t.Fatalf("wrong op: %+v", ops)
	}
	if ops[0].Attrs["trust"] != float64(50) {
		t.Fatalf("wrong attrs: %+v", ops[0].Attrs)
	}
}

func TestCompileRuntimeWrite_RelationshipMachineSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "relationships/ally/player/npc/machines/link", "strong", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set_attached_state" || ops[0].Machine != "link" || ops[0].From != "player" || ops[0].To != "npc" || ops[0].State != "strong" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_GlobalMachineSet(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "machines/arc", "end", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "set_machine_state" || ops[0].Machine != "arc" || ops[0].State != "end" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

// ---- Remove variants ----

func TestCompileRuntimeWrite_RemoveEntity(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "entities/player", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "destroy_entity" || ops[0].ID != "player" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_RemoveRelationship(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "relationships/ally/player/npc", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "remove_relationship" || ops[0].RelType != "ally" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_RemoveEquipped(t *testing.T) {
	def, st := minDef(), minState()
	ops, err := CompileRuntimeWrite(def, st, "entities/player/equipped/hand", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "unequip" || ops[0].Entity != "player" || ops[0].Slot != "hand" {
		t.Fatalf("wrong op: %+v", ops)
	}
}

func TestCompileRuntimeWrite_RemoveInventory_FullCount(t *testing.T) {
	def, st := minDef(), minState() // player has 2 swords
	ops, err := CompileRuntimeWrite(def, st, "entities/player/inventory/sword", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != "remove_item" || ops[0].Count != 2 {
		t.Fatalf("expected remove_item count=2, got: %+v", ops)
	}
}

func TestCompileRuntimeWrite_RemoveInventory_ZeroNoOp(t *testing.T) {
	def, st := minDef(), minState()
	// npc has no swords
	ops, err := CompileRuntimeWrite(def, st, "entities/npc/inventory/sword", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("expected no-op for zero inventory remove, got: %+v", ops)
	}
}

// ---- Read-only / unknown paths → errPathNotWritable ----

func TestCompileRuntimeWrite_DerivedRejected(t *testing.T) {
	def, st := minDef(), minState()
	_, err := CompileRuntimeWrite(def, st, "derived/someValue", float64(1), false)
	if !IsPathNotWritable(err) {
		t.Fatalf("expected errPathNotWritable, got: %v", err)
	}
}

func TestCompileRuntimeWrite_BeatsRejected(t *testing.T) {
	def, st := minDef(), minState()
	_, err := CompileRuntimeWrite(def, st, "beats/intro", float64(1), false)
	if !IsPathNotWritable(err) {
		t.Fatalf("expected errPathNotWritable, got: %v", err)
	}
}

func TestCompileRuntimeWrite_ActionsRejected(t *testing.T) {
	def, st := minDef(), minState()
	_, err := CompileRuntimeWrite(def, st, "actions/finish", nil, false)
	if !IsPathNotWritable(err) {
		t.Fatalf("expected errPathNotWritable, got: %v", err)
	}
}

func TestCompileRuntimeWrite_EndingReachedRejected(t *testing.T) {
	def, st := minDef(), minState()
	_, err := CompileRuntimeWrite(def, st, "endingReached", true, false)
	if !IsPathNotWritable(err) {
		t.Fatalf("expected errPathNotWritable, got: %v", err)
	}
}

func TestCompileRuntimeWrite_UnknownRootRejected(t *testing.T) {
	def, st := minDef(), minState()
	_, err := CompileRuntimeWrite(def, st, "bogus/thing", nil, false)
	if !IsPathNotWritable(err) {
		t.Fatalf("expected errPathNotWritable, got: %v", err)
	}
}

// ---- ForceWrite tests ----

func TestForceWrite_SetsEntityAttr(t *testing.T) {
	st := minState()
	// Force health to 999 (out of range but raw write bypasses validation).
	if err := ForceWrite(st, "entities/player/attrs/health", float64(999), false); err != nil {
		t.Fatalf("ForceWrite failed: %v", err)
	}
	if st.Entities["player"].Attrs["health"] != float64(999) {
		t.Fatalf("expected health=999, got %v", st.Entities["player"].Attrs["health"])
	}
}

func TestForceWrite_SetsWorldVar(t *testing.T) {
	st := minState()
	if err := ForceWrite(st, "world/day", float64(42), false); err != nil {
		t.Fatalf("ForceWrite failed: %v", err)
	}
	if st.World["day"] != float64(42) {
		t.Fatalf("expected day=42, got %v", st.World["day"])
	}
}

func TestForceWrite_RemovesEntityAttr(t *testing.T) {
	st := minState()
	if err := ForceWrite(st, "entities/player/attrs/health", nil, true); err != nil {
		t.Fatalf("ForceWrite remove failed: %v", err)
	}
	if _, ok := st.Entities["player"].Attrs["health"]; ok {
		t.Fatal("expected health attr to be removed")
	}
}

func TestForceWrite_MissingIntermediateErrors(t *testing.T) {
	st := minState()
	err := ForceWrite(st, "entities/ghost/attrs/health", float64(50), false)
	if err == nil {
		t.Fatal("expected error for missing intermediate 'ghost'")
	}
}

func TestForceWrite_ArrayPathErrors(t *testing.T) {
	st := minState()
	// relationships is a []any after marshal — navigating into it as a map key
	// for a nested key should error.
	err := ForceWrite(st, "relationships/0/type", "ally2", false)
	if err == nil {
		t.Fatal("expected error for array-valued intermediate path")
	}
}
