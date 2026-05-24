package cli

import "testing"

func TestDefineBuildsUpDefinition(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	steps := [][]string{
		{"define", "var", "set", "g", "day", "--spec", `{"type":"int","default":1,"min":1}`},
		{"define", "entity-type", "set", "g", "character", "--spec", `{"attributes":{"health":{"type":"int","default":100}}}`},
		{"define", "item-type", "set", "g", "gold", "--spec", `{"maxStack":1000}`},
		{"define", "machine", "set", "g", "arc", "--spec", `{"initial":"intro","states":["intro","end"]}`},
		{"define", "transition", "set", "g", "arc", "--spec", `{"id":"go","from":"intro","to":"end"}`},
	}
	for _, s := range steps {
		env, _ := runCLI(t, dir, s...)
		if !env.OK {
			t.Fatalf("%v failed: %+v", s, env.Error)
		}
	}

	// A transition pointing at a non-existent state must be rejected by validation.
	env, _ := runCLI(t, dir, "define", "transition", "set", "g", "arc", "--spec", `{"id":"bad","from":"intro","to":"void"}`)
	if env.OK {
		t.Fatal("expected invalid-definition rejection")
	}

	// Remove a var.
	env, _ = runCLI(t, dir, "define", "var", "rm", "g", "day")
	if !env.OK {
		t.Fatalf("var rm failed: %+v", env.Error)
	}
}
