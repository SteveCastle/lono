package engine

import "testing"

func defForEffects() *Definition {
	zero, hundred := 0.0, 100.0
	return &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"day":   {Type: "int", Default: float64(1)},
			"alarm": {Type: "bool", Default: false},
		},
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"health": {Type: "int", Default: float64(100), Min: &zero, Max: &hundred},
			}},
		},
	}
}

func stateForEffects() *State {
	st, _ := NewInstance(defForEffects(), "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	return st
}

func TestScalarEffects(t *testing.T) {
	def := defForEffects()
	st := stateForEffects()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	if err := applyEffect(def, st, ctx, Effect{Op: "set", Target: "world.alarm", Value: true}); err != nil {
		t.Fatal(err)
	}
	if st.World["alarm"] != true {
		t.Fatalf("set failed: %v", st.World["alarm"])
	}

	if err := applyEffect(def, st, ctx, Effect{Op: "dec", Target: "entity.player.health", Value: float64(30)}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["player"].Attrs["health"] != float64(70) {
		t.Fatalf("dec failed: %v", st.Entities["player"].Attrs["health"])
	}
}

func TestScalarEffectBoundsRejected(t *testing.T) {
	def := defForEffects()
	st := stateForEffects()
	ctx := newEvalCtx(nil, &RNG{state: 1})
	err := applyEffect(def, st, ctx, Effect{Op: "inc", Target: "entity.player.health", Value: float64(50)})
	if err == nil {
		t.Fatal("expected bounds violation (health max 100)")
	}
	if st.Entities["player"].Attrs["health"] != float64(100) {
		t.Fatal("failed effect must not mutate state")
	}
}

func TestRollValueReference(t *testing.T) {
	def := defForEffects()
	st := stateForEffects()
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.rolls["dmg"] = 25
	if err := applyEffect(def, st, ctx, Effect{Op: "dec", Target: "entity.player.health", Value: map[string]any{"$roll": "dmg"}}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["player"].Attrs["health"] != float64(75) {
		t.Fatalf("roll-ref value failed: %v", st.Entities["player"].Attrs["health"])
	}
}

func TestInventoryEffects(t *testing.T) {
	def := defForEffects()
	def.ItemTypes = map[string]ItemType{"gold": {}, "potion": {MaxStack: intp(3)}}
	st := stateForEffects()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	if err := applyEffect(def, st, ctx, Effect{Op: "add_item", Entity: "player", Item: "gold", Count: 50}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["player"].Inventory["gold"] != 50 {
		t.Fatalf("add_item failed: %v", st.Entities["player"].Inventory["gold"])
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_item", Entity: "player", Item: "gold", Count: 20}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["player"].Inventory["gold"] != 30 {
		t.Fatalf("remove_item failed: %v", st.Entities["player"].Inventory["gold"])
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_item", Entity: "player", Item: "gold", Count: 999}); err == nil {
		t.Fatal("expected underflow error")
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "add_item", Entity: "player", Item: "potion", Count: 5}); err == nil {
		t.Fatal("expected maxStack(3) violation")
	}
}

func intp(i int) *int { return &i }

func TestEntityEffects(t *testing.T) {
	def := defForEffects()
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})

	err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "character", ID: "aria",
		Attrs: map[string]any{"health": float64(90)}})
	if err != nil {
		t.Fatal(err)
	}
	aria := st.Entities["aria"]
	if aria == nil || aria.Type != "character" || aria.Attrs["health"] != float64(90) {
		t.Fatalf("create_entity failed: %+v", aria)
	}

	if err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "character", ID: "aria"}); err == nil {
		t.Fatal("expected duplicate id error")
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "ghost", ID: "x"}); err == nil {
		t.Fatal("expected unknown type error")
	}

	if err := applyEffect(def, st, ctx, Effect{Op: "destroy_entity", ID: "aria"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Entities["aria"]; ok {
		t.Fatal("destroy_entity left entity behind")
	}
}

func TestCreateEntityAppliesAttrDefaults(t *testing.T) {
	def := defForEffects()
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})
	if err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "character", ID: "p"}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["p"].Attrs["health"] != float64(100) {
		t.Fatalf("default not applied: %v", st.Entities["p"].Attrs["health"])
	}
}

func defWithTrust() *Definition {
	def := defForEffects()
	lo, hi := -100.0, 100.0
	def.RelationshipTypes = map[string]RelType{
		"trust": {From: "character", To: "character", Directed: true,
			Attributes: map[string]VarSpec{"value": {Type: "int", Default: float64(0), Min: &lo, Max: &hi}}},
	}
	return def
}

func TestRelationshipEffects(t *testing.T) {
	def := defWithTrust()
	st, _ := NewInstance(def, "r", 1)
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	ctx := newEvalCtx(nil, &RNG{state: 1})

	by := 3.0
	if err := applyEffect(def, st, ctx, Effect{Op: "adjust_relationship", RelType: "trust", From: "aria", To: "player", Attr: "value", By: &by}); err != nil {
		t.Fatal(err)
	}
	r := findRelationship(st, "trust", "aria", "player")
	if r == nil || r.Attrs["value"] != float64(3) {
		t.Fatalf("adjust created/updated wrong: %+v", r)
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "set_relationship", RelType: "trust", From: "aria", To: "player", Attrs: map[string]any{"value": float64(10)}}); err != nil {
		t.Fatal(err)
	}
	if findRelationship(st, "trust", "aria", "player").Attrs["value"] != float64(10) {
		t.Fatal("set_relationship failed")
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_relationship", RelType: "trust", From: "aria", To: "player"}); err != nil {
		t.Fatal(err)
	}
	if findRelationship(st, "trust", "aria", "player") != nil {
		t.Fatal("remove_relationship failed")
	}
}

func TestAttachedMachinesInitOnCreate(t *testing.T) {
	def := defForEffects()
	def.Machines = map[string]Machine{
		"mood": {Attach: &AttachSpec{To: "entityType:character"}, Initial: "calm", States: []string{"calm", "angry"}},
	}
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})
	if err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "character", ID: "npc"}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["npc"].Machines["mood"] != "calm" {
		t.Fatalf("entity-attached machine not initialized: %+v", st.Entities["npc"].Machines)
	}
}

func TestAttachedRelMachineInitOnCreate(t *testing.T) {
	def := defWithTrust()
	def.Machines = map[string]Machine{
		"bond": {Attach: &AttachSpec{To: "relationshipType:trust"}, Initial: "new", States: []string{"new", "close"}},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["a"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["b"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	if err := applyEffect(def, st, ctx, Effect{Op: "set_relationship", RelType: "trust", From: "a", To: "b", Attrs: map[string]any{"value": float64(1)}}); err != nil {
		t.Fatal(err)
	}
	r := findRelationship(st, "trust", "a", "b")
	if r.Machines["bond"] != "new" {
		t.Fatalf("rel-attached machine not initialized: %+v", r.Machines)
	}
}

func TestThisWriteTargetAndSetAttached(t *testing.T) {
	def := defWithTrust()
	def.Machines = map[string]Machine{
		"bond": {Attach: &AttachSpec{To: "relationshipType:trust"}, Initial: "new", States: []string{"new", "close"}},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["a"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["b"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	rel := &Relationship{Type: "trust", From: "a", To: "b", Attrs: map[string]any{"value": float64(5)}, Machines: map[string]string{"bond": "new"}}
	st.Relationships = []*Relationship{rel}

	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.host = &hostRef{kind: "relationship", rel: rel}
	// inc this.value (a relationship attribute, bounded -100..100)
	if err := applyEffect(def, st, ctx, Effect{Op: "inc", Target: "this.value", Value: float64(10)}); err != nil {
		t.Fatal(err)
	}
	if rel.Attrs["value"] != float64(15) {
		t.Fatalf("this.value inc failed: %v", rel.Attrs["value"])
	}
	// set_attached_state forces the bond machine
	if err := applyEffect(def, st, ctx, Effect{Op: "set_attached_state", Machine: "bond", From: "a", To: "b", State: "close"}); err != nil {
		t.Fatal(err)
	}
	if rel.Machines["bond"] != "close" {
		t.Fatalf("set_attached_state failed: %v", rel.Machines["bond"])
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "set_attached_state", Machine: "bond", From: "a", To: "b", State: "bogus"}); err == nil {
		t.Fatal("expected invalid state error")
	}
}

func TestMachineAndRollEffects(t *testing.T) {
	def := defForEffects()
	def.Machines = map[string]Machine{
		"quest": {Initial: "a", States: []string{"a", "b"}},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	ctx := newEvalCtx(nil, &RNG{state: 5})

	if err := applyEffect(def, st, ctx, Effect{Op: "set_machine_state", Machine: "quest", State: "b"}); err != nil {
		t.Fatal(err)
	}
	if st.Machines["quest"] != "b" {
		t.Fatalf("set_machine_state failed: %v", st.Machines["quest"])
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "set_machine_state", Machine: "quest", State: "z"}); err == nil {
		t.Fatal("expected invalid state error")
	}

	if err := applyEffect(def, st, ctx, Effect{Op: "roll", Dice: "1d6", Store: "r1"}); err != nil {
		t.Fatal(err)
	}
	if ctx.rolls["r1"] < 1 || ctx.rolls["r1"] > 6 {
		t.Fatalf("roll out of range: %v", ctx.rolls["r1"])
	}
	if len(ctx.record) != 1 {
		t.Fatalf("roll not recorded: %v", ctx.record)
	}
}

func TestMarkBeatEffect(t *testing.T) {
	def := defForEffects()
	def.Beats = map[string]Beat{"b1": {Text: "hi"}}
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})

	if err := applyEffect(def, st, ctx, Effect{Op: "mark_beat", Beat: "b1"}); err != nil {
		t.Fatal(err)
	}
	if len(st.DeliveredBeats) != 1 || st.DeliveredBeats[0] != "b1" {
		t.Fatalf("beat not recorded: %v", st.DeliveredBeats)
	}
	// Idempotent: marking again does not duplicate.
	if err := applyEffect(def, st, ctx, Effect{Op: "mark_beat", Beat: "b1"}); err != nil {
		t.Fatal(err)
	}
	if len(st.DeliveredBeats) != 1 {
		t.Fatalf("duplicate delivery: %v", st.DeliveredBeats)
	}
	// Unknown beat errors.
	if err := applyEffect(def, st, ctx, Effect{Op: "mark_beat", Beat: "nope"}); err == nil {
		t.Fatal("expected unknown-beat error")
	}
}

func TestSetAttachedStateEntityAndThisWrite(t *testing.T) {
	def := defForEffects() // entityType character with bounded health
	def.Machines = map[string]Machine{
		"mood": {Attach: &AttachSpec{To: "entityType:character"}, Initial: "calm", States: []string{"calm", "angry"}},
	}
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})
	if err := applyEffect(def, st, ctx, Effect{Op: "create_entity", EntityType: "character", ID: "npc"}); err != nil {
		t.Fatal(err)
	}
	ctx.host = &hostRef{kind: "entity", id: "npc", ent: st.Entities["npc"]}
	if err := applyEffect(def, st, ctx, Effect{Op: "dec", Target: "this.health", Value: float64(30)}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["npc"].Attrs["health"] != float64(70) {
		t.Fatalf("this.health write failed: %v", st.Entities["npc"].Attrs["health"])
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "set_attached_state", Machine: "mood", Entity: "npc", State: "angry"}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["npc"].Machines["mood"] != "angry" {
		t.Fatalf("entity set_attached_state failed: %v", st.Entities["npc"].Machines)
	}
}

func TestMarkBeatRepeatableNotRecorded(t *testing.T) {
	def := defForEffects()
	fls := false
	def.Beats = map[string]Beat{"loop": {Text: "again", Once: &fls}}
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, &RNG{state: 1})
	if err := applyEffect(def, st, ctx, Effect{Op: "mark_beat", Beat: "loop"}); err != nil {
		t.Fatal(err)
	}
	if len(st.DeliveredBeats) != 0 {
		t.Fatalf("repeatable beat must not be recorded as delivered: %v", st.DeliveredBeats)
	}
}

// defWithSetAttr creates a definition with an entity-type that has a set
// attribute ("clues": set of strings) and a ref-set attribute ("party": set of
// refs to "character"). An entity "aria" and "player" are pre-populated.
func defWithSetAttr() (*Definition, *State) {
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"clues": {Type: "set", Elem: "string"},
				"party": {Type: "set", Elem: "ref", RefType: "character"},
			}},
		},
	}
	st, _ := NewInstance(def, "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{
		"clues": []any{},
		"party": []any{},
	}, Inventory: map[string]int{}}
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{
		"clues": []any{},
		"party": []any{},
	}, Inventory: map[string]int{}}
	return def, st
}

func TestSetEffects(t *testing.T) {
	def, st := defWithSetAttr()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	// add_to: adds an element.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "entity.player.clues", Value: "alibi"}); err != nil {
		t.Fatal(err)
	}
	clues := st.Entities["player"].Attrs["clues"].([]any)
	if len(clues) != 1 || clues[0] != "alibi" {
		t.Fatalf("add_to failed: %v", clues)
	}

	// add_to: duplicate is a no-op.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "entity.player.clues", Value: "alibi"}); err != nil {
		t.Fatal(err)
	}
	if len(st.Entities["player"].Attrs["clues"].([]any)) != 1 {
		t.Fatal("add_to duplicate should be a no-op")
	}

	// add_to: add a second element.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "entity.player.clues", Value: "motive"}); err != nil {
		t.Fatal(err)
	}
	if len(st.Entities["player"].Attrs["clues"].([]any)) != 2 {
		t.Fatal("add_to second element failed")
	}

	// remove_from: removes the element.
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_from", Target: "entity.player.clues", Value: "alibi"}); err != nil {
		t.Fatal(err)
	}
	clues2 := st.Entities["player"].Attrs["clues"].([]any)
	if len(clues2) != 1 || clues2[0] != "motive" {
		t.Fatalf("remove_from failed: %v", clues2)
	}

	// remove_from: absent element is a no-op (no error).
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_from", Target: "entity.player.clues", Value: "ghost"}); err != nil {
		t.Fatal(err)
	}

	// clear: empties the set.
	if err := applyEffect(def, st, ctx, Effect{Op: "clear", Target: "entity.player.clues"}); err != nil {
		t.Fatal(err)
	}
	if len(st.Entities["player"].Attrs["clues"].([]any)) != 0 {
		t.Fatal("clear failed")
	}
}

func TestSetRefElemValidation(t *testing.T) {
	def, st := defWithSetAttr()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	// add_to ref-set with existing entity: ok.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "entity.player.party", Value: "aria"}); err != nil {
		t.Fatalf("ref add_to existing entity failed: %v", err)
	}

	// add_to ref-set with non-existent entity: error.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "entity.player.party", Value: "ghost"}); err == nil {
		t.Fatal("expected error for non-existent ref entity")
	}
}

func TestSetNonSetTargetRejected(t *testing.T) {
	def := defForEffects()
	st := stateForEffects()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	// "alarm" is a bool, not a set.
	if err := applyEffect(def, st, ctx, Effect{Op: "add_to", Target: "world.alarm", Value: "x"}); err == nil {
		t.Fatal("expected error for non-set target")
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "remove_from", Target: "world.alarm", Value: "x"}); err == nil {
		t.Fatal("expected error for non-set target")
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "clear", Target: "world.alarm"}); err == nil {
		t.Fatal("expected error for non-set target")
	}
}

func equipDef() *Definition {
	def := defForEffects() // entityType character
	et := def.EntityTypes["character"]
	et.Slots = map[string]SlotSpec{
		"torso": {Accepts: []string{"dress", "top"}},
		"head":  {Accepts: []string{"hat"}},
	}
	def.EntityTypes["character"] = et
	def.ItemTypes = map[string]ItemType{
		"silk_dress": {Category: "dress", Equippable: true, Attributes: map[string]any{"style": float64(8)}},
		"gold":       {}, // not equippable
	}
	return def
}

func TestEquipUnequip(t *testing.T) {
	def := equipDef()
	st, _ := NewInstance(def, "r", 1)
	st.Entities["p"] = &Entity{Type: "character", Attrs: map[string]any{}, Inventory: map[string]int{}}
	ctx := newEvalCtx(nil, &RNG{state: 1})

	if err := applyEffect(def, st, ctx, Effect{Op: "equip", Entity: "p", Slot: "torso", Item: "silk_dress"}); err != nil {
		t.Fatal(err)
	}
	if st.Entities["p"].Equipped["torso"] != "silk_dress" {
		t.Fatalf("equip failed: %v", st.Entities["p"].Equipped)
	}
	// Slot occupied -> error.
	if err := applyEffect(def, st, ctx, Effect{Op: "equip", Entity: "p", Slot: "torso", Item: "silk_dress"}); err == nil {
		t.Fatal("expected slot-occupied error")
	}
	// Wrong category for slot -> error.
	if err := applyEffect(def, st, ctx, Effect{Op: "equip", Entity: "p", Slot: "head", Item: "silk_dress"}); err == nil {
		t.Fatal("expected category-mismatch error")
	}
	// Not equippable -> error.
	if err := applyEffect(def, st, ctx, Effect{Op: "equip", Entity: "p", Slot: "torso", Item: "gold"}); err == nil {
		t.Fatal("expected not-equippable error")
	}
	// Unknown slot -> error.
	if err := applyEffect(def, st, ctx, Effect{Op: "equip", Entity: "p", Slot: "feet", Item: "silk_dress"}); err == nil {
		t.Fatal("expected unknown-slot error")
	}
	// Unequip frees the slot.
	if err := applyEffect(def, st, ctx, Effect{Op: "unequip", Entity: "p", Slot: "torso"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Entities["p"].Equipped["torso"]; ok {
		t.Fatalf("unequip left slot set: %v", st.Entities["p"].Equipped)
	}
}
