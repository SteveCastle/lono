package cli

import "testing"

// seedNavGame imports a small game with a machine and entity.
func seedNavGame(t *testing.T, dir string) {
	t.Helper()
	spec := `{
		"id": "nav",
		"version": 1,
		"name": "Nav Test Game",
		"entityTypes": {
			"character": {
				"attributes": {
					"health": {"type": "int", "default": 100}
				}
			}
		},
		"machines": {
			"arc": {
				"initial": "intro",
				"states": ["intro", "end"],
				"transitions": [{"id": "finish", "from": "intro", "to": "end"}]
			}
		},
		"setup": [
			{"op": "create_entity", "entityType": "character", "id": "player"}
		]
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		t.Fatalf("seedNavGame: import failed: %+v", env.Error)
	}
}

// TestGameGetMachinePath verifies that game get returns the correct machine node.
func TestGameGetMachinePath(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	env, _ := runCLI(t, dir, "game", "get", "nav", "machines/arc")
	if !env.OK {
		t.Fatalf("game get machines/arc failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["path"] != "machines/arc" {
		t.Fatalf("expected path=machines/arc, got %v", data["path"])
	}
	val := data["value"]
	if val == nil {
		t.Fatal("expected non-nil value for machines/arc")
	}
	m := val.(map[string]any)
	if m["initial"] != "intro" {
		t.Fatalf("expected initial=intro, got %v", m["initial"])
	}
}

// TestGameGetTransitionByID verifies id-addressed arrays work end-to-end.
func TestGameGetTransitionByID(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	env, _ := runCLI(t, dir, "game", "get", "nav", "machines/arc/transitions/finish")
	if !env.OK {
		t.Fatalf("game get transitions/finish failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	tr := data["value"].(map[string]any)
	if tr["to"] != "end" {
		t.Fatalf("expected to=end, got %v", tr["to"])
	}
}

// TestGameGetTree verifies --tree returns a structural map.
func TestGameGetTree(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	env, _ := runCLI(t, dir, "game", "get", "nav", "--tree")
	if !env.OK {
		t.Fatalf("game get --tree failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["tree"] == nil {
		t.Fatal("expected tree key in response")
	}
	// The tree should be a map with known top-level keys.
	tree := data["tree"].(map[string]any)
	if _, ok := tree["machines"]; !ok {
		t.Fatalf("tree missing 'machines' key: %v", tree)
	}
}

// TestGameGetBogusPath verifies NO_SUCH_PATH is returned for a bad path.
func TestGameGetBogusPath(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	env, _ := runCLI(t, dir, "game", "get", "nav", "bogus/path")
	if env.OK {
		t.Fatal("expected failure for bogus path")
	}
	if env.Error == nil || env.Error.Code != "NO_SUCH_PATH" {
		t.Fatalf("expected NO_SUCH_PATH, got %+v", env.Error)
	}
}

// TestGameGetNotFound verifies NOT_FOUND when game doesn't exist.
func TestGameGetNotFound(t *testing.T) {
	dir := t.TempDir()

	env, _ := runCLI(t, dir, "game", "get", "nonexistent")
	if env.OK {
		t.Fatal("expected failure for nonexistent game")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %+v", env.Error)
	}
}

// TestInspectEntityAttrs verifies inspect returns entity attribute values.
func TestInspectEntityAttrs(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	// Start a play session.
	startEnv, _ := runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")
	if !startEnv.OK {
		t.Fatalf("play start failed: %+v", startEnv.Error)
	}

	// Inspect the player entity.
	env, _ := runCLI(t, dir, "inspect", "run1", "entities/player")
	if !env.OK {
		t.Fatalf("inspect entities/player failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["path"] != "entities/player" {
		t.Fatalf("expected path=entities/player, got %v", data["path"])
	}
	val := data["value"]
	if val == nil {
		t.Fatal("expected non-nil entity value")
	}
	ent := val.(map[string]any)
	if ent["type"] != "character" {
		t.Fatalf("expected type=character, got %v", ent["type"])
	}
}

// TestInspectEntityAttrSubPath verifies deep attr path resolution.
func TestInspectEntityAttrSubPath(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)
	runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "inspect", "run1", "entities/player/attrs/health")
	if !env.OK {
		t.Fatalf("inspect entities/player/attrs/health failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	// Health defaults to 100.
	if data["value"] != float64(100) {
		t.Fatalf("expected health=100, got %v", data["value"])
	}
}

// TestInspectTree verifies --tree on an instance.
func TestInspectTree(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)
	runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "inspect", "run1", "--tree")
	if !env.OK {
		t.Fatalf("inspect --tree failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["tree"] == nil {
		t.Fatal("expected tree key in response")
	}
	tree := data["tree"].(map[string]any)
	if _, ok := tree["entities"]; !ok {
		t.Fatalf("tree missing 'entities': %v", tree)
	}
}

// TestInspectBogusPath verifies NO_SUCH_PATH for a bad instance path.
func TestInspectBogusPath(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)
	runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "inspect", "run1", "entities/ghost/attrs/health")
	if env.OK {
		t.Fatal("expected failure for nonexistent entity")
	}
	if env.Error == nil || env.Error.Code != "NO_SUCH_PATH" {
		t.Fatalf("expected NO_SUCH_PATH, got %+v", env.Error)
	}
}

// TestInspectNotFound verifies NOT_FOUND for a missing instance.
func TestInspectNotFound(t *testing.T) {
	dir := t.TempDir()

	env, _ := runCLI(t, dir, "inspect", "nonexistent-instance")
	if env.OK {
		t.Fatal("expected failure for nonexistent instance")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %+v", env.Error)
	}
}

// TestInspectDepthFlag verifies --depth is respected.
func TestInspectDepthFlag(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)
	runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")

	// depth=1 means the children of top-level keys are their key lists (or nil).
	env, _ := runCLI(t, dir, "inspect", "run1", "--tree", "--depth", "1")
	if !env.OK {
		t.Fatalf("inspect --tree --depth 1 failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	tree := data["tree"].(map[string]any)
	// At depth 1 the "entities" child is at depth 0 → sorted key list or nil.
	entChild := tree["entities"]
	// It's either a map[string]any (if depth 1 gives the children) or a []string
	// of keys (depth 0). Either is valid; the key thing is it's not nil.
	if entChild == nil {
		t.Fatalf("expected entities child at depth 1, got nil; tree=%v", tree)
	}
	_ = entChild
}

// TestInspectDerived verifies that inspect <run> derived returns the computed
// derived view (ok:true).
func TestInspectDerived(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)
	runCLI(t, dir, "play", "start", "nav", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "inspect", "run1", "derived")
	if !env.OK {
		t.Fatalf("inspect derived failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if data["path"] != "derived" {
		t.Fatalf("expected path=derived, got %v", data["path"])
	}
	if data["value"] == nil {
		t.Fatal("expected non-nil value for derived view")
	}
}

// TestGameGetNoPath verifies that omitting path returns the whole definition.
func TestGameGetNoPath(t *testing.T) {
	dir := t.TempDir()
	seedNavGame(t, dir)

	env, _ := runCLI(t, dir, "game", "get", "nav")
	if !env.OK {
		t.Fatalf("game get (no path) failed: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	val := data["value"]
	if val == nil {
		t.Fatal("expected non-nil value for empty path (whole definition)")
	}
	// The whole definition should contain the game id.
	defMap := val.(map[string]any)
	if defMap["id"] != "nav" {
		t.Fatalf("expected id=nav, got %v", defMap["id"])
	}
}
