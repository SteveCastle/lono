package cli

import "testing"

func TestStateAndActions(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "state", "run1")
	if !env.OK {
		t.Fatalf("state failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["state"] == nil || data["actions"] == nil {
		t.Fatalf("state payload missing fields: %+v", data)
	}

	env, _ = runCLI(t, dir, "actions", "run1")
	if !env.OK {
		t.Fatalf("actions failed: %+v", env.Error)
	}
	if env.Data.(map[string]any)["actions"] == nil {
		t.Fatal("actions payload missing actions")
	}

	env, _ = runCLI(t, dir, "state", "nope")
	if env.OK {
		t.Fatal("state on missing instance should fail")
	}
}
