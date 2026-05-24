package cli

import "testing"

func TestGameCreateShowListDelete(t *testing.T) {
	dir := t.TempDir()

	env, _ := runCLI(t, dir, "game", "create", "heist", "--name", "The Heist")
	if !env.OK {
		t.Fatalf("create failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "list")
	if !env.OK {
		t.Fatalf("list failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "show", "heist")
	if !env.OK {
		t.Fatalf("show failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "delete", "heist")
	if !env.OK {
		t.Fatalf("delete failed: %+v", env.Error)
	}

	env, _ = runCLI(t, dir, "game", "show", "heist")
	if env.OK {
		t.Fatal("show should fail after delete")
	}
}

func TestGameImportValidateExport(t *testing.T) {
	dir := t.TempDir()
	def := `{"id":"g","version":1,"machines":{"arc":{"initial":"x","states":["x"]}}}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", def)
	if !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	env, _ = runCLI(t, dir, "game", "validate", "g")
	if !env.OK {
		t.Fatalf("validate failed: %+v", env.Error)
	}

	// Invalid: initial not in states.
	bad := `{"id":"b","version":1,"machines":{"arc":{"initial":"nope","states":["x"]}}}`
	env, _ = runCLI(t, dir, "game", "import", "--spec", bad)
	if env.OK {
		t.Fatal("import of invalid def should fail")
	}
}

func TestGameCreateRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "heist", "--name", "Heist")
	if env, _ := runCLI(t, dir, "define", "machine", "set", "heist", "arc", "--spec", `{"initial":"a","states":["a"]}`); !env.OK {
		t.Fatalf("define failed: %+v", env.Error)
	}
	// Re-create without --force must be refused and must NOT wipe existing content.
	env, _ := runCLI(t, dir, "game", "create", "heist")
	if env.OK || env.Error == nil || env.Error.Code != "GAME_EXISTS" {
		t.Fatalf("expected GAME_EXISTS, got %+v", env)
	}
	env, _ = runCLI(t, dir, "game", "show", "heist")
	if _, ok := env.Data.(map[string]any)["machines"].(map[string]any)["arc"]; !ok {
		t.Fatal("refused re-create wiped the game's machine")
	}
	// --force allows a deliberate fresh recreate.
	if env, _ := runCLI(t, dir, "game", "create", "heist", "--force"); !env.OK {
		t.Fatalf("create --force failed: %+v", env.Error)
	}
}
