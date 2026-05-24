package cli

import "testing"

func TestDefineBeatAndOutput(t *testing.T) {
	dir := t.TempDir()
	def := `{"id":"g","version":1,
	  "machines":{"arc":{"initial":"intro","states":["intro","ending_good"],
	    "stateMeta":{"ending_good":{"terminal":true,"ending":true,"description":"You win."}},
	    "transitions":[{"id":"finish","from":"intro","to":"ending_good"}]}}}`
	if env, _ := runCLI(t, dir, "game", "import", "--spec", def); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	env, _ := runCLI(t, dir, "define", "beat", "set", "g", "intro_beat", "--spec",
		`{"text":"You step inside.","machineState":{"machine":"arc","state":"intro"}}`)
	if !env.OK {
		t.Fatalf("define beat failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}
	// At intro: the beat is active, no ending reached.
	env, _ = runCLI(t, dir, "state", "run1")
	beats := env.Data.(map[string]any)["beats"].([]any)
	if len(beats) != 1 || beats[0].(map[string]any)["id"] != "intro_beat" {
		t.Fatalf("expected intro_beat active, got %v", beats)
	}
	if er := env.Data.(map[string]any)["endingReached"]; er != nil && len(er.([]any)) != 0 {
		t.Fatalf("no ending expected at intro, got %v", er)
	}
	// Finish -> ending reached, intro beat no longer active.
	env, _ = runCLI(t, dir, "do", "run1", "arc", "finish")
	endings := env.Data.(map[string]any)["endingReached"].([]any)
	if len(endings) != 1 || endings[0].(map[string]any)["state"] != "ending_good" {
		t.Fatalf("expected ending_good, got %v", endings)
	}
}
