package engine

import "testing"

func fullDef() *Definition {
	def := defWithTrust() // character.health [0,100], trust.value [-100,100]
	def.World = map[string]VarSpec{"alarm": {Type: "bool", Default: false}}
	def.Machines = map[string]Machine{
		"arc": {Initial: "intro", States: []string{"intro", "rising"},
			Transitions: []Transition{{
				ID: "begin", From: StateSet{"intro"}, To: "rising",
				Params: map[string]VarSpec{"force": {Type: "int", Min: f(0)}},
				Guard:  &Guard{Target: "param.force", Op: "gte", Value: float64(1)},
				Effects: []Effect{
					{Op: "set", Target: "world.alarm", Value: true},
					{Op: "dec", Target: "entity.player.health", Value: map[string]any{"$roll": "dmg"}},
				},
			}}},
	}
	// roll happens before the dec via an earlier effect:
	def.Machines["arc"].Transitions[0].Effects = append(
		[]Effect{{Op: "roll", Dice: "1d4", Store: "dmg"}},
		def.Machines["arc"].Transitions[0].Effects...)
	def.Setup = []Effect{{Op: "create_entity", EntityType: "character", ID: "player"}}
	return def
}

func f(v float64) *float64 { return &v }

func TestStartInstanceAppliesSetup(t *testing.T) {
	st, err := StartInstance(fullDef(), "run1", 42)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entities["player"] == nil {
		t.Fatal("setup did not create player")
	}
	if st.Machines["arc"] != "intro" {
		t.Fatal("machine not at initial")
	}
}

func TestPerformActionAtomicAndDeterministic(t *testing.T) {
	def := fullDef()
	st, _ := StartInstance(def, "run1", 42)

	// Missing required param -> error, no mutation.
	if _, _, err := PerformAction(def, st, "arc", "begin", map[string]any{}); err == nil {
		t.Fatal("expected missing-param error")
	}
	if st.Machines["arc"] != "intro" || st.World["alarm"] == true {
		t.Fatal("failed action must not mutate original state")
	}

	// Guard fails (force=0 < 1).
	if _, _, err := PerformAction(def, st, "arc", "begin", map[string]any{"force": float64(0)}); err == nil {
		t.Fatal("expected guard failure")
	}

	// Success.
	ns, res, err := PerformAction(def, st, "arc", "begin", map[string]any{"force": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Machines["arc"] != "rising" || ns.World["alarm"] != true {
		t.Fatalf("state not advanced: %+v", ns.Machines)
	}
	h := ns.Entities["player"].Attrs["health"].(float64)
	if h < 96 || h > 99 { // 100 - 1d4
		t.Fatalf("health after 1d4 dmg out of range: %v", h)
	}
	if len(res.Rolls) != 1 {
		t.Fatalf("expected one recorded roll, got %v", res.Rolls)
	}
	if len(ns.History) != 1 || ns.History[0].Kind != "action" {
		t.Fatalf("history not recorded: %+v", ns.History)
	}

	// Determinism: same seed + same action => same roll.
	st2, _ := StartInstance(def, "run2", 42)
	ns2, _, _ := PerformAction(def, st2, "arc", "begin", map[string]any{"force": float64(2)})
	if ns.Entities["player"].Attrs["health"] != ns2.Entities["player"].Attrs["health"] {
		t.Fatal("same seed must yield same roll outcome")
	}
}

func TestGuardUsesDerived(t *testing.T) {
	def, st := socialState()
	def.Machines = map[string]Machine{
		"arc": {Initial: "a", States: []string{"a", "b"},
			Transitions: []Transition{{
				ID: "advance", From: StateSet{"a"}, To: "b",
				Guard: &Guard{Target: "derived.any_admirer", Op: "eq", Value: true},
			}}},
	}
	st.Machines["arc"] = "a"
	ns, _, err := PerformAction(def, st, "arc", "advance", nil)
	if err != nil {
		t.Fatalf("guard referencing derived should pass (aria adores player): %v", err)
	}
	if ns.Machines["arc"] != "b" {
		t.Fatal("machine did not advance")
	}
}

func TestApplyOpsAtomic(t *testing.T) {
	def := fullDef()
	st, _ := StartInstance(def, "run1", 42)

	// One good op then one bad op: nothing should persist.
	ops := []Effect{
		{Op: "set", Target: "world.alarm", Value: true},
		{Op: "set", Target: "entity.player.health", Value: float64(999)}, // exceeds max 100
	}
	if _, _, err := ApplyOps(def, st, ops); err == nil {
		t.Fatal("expected failure on out-of-bounds op")
	}
	if st.World["alarm"] == true {
		t.Fatal("partial mutation leaked from failed ApplyOps")
	}

	// All good.
	ns, res, err := ApplyOps(def, st, []Effect{{Op: "set", Target: "world.alarm", Value: true}})
	if err != nil {
		t.Fatal(err)
	}
	if ns.World["alarm"] != true {
		t.Fatal("apply did not take effect")
	}
	if len(ns.History) != 1 || ns.History[0].Kind != "apply" {
		t.Fatalf("apply history wrong: %+v", ns.History)
	}
	_ = res
}

func TestStartInstanceSeedsCast(t *testing.T) {
	// Build a def that uses first-class Entities + Relationships (no Setup).
	zero, hundred := 0.0, 100.0
	ms := 10 // maxStack for the sword
	def := &Definition{
		ID: "cast-test", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {
				Attributes: map[string]VarSpec{
					"charm": {Type: "int", Default: float64(0), Min: &zero, Max: &hundred},
				},
				Slots: map[string]SlotSpec{
					"hand": {Accepts: []string{"weapon"}},
				},
			},
		},
		ItemTypes: map[string]ItemType{
			"sword": {Category: "weapon", Equippable: true, MaxStack: &ms},
		},
		RelationshipTypes: map[string]RelType{
			"trust": {From: "character", To: "character", Directed: true,
				Attributes: map[string]VarSpec{
					"value": {Type: "int", Default: float64(0), Min: &zero, Max: &hundred},
				},
			},
		},
		Entities: map[string]EntityInit{
			"aria": {
				Type:      "character",
				Attrs:     map[string]any{"charm": float64(8)},
				Inventory: map[string]int{"sword": 2},
				Equipped:  map[string]string{"hand": "sword"},
			},
			"player": {
				Type: "character",
			},
		},
		Relationships: []RelInit{
			{Type: "trust", From: "aria", To: "player", Attrs: map[string]any{"value": float64(40)}},
		},
	}

	st, err := StartInstance(def, "r1", 42)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}

	// Entity attrs.
	aria, ok := st.Entities["aria"]
	if !ok {
		t.Fatal("aria not seeded")
	}
	if aria.Attrs["charm"] != float64(8) {
		t.Fatalf("aria.charm: %v", aria.Attrs["charm"])
	}

	// Inventory.
	if aria.Inventory["sword"] != 2 {
		t.Fatalf("aria.inventory.sword: %v", aria.Inventory["sword"])
	}

	// Equipped.
	if aria.Equipped["hand"] != "sword" {
		t.Fatalf("aria.equipped.hand: %v", aria.Equipped["hand"])
	}

	// Player also seeded.
	if _, ok := st.Entities["player"]; !ok {
		t.Fatal("player not seeded")
	}

	// Relationship.
	r := findRelationship(st, "trust", "aria", "player")
	if r == nil {
		t.Fatal("trust relationship not seeded")
	}
	if r.Attrs["value"] != float64(40) {
		t.Fatalf("trust.value: %v", r.Attrs["value"])
	}
}

func TestStartInstanceCastBeforeSetup(t *testing.T) {
	// Setup should be able to reference entities created by the cast.
	zero, hundred := 0.0, 100.0
	def := &Definition{
		ID: "order-test", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"charm": {Type: "int", Default: float64(0), Min: &zero, Max: &hundred},
			}},
		},
		Entities: map[string]EntityInit{
			"aria": {Type: "character"},
		},
		// Setup overrides aria's charm after the cast creates her.
		Setup: []Effect{
			{Op: "set", Target: "entity.aria.charm", Value: float64(99)},
		},
	}

	st, err := StartInstance(def, "r1", 1)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if st.Entities["aria"].Attrs["charm"] != float64(99) {
		t.Fatalf("setup override not applied: %v", st.Entities["aria"].Attrs["charm"])
	}
}

func datingDef() *Definition {
	def := defWithTrust() // character.health, trust.value [-100,100]
	def.Machines = map[string]Machine{
		"romance": {Attach: &AttachSpec{To: "relationshipType:trust"},
			Initial: "friends", States: []string{"friends", "dating"},
			Transitions: []Transition{{
				ID: "start_dating", From: StateSet{"friends"}, To: "dating",
				Guard:   &Guard{Target: "this.value", Op: "gte", Value: float64(50)},
				Effects: []Effect{{Op: "inc", Target: "this.value", Value: float64(5)}},
			}}},
	}
	return def
}

func TestPerformHostActionRelationship(t *testing.T) {
	def := datingDef()
	st, _ := NewInstance(def, "r", 1)
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{{Type: "trust", From: "aria", To: "player",
		Attrs: map[string]any{"value": float64(60)}, Machines: map[string]string{"romance": "friends"}}}

	host := &HostRef{Kind: "relationship", From: "aria", To: "player"}
	ns, _, err := PerformHostAction(def, st, "romance", "start_dating", nil, host)
	if err != nil {
		t.Fatalf("start_dating: %v", err)
	}
	r := findRelationship(ns, "trust", "aria", "player")
	if r.Machines["romance"] != "dating" {
		t.Fatalf("romance did not advance: %v", r.Machines)
	}
	if r.Attrs["value"] != float64(65) {
		t.Fatalf("this.value effect not applied: %v", r.Attrs["value"])
	}
	// Input state untouched (atomic clone).
	if st.Relationships[0].Machines["romance"] != "friends" {
		t.Fatal("input state mutated")
	}
}

func TestPerformHostActionGuardBlocks(t *testing.T) {
	def := datingDef()
	st, _ := NewInstance(def, "r", 1)
	st.Entities["aria"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"health": float64(100)}, Inventory: map[string]int{}}
	st.Relationships = []*Relationship{{Type: "trust", From: "aria", To: "player",
		Attrs: map[string]any{"value": float64(10)}, Machines: map[string]string{"romance": "friends"}}}
	host := &HostRef{Kind: "relationship", From: "aria", To: "player"}
	if _, _, err := PerformHostAction(def, st, "romance", "start_dating", nil, host); err == nil {
		t.Fatal("guard should block (value 10 < 50)")
	}
}

func TestAttachedMachineNotGlobalAndHostless(t *testing.T) {
	def := datingDef() // attached "romance" machine on relationshipType:trust
	st, _ := NewInstance(def, "r", 1)
	if _, ok := st.Machines["romance"]; ok {
		t.Fatal("attached machine must not be initialized into global st.Machines")
	}
	if _, _, err := PerformAction(def, st, "romance", "start_dating", nil); err == nil {
		t.Fatal("attached machine must not be advanceable via host-less PerformAction")
	}
}
