package cli

import "testing"

func seedGame(t *testing.T, dir string) {
	runCLI(t, dir, "game", "create", "g")
	runCLI(t, dir, "define", "entity-type", "set", "g", "character", "--spec", `{"attributes":{"health":{"type":"int","default":100}}}`)
	runCLI(t, dir, "define", "machine", "set", "g", "arc", "--spec", `{"initial":"intro","states":["intro","end"]}`)
	runCLI(t, dir, "define", "transition", "set", "g", "arc", "--spec", `{"id":"finish","from":"intro","to":"end"}`)
	// setup creates the player
	runCLI(t, dir, "game", "import", "--spec", `{"id":"g","version":1,
		"entityTypes":{"character":{"attributes":{"health":{"type":"int","default":100}}}},
		"machines":{"arc":{"initial":"intro","states":["intro","end"],
		  "transitions":[{"id":"finish","from":"intro","to":"end"}]}},
		"setup":[{"op":"create_entity","entityType":"character","id":"player"}]}`)
}

func TestPlayStartAndList(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)

	env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "42")
	if !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["state"] == nil || data["actions"] == nil {
		t.Fatalf("start should return state+actions: %+v", data)
	}

	env, _ = runCLI(t, dir, "play", "list")
	if !env.OK {
		t.Fatalf("list failed: %+v", env.Error)
	}
}

func TestPlayStartRejectsExistingId(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)

	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1"); !env.OK {
		t.Fatalf("first start failed: %+v", env.Error)
	}
	// Second start with the same id must be refused (no silent clobber).
	env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1")
	if env.OK || env.Error == nil || env.Error.Code != "INSTANCE_EXISTS" {
		t.Fatalf("expected INSTANCE_EXISTS, got %+v", env)
	}
	// --force allows overwrite.
	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1", "--force"); !env.OK {
		t.Fatalf("start --force failed: %+v", env.Error)
	}
}
