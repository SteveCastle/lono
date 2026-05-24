package cli

import (
	"encoding/json"
	"testing"
)

// seedBoundedGame creates a game with a character entity-type whose health
// attr has min=0 max=100, and seeds a "player" cast member via def.Entities.
func seedBoundedGame(t *testing.T, dir string) {
	t.Helper()
	spec := `{
		"id":"bg","version":1,
		"entityTypes":{"character":{"attributes":{"health":{"type":"int","default":100,"min":0,"max":100}}}},
		"entities":{"player":{"type":"character","attrs":{"health":100}}},
		"machines":{"arc":{"initial":"start","states":["start","end"],
		  "transitions":[{"id":"finish","from":"start","to":"end"}]}}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		b, _ := json.Marshal(env)
		t.Fatalf("seedBoundedGame import failed: %s", b)
	}
}

// TestSetValidated confirms that set applies a validated write and returns
// updated state.
func TestSetValidated(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r1", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "r1", "entities/player/attrs/health", "--value", "80")
	if !env.OK {
		t.Fatalf("set health=80 failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	player := st["entities"].(map[string]any)["player"].(map[string]any)
	if player["attrs"].(map[string]any)["health"] != float64(80) {
		t.Fatalf("expected health=80, got %v", player["attrs"].(map[string]any)["health"])
	}
}

// TestSetOutOfBoundsRejected confirms that setting health above max=100 fails
// (the engine validates the set op and rejects it).
func TestSetOutOfBoundsRejected(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r2", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "r2", "entities/player/attrs/health", "--value", "999")
	if env.OK {
		t.Fatal("expected failure for out-of-bounds health=999 but got OK")
	}
	if env.Error == nil {
		t.Fatal("expected error info for out-of-bounds set")
	}
}

// TestSetForceAllowsOutOfBounds confirms that --force bypasses validation and
// allows writing an out-of-bounds value directly.
func TestSetForceAllowsOutOfBounds(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r3", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "r3", "entities/player/attrs/health", "--value", "999", "--force")
	if !env.OK {
		t.Fatalf("set --force health=999 failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	player := st["entities"].(map[string]any)["player"].(map[string]any)
	if player["attrs"].(map[string]any)["health"] != float64(999) {
		t.Fatalf("expected health=999 via force, got %v", player["attrs"].(map[string]any)["health"])
	}
}

// TestRmDestroyEntity confirms that rm entities/<id> destroys the entity.
func TestRmDestroyEntity(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r4", "--seed", "1")

	env, _ := runCLI(t, dir, "rm", "r4", "entities/player")
	if !env.OK {
		t.Fatalf("rm entities/player failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	entities := st["entities"].(map[string]any)
	if _, exists := entities["player"]; exists {
		t.Fatal("expected player to be gone after rm")
	}
}

// TestSetPathNotWritable confirms that writing to a computed/read-only path
// returns PATH_NOT_WRITABLE.
func TestSetPathNotWritable(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r5", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "r5", "derived/x", "--value", "1")
	if env.OK {
		t.Fatal("expected failure for derived/x write but got OK")
	}
	if env.Error == nil || env.Error.Code != "PATH_NOT_WRITABLE" {
		t.Fatalf("expected PATH_NOT_WRITABLE, got %+v", env.Error)
	}
}

// TestSetBoolValue confirms that a boolean --value is parsed via JSON (not as a
// literal string "true").
func TestSetBoolValue(t *testing.T) {
	dir := t.TempDir()
	// Create a game with a bool world var.
	spec := `{
		"id":"gb","version":1,
		"world":{"active":{"type":"bool","default":false}},
		"machines":{"arc":{"initial":"start","states":["start"]}}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	runCLI(t, dir, "play", "start", "gb", "--id", "rb1", "--seed", "1")

	env, _ = runCLI(t, dir, "set", "rb1", "world/active", "--value", "true")
	if !env.OK {
		t.Fatalf("set bool failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	if st["world"].(map[string]any)["active"] != true {
		t.Fatalf("expected active=true, got %v", st["world"].(map[string]any)["active"])
	}
}

// TestSetMachineState confirms that setting a global machine state works.
func TestSetMachineState(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r6", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "r6", "machines/arc", "--value", "end")
	if !env.OK {
		t.Fatalf("set machines/arc failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	if st["machines"].(map[string]any)["arc"] != "end" {
		t.Fatalf("expected arc=end, got %v", st["machines"].(map[string]any)["arc"])
	}
}

// TestRmPathNotWritable confirms rm on a non-removable path returns
// PATH_NOT_WRITABLE.
func TestRmPathNotWritable(t *testing.T) {
	dir := t.TempDir()
	seedBoundedGame(t, dir)
	runCLI(t, dir, "play", "start", "bg", "--id", "r7", "--seed", "1")

	// world/<v> is not removable.
	env, _ := runCLI(t, dir, "rm", "r7", "world/day")
	if env.OK {
		t.Fatal("expected failure for rm world/day but got OK")
	}
	if env.Error == nil || env.Error.Code != "PATH_NOT_WRITABLE" {
		t.Fatalf("expected PATH_NOT_WRITABLE, got %+v", env.Error)
	}
}

// TestSetInstanceNotFound confirms a NOT_FOUND error for a missing instance.
func TestSetInstanceNotFound(t *testing.T) {
	dir := t.TempDir()
	env, _ := runCLI(t, dir, "set", "no-such", "entities/x/attrs/y", "--value", "1")
	if env.OK {
		t.Fatal("expected failure for missing instance")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %+v", env.Error)
	}
}

// TestSetValidatedFiresTriggers confirms that a validated runtime set settles
// reactive triggers and surfaces the fired trigger ids in the response.
func TestSetValidatedFiresTriggers(t *testing.T) {
	dir := t.TempDir()
	seedAlarmGame(t, dir)
	runCLI(t, dir, "play", "start", "ag", "--id", "sf1", "--seed", "1")

	env, _ := runCLI(t, dir, "set", "sf1", "world/alarm", "--value", "true")
	if !env.OK {
		t.Fatalf("set world/alarm=true failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	fired, _ := data["fired"].([]any)
	if !containsStr(fired, "raise") {
		t.Fatalf("expected fired to contain trigger \"raise\", got %v", data["fired"])
	}
	st := data["state"].(map[string]any)
	if st["world"].(map[string]any)["lockdown"] != true {
		t.Fatalf("expected lockdown=true after trigger fired, got %v", st["world"].(map[string]any)["lockdown"])
	}
}

// containsStr reports whether xs (a decoded JSON array) contains the string s.
func containsStr(xs []any, s string) bool {
	for _, x := range xs {
		if str, ok := x.(string); ok && str == s {
			return true
		}
	}
	return false
}

// TestSetEmptyStringValue confirms that --value "" sets the attribute to an
// empty string (not treated as "no value provided").
func TestSetEmptyStringValue(t *testing.T) {
	dir := t.TempDir()
	spec := `{
		"id":"es","version":1,
		"entityTypes":{"thing":{"attributes":{"label":{"type":"string","default":"hello"}}}},
		"entities":{"obj":{"type":"thing","attrs":{"label":"hello"}}},
		"machines":{"arc":{"initial":"start","states":["start"]}}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		b, _ := json.Marshal(env)
		t.Fatalf("import failed: %s", b)
	}
	runCLI(t, dir, "play", "start", "es", "--id", "es1", "--seed", "1")

	env, _ = runCLI(t, dir, "set", "es1", "entities/obj/attrs/label", "--value", "")
	if !env.OK {
		t.Fatalf("set --value \"\" failed: %+v", env.Error)
	}
	st := env.Data.(map[string]any)["state"].(map[string]any)
	obj := st["entities"].(map[string]any)["obj"].(map[string]any)
	got := obj["attrs"].(map[string]any)["label"]
	if got != "" {
		t.Fatalf("expected label=\"\", got %v", got)
	}
}
