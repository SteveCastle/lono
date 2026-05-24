package cli

import "testing"

func TestApplyUpdatesState(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "apply", "run1", "--ops", `[{"op":"set","target":"entity.player.health","value":50}]`)
	if !env.OK {
		t.Fatalf("apply failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	player := st["entities"].(map[string]any)["player"].(map[string]any)
	if player["attrs"].(map[string]any)["health"].(float64) != 50 {
		t.Fatalf("apply did not set health: %+v", player)
	}

	// Out-of-bounds rejected (no max here, so use a bad op: unknown target).
	env, _ = runCLI(t, dir, "apply", "run1", "--ops", `[{"op":"set","target":"world.nope","value":1}]`)
	if env.OK {
		t.Fatal("apply with unknown target should fail")
	}
}
