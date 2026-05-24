package cli

import "testing"

func TestDoAdvancesState(t *testing.T) {
	dir := t.TempDir()
	seedGame(t, dir)
	runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "do", "run1", "arc", "finish")
	if !env.OK {
		t.Fatalf("do failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	machines := st["machines"].(map[string]any)
	if machines["arc"] != "end" {
		t.Fatalf("machine not advanced: %v", machines["arc"])
	}

	// Action no longer available from "end".
	env, _ = runCLI(t, dir, "do", "run1", "arc", "finish")
	if env.OK {
		t.Fatal("finish should not be available from end")
	}
}
