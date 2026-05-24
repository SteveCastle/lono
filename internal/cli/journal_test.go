package cli

import "testing"

// journalGameSpec is a minimal game definition to exercise the record op.
const journalGameSpec = `{
	"id":"jg","version":1,
	"machines":{
		"arc":{"initial":"open","states":["open","ended"],
			"transitions":[{"id":"finish","from":"open","to":"ended"}]}
	}
}`

// TestJournalRecordApplyAndInspect:
// - imports a game, starts a session
// - apply record op
// - asserts returned data["log"] contains the entry
// - inspect <run> log returns the full log with the entry
func TestJournalRecordApplyAndInspect(t *testing.T) {
	dir := t.TempDir()

	if env, _ := runCLI(t, dir, "game", "import", "--spec", journalGameSpec); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "jg", "--id", "jrun1", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}

	// Apply a record op.
	ops := `[{"op":"record","text":"Aria forgave you.","tags":["aria"]}]`
	env, _ := runCLI(t, dir, "apply", "jrun1", "--ops", ops)
	if !env.OK {
		t.Fatalf("apply record failed: %+v", env.Error)
	}

	// The returned data should have a top-level "log" key with the entry.
	data := env.Data.(map[string]any)
	logRaw, ok := data["log"]
	if !ok {
		t.Fatal("apply response should contain top-level 'log' key")
	}
	logSlice, ok := logRaw.([]any)
	if !ok || len(logSlice) == 0 {
		t.Fatalf("log should be a non-empty array, got %T %v", logRaw, logRaw)
	}
	entry := logSlice[len(logSlice)-1].(map[string]any)
	if entry["text"] != "Aria forgave you." {
		t.Fatalf("log entry text mismatch: got %v", entry["text"])
	}

	// inspect <run> log returns the full log array.
	env, _ = runCLI(t, dir, "inspect", "jrun1", "log")
	if !env.OK {
		t.Fatalf("inspect log failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	logSlice2, ok := val.([]any)
	if !ok || len(logSlice2) == 0 {
		t.Fatalf("inspect log should return non-empty array, got %T %v", val, val)
	}
	entry2 := logSlice2[0].(map[string]any)
	if entry2["text"] != "Aria forgave you." {
		t.Fatalf("inspect log entry text mismatch: got %v", entry2["text"])
	}
}

// TestJournalPersistsAcrossSnapshot verifies that the log persists across
// snapshot create + restore into a new instance.
func TestJournalPersistsAcrossSnapshot(t *testing.T) {
	dir := t.TempDir()

	if env, _ := runCLI(t, dir, "game", "import", "--spec", journalGameSpec); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "jg", "--id", "jsave", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}

	// Record a journal entry.
	if env, _ := runCLI(t, dir, "apply", "jsave", "--ops",
		`[{"op":"record","text":"The locket was returned."}]`); !env.OK {
		t.Fatalf("apply record failed: %+v", env.Error)
	}

	// Create snapshot.
	if env, _ := runCLI(t, dir, "snapshot", "create", "jsave", "--id", "snap1"); !env.OK {
		t.Fatalf("snapshot create failed: %+v", env.Error)
	}

	// Restore into a new branch.
	if env, _ := runCLI(t, dir, "snapshot", "restore", "jsave", "snap1", "--into", "jbranch"); !env.OK {
		t.Fatalf("snapshot restore failed: %+v", env.Error)
	}

	// The log should persist in the branched instance.
	env, _ := runCLI(t, dir, "inspect", "jbranch", "log")
	if !env.OK {
		t.Fatalf("inspect log on branch failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	logSlice, ok := val.([]any)
	if !ok || len(logSlice) == 0 {
		t.Fatalf("log should persist across snapshot, got %T %v", val, val)
	}
	entry := logSlice[0].(map[string]any)
	if entry["text"] != "The locket was returned." {
		t.Fatalf("log entry text not preserved: got %v", entry["text"])
	}
}

// TestJournalLastNInStateData verifies the top-level "log" in stateData
// is capped at 10 entries (lastN).
func TestJournalLastNInStateData(t *testing.T) {
	dir := t.TempDir()

	if env, _ := runCLI(t, dir, "game", "import", "--spec", journalGameSpec); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "jg", "--id", "jlastn", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}

	// Apply 12 record ops.
	for i := 0; i < 12; i++ {
		ops := `[{"op":"record","text":"Event number goes here."}]`
		if env, _ := runCLI(t, dir, "apply", "jlastn", "--ops", ops); !env.OK {
			t.Fatalf("apply record %d failed: %+v", i, env.Error)
		}
	}

	// The state command's top-level "log" should show at most 10 entries.
	env, _ := runCLI(t, dir, "state", "jlastn")
	if !env.OK {
		t.Fatalf("state failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	logRaw, ok := data["log"]
	if !ok {
		t.Fatal("state response should contain top-level 'log' key")
	}
	logSlice, ok := logRaw.([]any)
	if !ok {
		t.Fatalf("log should be array, got %T", logRaw)
	}
	if len(logSlice) > 10 {
		t.Fatalf("top-level log should be capped at 10, got %d", len(logSlice))
	}

	// But inspect <run> log should return all 12.
	env, _ = runCLI(t, dir, "inspect", "jlastn", "log")
	if !env.OK {
		t.Fatalf("inspect log failed: %+v", env.Error)
	}
	val := env.Data.(map[string]any)["value"]
	fullLog, ok := val.([]any)
	if !ok || len(fullLog) != 12 {
		t.Fatalf("inspect log should return all 12 entries, got %d", len(fullLog))
	}
}
