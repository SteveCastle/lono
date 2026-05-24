package engine

import "testing"

func TestValidateDefinitionOK(t *testing.T) {
	if errs := ValidateDefinition(fullDef()); len(errs) != 0 {
		t.Fatalf("expected valid def, got %v", errs)
	}
}

func TestValidateDefinitionCatchesProblems(t *testing.T) {
	def := fullDef()
	// Machine initial state not in states.
	m := def.Machines["arc"]
	m.Initial = "nope"
	def.Machines["arc"] = m
	// Relationship type referencing an undefined entity type.
	def.RelationshipTypes["trust"] = RelType{From: "ghost", To: "character"}
	// Transition To referencing an undefined state.
	m2 := def.Machines["arc"]
	m2.Transitions[0].To = "void"
	def.Machines["arc"] = m2

	errs := ValidateDefinition(def)
	if len(errs) < 3 {
		t.Fatalf("expected >=3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateDerived(t *testing.T) {
	good := &Definition{
		ID: "g", Version: 1,
		RelationshipTypes: map[string]RelType{"romance": {From: "character", To: "character"}},
		EntityTypes:       map[string]EntityType{"character": {}},
		Derived: map[string]DerivedSpec{
			"admirers": {Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "count"},
		},
	}
	if errs := ValidateDefinition(good); len(errs) != 0 {
		t.Fatalf("expected valid, got %v", errs)
	}

	bad := &Definition{
		ID: "g", Version: 1,
		Derived: map[string]DerivedSpec{
			"x": {Over: "sideways", Reduce: "count"},                                       // bad over
			"y": {Over: "relationships", Where: WhereSpec{Type: "ghost"}, Reduce: "count"}, // unknown rel type
			"z": {Over: "relationships", Reduce: "frobnicate"},                             // bad reduce
		},
	}
	if errs := ValidateDefinition(bad); len(errs) < 3 {
		t.Fatalf("expected >=3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateEnumDefaultMember(t *testing.T) {
	def := fullDef()
	def.World["weather"] = VarSpec{Type: "enum", Values: []string{"sun", "rain"}, Default: "snow"}
	errs := ValidateDefinition(def)
	if len(errs) == 0 {
		t.Fatal("expected error for enum default not in values")
	}
}

func TestValidateAttachedMachine(t *testing.T) {
	good := &Definition{
		ID: "g", Version: 1,
		RelationshipTypes: map[string]RelType{"trust": {From: "character", To: "character"}},
		EntityTypes:       map[string]EntityType{"character": {}},
		Machines: map[string]Machine{
			"bond": {Attach: &AttachSpec{To: "relationshipType:trust"}, Initial: "a", States: []string{"a"}},
		},
	}
	if errs := ValidateDefinition(good); len(errs) != 0 {
		t.Fatalf("expected valid, got %v", errs)
	}
	bad := &Definition{
		ID: "g", Version: 1,
		Machines: map[string]Machine{
			"x": {Attach: &AttachSpec{To: "relationshipType:ghost"}, Initial: "a", States: []string{"a"}},
			"y": {Attach: &AttachSpec{To: "garbage"}, Initial: "a", States: []string{"a"}},
		},
	}
	if errs := ValidateDefinition(bad); len(errs) < 2 {
		t.Fatalf("expected >=2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateNarrative(t *testing.T) {
	good := &Definition{
		ID: "g", Version: 1,
		Machines: map[string]Machine{
			"arc": {Initial: "intro", States: []string{"intro", "end"},
				StateMeta: map[string]StateMeta{"end": {Terminal: true}}},
		},
		Beats: map[string]Beat{
			"b": {Text: "hi", MachineState: &MachineStateRef{Machine: "arc", State: "intro"}},
		},
	}
	if errs := ValidateDefinition(good); len(errs) != 0 {
		t.Fatalf("expected valid, got %v", errs)
	}
	bad := &Definition{
		ID: "g", Version: 1,
		Machines: map[string]Machine{
			"arc": {Initial: "intro", States: []string{"intro"},
				StateMeta: map[string]StateMeta{"ghost": {Terminal: true}}}, // unknown state
		},
		Beats: map[string]Beat{
			"nomachine": {Text: "x", MachineState: &MachineStateRef{Machine: "nope", State: "intro"}}, // unknown machine
			"nostate":   {Text: "y", MachineState: &MachineStateRef{Machine: "arc", State: "ghost"}},  // unknown state
			"notext":    {MachineState: &MachineStateRef{Machine: "arc", State: "intro"}},             // empty text
		},
	}
	if errs := ValidateDefinition(bad); len(errs) < 4 {
		t.Fatalf("expected >=4 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateDerivedAttrExistence(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		RelationshipTypes: map[string]RelType{
			"romance": {From: "character", To: "character",
				Attributes: map[string]VarSpec{"attraction": {Type: "int"}}},
		},
		EntityTypes: map[string]EntityType{"character": {}},
		Derived: map[string]DerivedSpec{
			"ok":      {Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "max:attraction"},
			"badattr": {Over: "relationships", Where: WhereSpec{Type: "romance", To: "player"}, Reduce: "max:bogus"},
			"badpred": {Over: "relationships", Where: WhereSpec{Type: "romance",
				Attrs: []AttrPred{{Attr: "ghost", Op: "gte", Value: float64(1)}}}, Reduce: "count"},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) != 2 {
		t.Fatalf("expected exactly 2 attr-existence errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateCastGood(t *testing.T) {
	// A valid cast should produce 0 errors.
	zero, hundred := 0.0, 100.0
	ms := 1
	def := &Definition{
		ID: "g", Version: 1,
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
			"trust": {From: "character", To: "character",
				Attributes: map[string]VarSpec{
					"value": {Type: "int", Default: float64(0), Min: &zero, Max: &hundred},
				},
			},
		},
		Entities: map[string]EntityInit{
			"aria": {
				Type:      "character",
				Attrs:     map[string]any{"charm": float64(50)},
				Inventory: map[string]int{"sword": 1},
				Equipped:  map[string]string{"hand": "sword"},
			},
			"player": {Type: "character"},
		},
		Relationships: []RelInit{
			{Type: "trust", From: "aria", To: "player", Attrs: map[string]any{"value": float64(30)}},
		},
	}
	if errs := ValidateDefinition(def); len(errs) != 0 {
		t.Fatalf("expected valid cast, got %d errors: %v", len(errs), errs)
	}
}

func TestValidateCastErrors(t *testing.T) {
	// Three invalid conditions: unknown entity type, rel referencing non-cast
	// entity, and equip of a non-equippable item.
	ms := 1
	def := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {
				Slots: map[string]SlotSpec{
					"hand": {Accepts: []string{"weapon"}},
				},
			},
		},
		ItemTypes: map[string]ItemType{
			"coin":  {Category: "currency", Equippable: false, MaxStack: &ms},
			"sword": {Category: "weapon", Equippable: true, MaxStack: &ms},
		},
		RelationshipTypes: map[string]RelType{
			"trust": {From: "character", To: "character"},
		},
		Entities: map[string]EntityInit{
			// Error 1: unknown entity type "ghost_type".
			"ghost_entity": {Type: "ghost_type"},
			// Error 2 (setup for rel): valid entity so rel can reference it.
			"player": {Type: "character"},
			// Error 3: equip of a non-equippable item (coin in hand slot).
			"aria": {
				Type:     "character",
				Equipped: map[string]string{"hand": "coin"},
			},
		},
		Relationships: []RelInit{
			// Error 2: "npc" is not in def.Entities.
			{Type: "trust", From: "player", To: "npc"},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) < 3 {
		t.Fatalf("expected >=3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateBeatAttachedMachine(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		RelationshipTypes: map[string]RelType{"romance": {From: "character", To: "character"}},
		EntityTypes:       map[string]EntityType{"character": {}},
		Machines: map[string]Machine{
			"stage": {Attach: &AttachSpec{To: "relationshipType:romance"}, Initial: "a", States: []string{"a"}},
		},
		Beats: map[string]Beat{
			"bad": {Text: "x", MachineState: &MachineStateRef{Machine: "stage", State: "a"}},
		},
	}
	found := false
	for _, e := range ValidateDefinition(def) {
		if e.Path == "beats.bad.machineState.machine" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected beat bound to attached machine to be rejected")
	}
}

// B4: compute and if validation

func TestValidateComputeAndIfEffects(t *testing.T) {
	// Bad def: compute with unknown fn and if with nil when -> >=2 errors.
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"score": {Type: "int", Default: float64(0)},
		},
		Machines: map[string]Machine{
			"arc": {
				Initial: "a", States: []string{"a", "b"},
				Transitions: []Transition{
					{
						ID: "go", From: StateSet{"a"}, To: "b",
						Effects: []Effect{
							// compute with bad fn ("" is not valid)
							{Op: "compute", Target: "world.score", Fn: "", A: float64(1), B: float64(2)},
							// if with nil when
							{Op: "if", Then: []Effect{{Op: "set", Target: "world.score", Value: float64(1)}}},
						},
					},
				},
			},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) < 2 {
		t.Fatalf("expected >=2 validation errors for bad compute fn and if without when, got %d: %v", len(errs), errs)
	}
}

func TestValidateComputeAndIfEffectsValid(t *testing.T) {
	// Valid def with compute and if effects -> 0 errors from these ops.
	lo, hi := 0.0, 100.0
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"score": {Type: "int", Default: float64(0), Min: &lo, Max: &hi},
			"day":   {Type: "int", Default: float64(1)},
		},
		Machines: map[string]Machine{
			"arc": {
				Initial: "a", States: []string{"a", "b"},
				Transitions: []Transition{
					{
						ID: "go", From: StateSet{"a"}, To: "b",
						Effects: []Effect{
							{Op: "compute", Target: "world.score", Fn: "add", A: float64(1), B: float64(2)},
							{
								Op:   "if",
								When: &Guard{Target: "world.day", Op: "gte", Value: float64(1)},
								Then: []Effect{{Op: "set", Target: "world.score", Value: float64(5)}},
							},
						},
					},
				},
			},
		},
	}
	errs := ValidateDefinition(def)
	// Should be zero errors
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for valid compute/if, got %d: %v", len(errs), errs)
	}
}

func TestValidateComputeSetupEffects(t *testing.T) {
	// compute in setup with bad fn -> error
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"score": {Type: "int", Default: float64(0)},
		},
		Setup: []Effect{
			{Op: "compute", Target: "world.score", Fn: "badop", A: float64(1), B: float64(2)},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) == 0 {
		t.Fatal("expected error for bad compute fn in setup")
	}
}

func TestValidateIfNestedComputeError(t *testing.T) {
	// if effect with a compute in then that has bad fn -> error propagates
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"score": {Type: "int", Default: float64(0)},
			"day":   {Type: "int", Default: float64(1)},
		},
		Setup: []Effect{
			{
				Op:   "if",
				When: &Guard{Target: "world.day", Op: "gte", Value: float64(1)},
				Then: []Effect{
					{Op: "compute", Target: "world.score", Fn: "invalid", A: float64(1), B: float64(2)},
				},
			},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) == 0 {
		t.Fatal("expected error for bad fn inside if.then")
	}
}

func TestValidateSetSpec(t *testing.T) {
	// Valid set specs: elem string and ref.
	good := &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"tags":   {Type: "set", Elem: "string"},
				"allies": {Type: "set", Elem: "ref", RefType: "character"},
			}},
		},
	}
	if errs := ValidateDefinition(good); len(errs) != 0 {
		t.Fatalf("expected valid set spec, got %v", errs)
	}

	// Invalid: set with unknown elem.
	bad := &Definition{
		ID: "b", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"things": {Type: "set", Elem: "number"},
			}},
		},
	}
	errs := ValidateDefinition(bad)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown set elem")
	}

	// Invalid: set with empty elem.
	bad2 := &Definition{
		ID: "b2", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"things": {Type: "set"},
			}},
		},
	}
	errs2 := ValidateDefinition(bad2)
	if len(errs2) == 0 {
		t.Fatal("expected error for set with empty elem")
	}
}
