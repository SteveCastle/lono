package cli

import "testing"

// TestDefineAliasItem verifies that `define item set` behaves like `define item-type set`.
func TestDefineAliasItem(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	env, _ := runCLI(t, dir, "define", "item", "set", "g", "sword",
		"--spec", `{"category":"weapon"}`)
	if !env.OK {
		t.Fatalf("define item set failed: %+v", env.Error)
	}

	// Confirm it appears under itemTypes via game get.
	env, _ = runCLI(t, dir, "game", "get", "g", "itemTypes/sword")
	if !env.OK {
		t.Fatalf("game get itemTypes/sword failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"].(map[string]any)
	if val["category"] != "weapon" {
		t.Fatalf("expected category=weapon, got %v", val["category"])
	}
}

// TestDefineAliasEvent verifies that `define event set` behaves like `define beat set`.
func TestDefineAliasEvent(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	env, _ := runCLI(t, dir, "define", "event", "set", "g", "intro",
		"--spec", `{"text":"hi"}`)
	if !env.OK {
		t.Fatalf("define event set failed: %+v", env.Error)
	}

	// Confirm it appears under beats via game get.
	env, _ = runCLI(t, dir, "game", "get", "g", "beats/intro")
	if !env.OK {
		t.Fatalf("game get beats/intro failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"].(map[string]any)
	if val["text"] != "hi" {
		t.Fatalf("expected text=hi, got %v", val["text"])
	}
}

// TestDefineAliasBranch verifies that `define branch set` behaves like `define transition set`.
func TestDefineAliasBranch(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// First create a machine with two states.
	runCLI(t, dir, "define", "machine", "set", "g", "arc",
		"--spec", `{"initial":"a","states":["a","b"]}`)

	env, _ := runCLI(t, dir, "define", "branch", "set", "g", "arc",
		"--spec", `{"id":"go","from":"a","to":"b"}`)
	if !env.OK {
		t.Fatalf("define branch set failed: %+v", env.Error)
	}

	// Confirm transition appears under machines/arc/transitions.
	env, _ = runCLI(t, dir, "game", "get", "g", "machines/arc/transitions/go")
	if !env.OK {
		t.Fatalf("game get machines/arc/transitions/go failed: %+v", env.Error)
	}
	tr := env.Data.(map[string]any)["value"].(map[string]any)
	if tr["to"] != "b" {
		t.Fatalf("expected to=b, got %v", tr["to"])
	}
}

// TestDefineAliasRelationshipType verifies `define relationship-type set` aliases rel-type.
func TestDefineAliasRelationshipType(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// Need entity types first so the relationship-type validation passes.
	runCLI(t, dir, "define", "entity-type", "set", "g", "person", "--spec", `{}`)

	env, _ := runCLI(t, dir, "define", "relationship-type", "set", "g", "knows",
		"--spec", `{"from":"person","to":"person","directed":false}`)
	if !env.OK {
		t.Fatalf("define relationship-type set failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "get", "g", "relationshipTypes/knows")
	if !env.OK {
		t.Fatalf("game get relationshipTypes/knows failed: %+v", env.Error)
	}
}

// TestDefineScene verifies `define scene set/rm` writes to stateMeta.
func TestDefineScene(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// Create a machine with states.
	runCLI(t, dir, "define", "machine", "set", "g", "arc",
		"--spec", `{"initial":"start","states":["start","ending_good"]}`)

	// Set scene meta for a state.
	env, _ := runCLI(t, dir, "define", "scene", "set", "g", "arc", "ending_good",
		"--spec", `{"terminal":true,"ending":true,"description":"win"}`)
	if !env.OK {
		t.Fatalf("define scene set failed: %+v", env.Error)
	}

	// Confirm it appears under machines/arc/stateMeta/ending_good.
	env, _ = runCLI(t, dir, "game", "get", "g", "machines/arc/stateMeta/ending_good")
	if !env.OK {
		t.Fatalf("game get machines/arc/stateMeta/ending_good failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"].(map[string]any)
	if val["description"] != "win" {
		t.Fatalf("expected description=win, got %v", val["description"])
	}
	if val["terminal"] != true {
		t.Fatalf("expected terminal=true, got %v", val["terminal"])
	}
	if val["ending"] != true {
		t.Fatalf("expected ending=true, got %v", val["ending"])
	}

	// Remove the scene meta.
	env, _ = runCLI(t, dir, "define", "scene", "rm", "g", "arc", "ending_good")
	if !env.OK {
		t.Fatalf("define scene rm failed: %+v", env.Error)
	}

	// Confirm gone.
	env, _ = runCLI(t, dir, "game", "get", "g", "machines/arc/stateMeta/ending_good")
	if env.OK {
		t.Fatal("expected NO_SUCH_PATH after scene rm")
	}
	if env.Error == nil || env.Error.Code != "NO_SUCH_PATH" {
		t.Fatalf("expected NO_SUCH_PATH, got %+v", env.Error)
	}
}

// TestDefineSceneUnknownState verifies scene set is rejected for a state not in the machine.
func TestDefineSceneUnknownState(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")
	runCLI(t, dir, "define", "machine", "set", "g", "arc",
		"--spec", `{"initial":"start","states":["start"]}`)

	env, _ := runCLI(t, dir, "define", "scene", "set", "g", "arc", "ghost",
		"--spec", `{"description":"phantom"}`)
	if env.OK {
		t.Fatal("expected INVALID_DEFINITION for scene on unknown state")
	}
	if env.Error == nil || env.Error.Code != "INVALID_DEFINITION" {
		t.Fatalf("expected INVALID_DEFINITION, got %+v", env.Error)
	}
}

// TestDefineAliasItemRm verifies the alias works for rm too.
func TestDefineAliasItemRm(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")
	runCLI(t, dir, "define", "item", "set", "g", "sword", "--spec", `{}`)

	env, _ := runCLI(t, dir, "define", "item", "rm", "g", "sword")
	if !env.OK {
		t.Fatalf("define item rm failed: %+v", env.Error)
	}

	// Confirm gone.
	env, _ = runCLI(t, dir, "game", "get", "g", "itemTypes/sword")
	if env.OK {
		t.Fatal("expected NO_SUCH_PATH after item rm")
	}
}
