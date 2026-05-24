package cli

import (
	"encoding/json"
	"testing"
)

// TestDefineLore exercises define lore set/rm via the CLI.
func TestDefineLore(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// set a lore entry
	env, _ := runCLI(t, dir, "define", "lore", "set", "g", "founding",
		"--spec", `{"title":"The Founding","text":"Built in Year 312.","tags":["history"],"when":"Year 312"}`)
	if !env.OK {
		t.Fatalf("define lore set failed: %+v", env.Error)
	}

	// show the game and confirm lore is present
	env2, _ := runCLI(t, dir, "game", "show", "g")
	if !env2.OK {
		t.Fatalf("game show failed: %+v", env2.Error)
	}
	b, _ := json.Marshal(env2.Data)
	var def map[string]any
	if err := json.Unmarshal(b, &def); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	lore, ok := def["lore"].(map[string]any)
	if !ok {
		t.Fatalf("lore missing from game definition: %v", def)
	}
	founding, ok := lore["founding"].(map[string]any)
	if !ok {
		t.Fatalf("founding lore entry missing")
	}
	if founding["title"] != "The Founding" {
		t.Errorf("title: got %v", founding["title"])
	}
	if founding["when"] != "Year 312" {
		t.Errorf("when: got %v", founding["when"])
	}

	// set a second entry
	env3, _ := runCLI(t, dir, "define", "lore", "set", "g", "locket",
		"--spec", `{"title":"The Locket","text":"Silver locket with initials E.A.","subject":"player"}`)
	if !env3.OK {
		t.Fatalf("define lore set locket failed: %+v", env3.Error)
	}

	// remove the first entry
	env4, _ := runCLI(t, dir, "define", "lore", "rm", "g", "founding")
	if !env4.OK {
		t.Fatalf("define lore rm failed: %+v", env4.Error)
	}

	// confirm founding is gone and locket remains
	env5, _ := runCLI(t, dir, "game", "show", "g")
	b2, _ := json.Marshal(env5.Data)
	var def2 map[string]any
	_ = json.Unmarshal(b2, &def2)
	lore2, _ := def2["lore"].(map[string]any)
	if _, exists := lore2["founding"]; exists {
		t.Error("founding should have been removed")
	}
	if _, exists := lore2["locket"]; !exists {
		t.Error("locket should still exist")
	}
}

// TestDefineLoreValidation verifies that define lore rejects entries with empty
// title or text (validation fires before save).
func TestDefineLoreValidation(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// empty title → should fail
	env, _ := runCLI(t, dir, "define", "lore", "set", "g", "bad",
		"--spec", `{"title":"","text":"some text"}`)
	if env.OK {
		t.Fatal("expected failure for empty title")
	}

	// empty text → should fail
	env2, _ := runCLI(t, dir, "define", "lore", "set", "g", "bad2",
		"--spec", `{"title":"some title","text":""}`)
	if env2.OK {
		t.Fatal("expected failure for empty text")
	}
}
