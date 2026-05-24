package cli

import "testing"

func TestSnapshotCreateListRestore(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "snapshot", "create", "run1", "--id", "s1", "--label", "before")
	if !env.OK {
		t.Fatalf("create failed: %+v", env.Error)
	}

	// Mutate, then restore the snapshot into a new branched instance.
	runCLI(t, dir, "apply", "run1", "--ops", `[{"op":"set","target":"entity.player.health","value":10}]`)

	env, _ = runCLI(t, dir, "snapshot", "list", "run1")
	if !env.OK || env.Data.(map[string]any)["snapshots"] == nil {
		t.Fatalf("list failed: %+v", env)
	}

	env, _ = runCLI(t, dir, "snapshot", "restore", "run1", "s1", "--into", "branch1")
	if !env.OK {
		t.Fatalf("restore failed: %+v", env.Error)
	}

	// Branched instance has the pre-mutation health (100), original keeps 10.
	env, _ = runCLI(t, dir, "state", "branch1")
	bh := env.Data.(map[string]any)["state"].(map[string]any)["entities"].(map[string]any)["player"].(map[string]any)["attrs"].(map[string]any)["health"].(float64)
	if bh != 100 {
		t.Fatalf("branch should have restored health 100, got %v", bh)
	}
}

func TestRestoreFlagGuards(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")
	runCLI(t, dir, "snapshot", "create", "run1", "--id", "s1")

	// --in-place and --into are mutually exclusive.
	env, _ := runCLI(t, dir, "snapshot", "restore", "run1", "s1", "--in-place", "--into", "x")
	if env.OK || env.Error.Code != "BAD_INPUT" {
		t.Fatalf("expected BAD_INPUT for conflicting flags, got %+v", env)
	}

	// --into an existing instance must be refused (no clobber).
	env, _ = runCLI(t, dir, "snapshot", "restore", "run1", "s1", "--into", "run1")
	if env.OK || env.Error.Code != "INSTANCE_EXISTS" {
		t.Fatalf("expected INSTANCE_EXISTS for existing branch target, got %+v", env)
	}

	// --in-place succeeds.
	if env, _ := runCLI(t, dir, "snapshot", "restore", "run1", "s1", "--in-place"); !env.OK {
		t.Fatalf("in-place restore failed: %+v", env.Error)
	}
}
