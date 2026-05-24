package cli

import (
	"encoding/json"
	"testing"
)

// seedConstructionGame creates a small game with an entity-type "character"
// (attributes: name string), an item-type "locket", and a rel-type "romance".
func seedConstructionGame(t *testing.T, dir string) {
	t.Helper()
	runCLI(t, dir, "game", "create", "g")

	steps := [][]string{
		{"define", "entity-type", "set", "g", "character", "--spec", `{"attributes":{"name":{"type":"string","default":""}}}`},
		{"define", "item-type", "set", "g", "locket", "--spec", `{}`},
		{"define", "rel-type", "set", "g", "romance", "--spec", `{"from":"character","to":"character","directed":false}`},
	}
	for _, s := range steps {
		env, _ := runCLI(t, dir, s...)
		if !env.OK {
			t.Fatalf("seedConstructionGame %v failed: %+v", s, env.Error)
		}
	}
}

// TestConstructionCastAddListGiveRm exercises the full B5 surface.
func TestConstructionCastAddListGiveRm(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	// Add character aria.
	env, _ := runCLI(t, dir, "game", "add", "character", "g", "aria",
		"--type", "character", "--attrs", `{"name":"Aria"}`)
	if !env.OK {
		t.Fatalf("add character aria failed: %+v", env.Error)
	}

	// Add character player (no attrs).
	env, _ = runCLI(t, dir, "game", "add", "character", "g", "player",
		"--type", "character")
	if !env.OK {
		t.Fatalf("add character player failed: %+v", env.Error)
	}

	// Add relationship romance aria->player.
	env, _ = runCLI(t, dir, "game", "add", "relationship", "g", "romance", "aria", "player",
		"--attrs", `{}`)
	if !env.OK {
		t.Fatalf("add relationship failed: %+v", env.Error)
	}

	// list characters shows both.
	env, _ = runCLI(t, dir, "game", "list", "characters", "g")
	if !env.OK {
		t.Fatalf("list characters failed: %+v", env.Error)
	}
	listData := env.Data.(map[string]any)
	entities := listData["characters"].(map[string]any)
	if _, ok := entities["aria"]; !ok {
		t.Fatal("list characters: aria not found")
	}
	if _, ok := entities["player"]; !ok {
		t.Fatal("list characters: player not found")
	}

	// list relationships shows the romance.
	env, _ = runCLI(t, dir, "game", "list", "relationships", "g")
	if !env.OK {
		t.Fatalf("list relationships failed: %+v", env.Error)
	}
	relData := env.Data.(map[string]any)
	rels := relData["relationships"].([]any)
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(rels))
	}

	// give aria a locket.
	env, _ = runCLI(t, dir, "game", "give", "g", "aria", "--item", "locket")
	if !env.OK {
		t.Fatalf("give locket failed: %+v", env.Error)
	}

	// Confirm locket appears in entities via game get.
	env, _ = runCLI(t, dir, "game", "get", "g", "entities/aria/inventory/locket")
	if !env.OK {
		t.Fatalf("game get entities/aria/inventory/locket failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	if val != float64(1) {
		t.Fatalf("expected locket count=1, got %v", val)
	}

	// rm character aria — should also remove the relationship referencing her.
	env, _ = runCLI(t, dir, "game", "rm", "character", "g", "aria")
	if !env.OK {
		t.Fatalf("rm character aria failed: %+v", env.Error)
	}

	// Confirm aria gone.
	env, _ = runCLI(t, dir, "game", "get", "g", "entities/aria")
	if env.OK {
		t.Fatal("expected NO_SUCH_PATH for removed character aria")
	}
	if env.Error == nil || env.Error.Code != "NO_SUCH_PATH" {
		t.Fatalf("expected NO_SUCH_PATH, got %+v", env.Error)
	}

	// Confirm the romance relationship referencing aria is gone.
	env, _ = runCLI(t, dir, "game", "list", "relationships", "g")
	if !env.OK {
		t.Fatalf("list relationships after rm failed: %+v", env.Error)
	}
	relData2 := env.Data.(map[string]any)
	rels2 := relData2["relationships"].([]any)
	if len(rels2) != 0 {
		t.Fatalf("expected 0 relationships after rm aria, got %d", len(rels2))
	}
}

// TestConstructionAddRelationshipBadEndpoint verifies that adding a relationship
// whose endpoint is not a cast member is rejected with INVALID_DEFINITION.
func TestConstructionAddRelationshipBadEndpoint(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	// Add one character, leave "ghost" undefined.
	runCLI(t, dir, "game", "add", "character", "g", "player", "--type", "character")

	env, _ := runCLI(t, dir, "game", "add", "relationship", "g", "romance", "player", "ghost")
	if env.OK {
		t.Fatal("expected INVALID_DEFINITION for relationship to non-existent cast member")
	}
	if env.Error == nil || env.Error.Code != "INVALID_DEFINITION" {
		t.Fatalf("expected INVALID_DEFINITION, got %+v", env.Error)
	}
}

// TestConstructionAddCharacterBadAttrs verifies that passing non-JSON --attrs
// is rejected with BAD_INPUT.
func TestConstructionAddCharacterBadAttrs(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	env, _ := runCLI(t, dir, "game", "add", "character", "g", "aria",
		"--type", "character", "--attrs", "not-json")
	if env.OK {
		t.Fatal("expected BAD_INPUT for non-JSON attrs")
	}
	if env.Error == nil || env.Error.Code != "BAD_INPUT" {
		t.Fatalf("expected BAD_INPUT, got %+v", env.Error)
	}
}

// TestConstructionRmRelationship verifies game rm relationship works directly.
func TestConstructionRmRelationship(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	runCLI(t, dir, "game", "add", "character", "g", "aria", "--type", "character")
	runCLI(t, dir, "game", "add", "character", "g", "player", "--type", "character")
	runCLI(t, dir, "game", "add", "relationship", "g", "romance", "aria", "player")

	// Direct rm relationship.
	env, _ := runCLI(t, dir, "game", "rm", "relationship", "g", "romance", "aria", "player")
	if !env.OK {
		t.Fatalf("rm relationship failed: %+v", env.Error)
	}

	// Confirm it's gone.
	env, _ = runCLI(t, dir, "game", "list", "relationships", "g")
	if !env.OK {
		t.Fatalf("list relationships after rm failed: %+v", env.Error)
	}
	rels := env.Data.(map[string]any)["relationships"].([]any)
	if len(rels) != 0 {
		t.Fatalf("expected 0 relationships, got %d", len(rels))
	}
}

// TestConstructionGiveWithCount verifies --count flag.
func TestConstructionGiveWithCount(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	runCLI(t, dir, "game", "add", "character", "g", "aria", "--type", "character")

	env, _ := runCLI(t, dir, "game", "give", "g", "aria", "--item", "locket", "--count", "3")
	if !env.OK {
		t.Fatalf("give with count failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "get", "g", "entities/aria/inventory/locket")
	if !env.OK {
		t.Fatalf("get inventory locket failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	if val != float64(3) {
		t.Fatalf("expected locket count=3, got %v", val)
	}
}

// TestConstructionGiveEquip verifies --equip flag (requires equippable item with slot).
func TestConstructionGiveEquip(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g2")

	// Entity type with a "trinket" slot accepting "trinket" category.
	runCLI(t, dir, "define", "entity-type", "set", "g2", "hero", "--spec",
		`{"slots":{"trinket":{"accepts":["trinket"]}}}`)
	// Equippable item type in category "trinket".
	runCLI(t, dir, "define", "item-type", "set", "g2", "locket",
		"--spec", `{"equippable":true,"category":"trinket"}`)

	runCLI(t, dir, "game", "add", "character", "g2", "aria", "--type", "hero")

	env, _ := runCLI(t, dir, "game", "give", "g2", "aria",
		"--item", "locket", "--equip", "trinket")
	if !env.OK {
		t.Fatalf("give --equip failed: %+v", env.Error)
	}

	// Verify equipped via game get.
	env, _ = runCLI(t, dir, "game", "get", "g2", "entities/aria/equipped/trinket")
	if !env.OK {
		t.Fatalf("get equipped/trinket failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	if val != "locket" {
		t.Fatalf("expected equipped trinket=locket, got %v", val)
	}
}

// TestAddCharacterDescription verifies --description round-trips via game add
// character and is then visible in game get entities/<id>/description and in the
// runtime state via inspect <run> entities/<id>/description.
func TestAddCharacterDescription(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	const desc = "A sunlit study with mahogany shelves."
	env, _ := runCLI(t, dir, "game", "add", "character", "g", "study",
		"--type", "character", "--description", desc)
	if !env.OK {
		t.Fatalf("add character with description failed: %+v", env.Error)
	}

	// game get entities/study/description should return the authored description.
	env, _ = runCLI(t, dir, "game", "get", "g", "entities/study/description")
	if !env.OK {
		t.Fatalf("game get entities/study/description failed: %+v", env.Error)
	}
	got := env.Data.(map[string]any)["value"]
	if got != desc {
		t.Fatalf("description round-trip: got %v, want %q", got, desc)
	}

	// Start a run, then inspect to confirm the runtime entity has the description.
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")
	env, _ = runCLI(t, dir, "inspect", "run1", "entities/study/description")
	if !env.OK {
		t.Fatalf("inspect entities/study/description failed: %+v", env.Error)
	}
	gotInspect := env.Data.(map[string]any)["value"]
	if gotInspect != desc {
		t.Fatalf("inspect description: got %v, want %q", gotInspect, desc)
	}
}

// TestConstructionValidateOK confirms game validate is clean after construction.
func TestConstructionValidateOK(t *testing.T) {
	dir := t.TempDir()
	seedConstructionGame(t, dir)

	runCLI(t, dir, "game", "add", "character", "g", "aria", "--type", "character",
		"--attrs", `{"name":"Aria"}`)
	runCLI(t, dir, "game", "add", "character", "g", "player", "--type", "character")
	runCLI(t, dir, "game", "add", "relationship", "g", "romance", "aria", "player")

	env, _ := runCLI(t, dir, "game", "validate", "g")
	if !env.OK {
		b, _ := json.Marshal(env)
		t.Fatalf("validate after construction failed: %s", b)
	}
}
