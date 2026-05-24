package cli

import (
	"encoding/json"
	"testing"
)

// setGameSpec is a minimal game definition with a "character" entity type that
// has a "clues" set attribute (elem:"string") and a transition guarded by a
// contains check on that attribute.
const setGameSpec = `{
	"id":"sg","version":1,
	"entityTypes":{
		"character":{
			"attributes":{
				"clues":{"type":"set","elem":"string"}
			}
		}
	},
	"machines":{
		"arc":{
			"initial":"open",
			"states":["open","solved"],
			"transitions":[{
				"id":"solve",
				"from":"open",
				"to":"solved",
				"guard":{"target":"entity.player.clues","op":"contains","value":"alibi"}
			}]
		}
	},
	"setup":[{"op":"create_entity","entityType":"character","id":"player"}]
}`

func TestSetCollectionEndToEnd(t *testing.T) {
	dir := t.TempDir()

	// Import the game definition.
	env, _ := runCLI(t, dir, "game", "import", "--spec", setGameSpec)
	if !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}

	// Start a play session.
	env, _ = runCLI(t, dir, "play", "start", "sg", "--id", "run1", "--seed", "1")
	if !env.OK {
		t.Fatalf("play start failed: %+v", env.Error)
	}

	// Before add_to: the "solve" transition should be disabled (contains guard fails).
	env, _ = runCLI(t, dir, "actions", "run1")
	if !env.OK {
		t.Fatalf("actions failed: %+v", env.Error)
	}
	actions := env.Data.(map[string]any)["actions"].([]any)
	solveEnabled := false
	for _, a := range actions {
		m := a.(map[string]any)
		if m["action"] == "solve" {
			if enabled, ok := m["enabled"].(bool); ok && enabled {
				solveEnabled = true
			}
		}
	}
	if solveEnabled {
		t.Fatal("solve should be disabled before adding the alibi clue")
	}

	// Apply add_to to add "alibi" to the player's clues set.
	ops := `[{"op":"add_to","target":"entity.player.clues","value":"alibi"}]`
	env, _ = runCLI(t, dir, "apply", "run1", "--ops", ops)
	if !env.OK {
		t.Fatalf("apply add_to failed: %+v", env.Error)
	}

	// Inspect: entities/player/attrs/clues should be ["alibi"].
	env, _ = runCLI(t, dir, "inspect", "run1", "entities/player/attrs/clues")
	if !env.OK {
		t.Fatalf("inspect failed: %+v", env.Error)
	}
	rawVal := env.Data.(map[string]any)["value"]
	b, _ := json.Marshal(rawVal)
	if string(b) != `["alibi"]` {
		t.Fatalf("expected clues=[\"alibi\"], got %s", b)
	}

	// After add_to: the "solve" transition should now be enabled.
	env, _ = runCLI(t, dir, "actions", "run1")
	if !env.OK {
		t.Fatalf("actions after add_to failed: %+v", env.Error)
	}
	actions2 := env.Data.(map[string]any)["actions"].([]any)
	solveEnabled2 := false
	for _, a := range actions2 {
		m := a.(map[string]any)
		if m["action"] == "solve" {
			if enabled, ok := m["enabled"].(bool); ok && enabled {
				solveEnabled2 = true
			}
		}
	}
	if !solveEnabled2 {
		t.Fatal("solve should be enabled after adding the alibi clue")
	}

	// duplicate add_to is a no-op: still one element.
	env, _ = runCLI(t, dir, "apply", "run1", "--ops", ops)
	if !env.OK {
		t.Fatalf("apply duplicate add_to failed: %+v", env.Error)
	}
	env, _ = runCLI(t, dir, "inspect", "run1", "entities/player/attrs/clues")
	if !env.OK {
		t.Fatalf("inspect after dup add_to failed: %+v", env.Error)
	}
	rawVal2 := env.Data.(map[string]any)["value"]
	b2, _ := json.Marshal(rawVal2)
	if string(b2) != `["alibi"]` {
		t.Fatalf("expected clues still [\"alibi\"] after dup, got %s", b2)
	}

	// clear empties the set.
	env, _ = runCLI(t, dir, "apply", "run1", "--ops", `[{"op":"clear","target":"entity.player.clues"}]`)
	if !env.OK {
		t.Fatalf("apply clear failed: %+v", env.Error)
	}
	env, _ = runCLI(t, dir, "inspect", "run1", "entities/player/attrs/clues")
	if !env.OK {
		t.Fatalf("inspect after clear failed: %+v", env.Error)
	}
	rawVal3 := env.Data.(map[string]any)["value"]
	b3, _ := json.Marshal(rawVal3)
	if string(b3) != `[]` {
		t.Fatalf("expected clues=[] after clear, got %s", b3)
	}
}

func TestValidateSetSpec(t *testing.T) {
	dir := t.TempDir()

	// Valid set spec (elem:"string") should import cleanly.
	valid := `{"id":"vg","version":1,"entityTypes":{"character":{"attributes":{"tags":{"type":"set","elem":"string"}}}}}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", valid)
	if !env.OK {
		t.Fatalf("valid set spec import failed: %+v", env.Error)
	}

	// Invalid set spec (unknown elem) should be rejected.
	invalid := `{"id":"ig","version":1,"entityTypes":{"character":{"attributes":{"tags":{"type":"set","elem":"number"}}}}}`
	env, _ = runCLI(t, dir, "game", "import", "--spec", invalid)
	if env.OK {
		t.Fatal("import with invalid set elem should fail")
	}
}
