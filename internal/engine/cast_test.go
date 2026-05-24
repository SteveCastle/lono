package engine

import "testing"

// castDef returns a minimal Definition suitable for cast-helper tests.
func castDef() *Definition {
	return &Definition{
		ID: "g", Version: 1,
		EntityTypes: map[string]EntityType{
			"character": {},
		},
		ItemTypes: map[string]ItemType{
			"sword": {Category: "weapon", Equippable: true},
		},
		RelationshipTypes: map[string]RelType{
			"trust": {From: "character", To: "character"},
		},
	}
}

func TestAddCharacter(t *testing.T) {
	def := castDef()

	// Errors on empty id/type.
	if err := AddCharacter(def, "", "character", nil, ""); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := AddCharacter(def, "aria", "", nil, ""); err == nil {
		t.Fatal("expected error for empty type")
	}

	// Add aria.
	attrs := map[string]any{"charm": float64(7)}
	if err := AddCharacter(def, "aria", "character", attrs, ""); err != nil {
		t.Fatalf("AddCharacter: %v", err)
	}
	e, ok := def.Entities["aria"]
	if !ok {
		t.Fatal("aria not added")
	}
	if e.Type != "character" {
		t.Fatalf("aria.Type: %v", e.Type)
	}
	if e.Attrs["charm"] != float64(7) {
		t.Fatalf("aria.Attrs.charm: %v", e.Attrs["charm"])
	}

	// Replace with updated attrs.
	if err := AddCharacter(def, "aria", "character", map[string]any{"charm": float64(9)}, ""); err != nil {
		t.Fatalf("AddCharacter replace: %v", err)
	}
	if def.Entities["aria"].Attrs["charm"] != float64(9) {
		t.Fatalf("replace did not update: %v", def.Entities["aria"].Attrs["charm"])
	}

	// Nil Entities map is initialized automatically.
	def2 := castDef()
	def2.Entities = nil
	if err := AddCharacter(def2, "player", "character", nil, ""); err != nil {
		t.Fatalf("AddCharacter nil map: %v", err)
	}
	if def2.Entities["player"].Type != "character" {
		t.Fatalf("nil map not initialized: %+v", def2.Entities)
	}
}

func TestRemoveCharacter(t *testing.T) {
	def := castDef()
	_ = AddCharacter(def, "aria", "character", nil, "")
	_ = AddCharacter(def, "player", "character", nil, "")
	_ = AddRelationship(def, "trust", "aria", "player", nil)
	_ = AddRelationship(def, "trust", "player", "aria", nil)

	// Error if absent.
	if err := RemoveCharacter(def, "ghost"); err == nil {
		t.Fatal("expected error for missing character")
	}

	// Remove aria — cascades both relationships.
	if err := RemoveCharacter(def, "aria"); err != nil {
		t.Fatalf("RemoveCharacter: %v", err)
	}
	if _, ok := def.Entities["aria"]; ok {
		t.Fatal("aria still present after remove")
	}
	if len(def.Relationships) != 0 {
		t.Fatalf("expected 0 relationships after cascade, got %d: %v", len(def.Relationships), def.Relationships)
	}
	// player is still there.
	if _, ok := def.Entities["player"]; !ok {
		t.Fatal("player was incorrectly removed")
	}
}

func TestAddRelationship(t *testing.T) {
	def := castDef()
	_ = AddCharacter(def, "aria", "character", nil, "")
	_ = AddCharacter(def, "player", "character", nil, "")

	if err := AddRelationship(def, "trust", "aria", "player", map[string]any{"value": float64(10)}); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}
	if len(def.Relationships) != 1 {
		t.Fatalf("expected 1, got %d", len(def.Relationships))
	}
	if def.Relationships[0].Attrs["value"] != float64(10) {
		t.Fatalf("attrs: %v", def.Relationships[0].Attrs)
	}

	// Replace same tuple.
	if err := AddRelationship(def, "trust", "aria", "player", map[string]any{"value": float64(99)}); err != nil {
		t.Fatalf("AddRelationship replace: %v", err)
	}
	if len(def.Relationships) != 1 {
		t.Fatalf("expected still 1, got %d", len(def.Relationships))
	}
	if def.Relationships[0].Attrs["value"] != float64(99) {
		t.Fatalf("replace did not update attrs: %v", def.Relationships[0].Attrs)
	}

	// Validation errors for empty fields.
	if err := AddRelationship(def, "", "aria", "player", nil); err == nil {
		t.Fatal("expected error for empty type")
	}
	if err := AddRelationship(def, "trust", "", "player", nil); err == nil {
		t.Fatal("expected error for empty from")
	}
	if err := AddRelationship(def, "trust", "aria", "", nil); err == nil {
		t.Fatal("expected error for empty to")
	}
}

func TestRemoveRelationship(t *testing.T) {
	def := castDef()
	_ = AddCharacter(def, "aria", "character", nil, "")
	_ = AddCharacter(def, "player", "character", nil, "")
	_ = AddRelationship(def, "trust", "aria", "player", nil)

	// Error if absent.
	if err := RemoveRelationship(def, "trust", "player", "aria"); err == nil {
		t.Fatal("expected error for missing relationship")
	}

	if err := RemoveRelationship(def, "trust", "aria", "player"); err != nil {
		t.Fatalf("RemoveRelationship: %v", err)
	}
	if len(def.Relationships) != 0 {
		t.Fatalf("expected 0, got %d", len(def.Relationships))
	}
}

func TestGiveItem(t *testing.T) {
	def := castDef()
	_ = AddCharacter(def, "aria", "character", nil, "")

	// Error if char absent.
	if err := GiveItem(def, "ghost", "sword", 1, ""); err == nil {
		t.Fatal("expected error for missing character")
	}

	// Add inventory (count > 0).
	if err := GiveItem(def, "aria", "sword", 3, ""); err != nil {
		t.Fatalf("GiveItem inventory: %v", err)
	}
	if def.Entities["aria"].Inventory["sword"] != 3 {
		t.Fatalf("inventory: %v", def.Entities["aria"].Inventory["sword"])
	}

	// Equip (slot non-empty).
	if err := GiveItem(def, "aria", "sword", 0, "hand"); err != nil {
		t.Fatalf("GiveItem equip: %v", err)
	}
	if def.Entities["aria"].Equipped["hand"] != "sword" {
		t.Fatalf("equipped: %v", def.Entities["aria"].Equipped["hand"])
	}

	// Both at once.
	_ = AddCharacter(def, "player", "character", nil, "")
	if err := GiveItem(def, "player", "sword", 2, "hand"); err != nil {
		t.Fatalf("GiveItem both: %v", err)
	}
	if def.Entities["player"].Inventory["sword"] != 2 {
		t.Fatalf("player inventory: %v", def.Entities["player"].Inventory["sword"])
	}
	if def.Entities["player"].Equipped["hand"] != "sword" {
		t.Fatalf("player equipped: %v", def.Entities["player"].Equipped["hand"])
	}

	// count == 0 and no slot: no-op (no error, no change to inventory key).
	prev := def.Entities["aria"].Inventory["sword"]
	if err := GiveItem(def, "aria", "sword", 0, ""); err != nil {
		t.Fatalf("GiveItem no-op: %v", err)
	}
	if def.Entities["aria"].Inventory["sword"] != prev {
		t.Fatalf("no-op changed inventory: %v", def.Entities["aria"].Inventory["sword"])
	}

	// Nil maps are initialized.
	def2 := castDef()
	def2.Entities = map[string]EntityInit{
		"bare": {Type: "character"},
	}
	if err := GiveItem(def2, "bare", "sword", 1, "hand"); err != nil {
		t.Fatalf("GiveItem nil maps: %v", err)
	}
	if def2.Entities["bare"].Inventory["sword"] != 1 {
		t.Fatalf("nil inventory not initialized: %v", def2.Entities["bare"].Inventory)
	}
	if def2.Entities["bare"].Equipped["hand"] != "sword" {
		t.Fatalf("nil equipped not initialized: %v", def2.Entities["bare"].Equipped)
	}
}

// TestAddCharacterDescription verifies that AddCharacter stores the description
// in EntityInit and that seedCast seeds it onto the runtime Entity.
func TestAddCharacterDescription(t *testing.T) {
	def := castDef()
	// Add entity type with a location attr so we can start an instance.
	def.EntityTypes["character"] = EntityType{Attributes: map[string]VarSpec{}}

	const desc = "A worn leather armchair, crammed with dog-eared books."
	if err := AddCharacter(def, "chair", "character", nil, desc); err != nil {
		t.Fatalf("AddCharacter with description: %v", err)
	}
	if def.Entities["chair"].Description != desc {
		t.Fatalf("EntityInit.Description: got %q, want %q", def.Entities["chair"].Description, desc)
	}

	// Start an instance: seedCast should copy Description onto Entity.
	st, err := StartInstance(def, "r", 1)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	ent, ok := st.Entities["chair"]
	if !ok {
		t.Fatal("chair entity not in state")
	}
	if ent.Description != desc {
		t.Fatalf("Entity.Description after start: got %q, want %q", ent.Description, desc)
	}
}

// TestDescriptionEmptyNotSeeded verifies that an empty description is not set.
func TestDescriptionEmptyNotSeeded(t *testing.T) {
	def := castDef()
	def.EntityTypes["character"] = EntityType{Attributes: map[string]VarSpec{}}
	if err := AddCharacter(def, "player", "character", nil, ""); err != nil {
		t.Fatalf("AddCharacter: %v", err)
	}
	st, err := StartInstance(def, "r", 1)
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if st.Entities["player"].Description != "" {
		t.Fatalf("empty description should not be set: got %q", st.Entities["player"].Description)
	}
}
