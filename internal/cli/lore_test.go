package cli

import (
	"encoding/json"
	"strings"
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

// TestDiscoverViaApply verifies that the discover op works via `apply` and
// that discoveredLore appears in the output.
func TestDiscoverViaApply(t *testing.T) {
	dir := t.TempDir()

	// Create game with a lore entry and a machine to hang a play session on.
	runCLI(t, dir, "game", "create", "g")
	runCLI(t, dir, "define", "lore", "set", "g", "founding",
		"--spec", `{"title":"The Founding","text":"Built in Year 312."}`)
	runCLI(t, dir, "define", "machine", "set", "g", "arc",
		"--spec", `{"initial":"start","states":["start"]}`)

	// Start a play session.
	env, _ := runCLI(t, dir, "play", "start", "g", "--id", "r1", "--seed", "1")
	if !env.OK {
		t.Fatalf("play start failed: %+v", env.Error)
	}

	// Apply discover.
	env2, _ := runCLI(t, dir, "apply", "r1",
		"--ops", `[{"op":"discover","lore":"founding"}]`)
	if !env2.OK {
		t.Fatalf("apply discover failed: %+v", env2.Error)
	}

	// discoveredLore should appear at top-level in the response data.
	b, _ := json.Marshal(env2.Data)
	var data map[string]any
	_ = json.Unmarshal(b, &data)
	dl, ok := data["discoveredLore"]
	if !ok {
		t.Fatal("discoveredLore missing from apply response")
	}
	arr, ok := dl.([]any)
	if !ok || len(arr) != 1 || arr[0] != "founding" {
		t.Fatalf("discoveredLore = %v, want [founding]", dl)
	}

	// Apply discover again (idempotent) — should still be 1 entry.
	env3, _ := runCLI(t, dir, "apply", "r1",
		"--ops", `[{"op":"discover","lore":"founding"}]`)
	if !env3.OK {
		t.Fatalf("apply discover (idempotent) failed: %+v", env3.Error)
	}
	b3, _ := json.Marshal(env3.Data)
	var data3 map[string]any
	_ = json.Unmarshal(b3, &data3)
	arr3 := data3["discoveredLore"].([]any)
	if len(arr3) != 1 {
		t.Fatalf("after idempotent discover, discoveredLore = %v, want [founding]", arr3)
	}

	// Apply discover with unknown lore id — should fail.
	env4, _ := runCLI(t, dir, "apply", "r1",
		"--ops", `[{"op":"discover","lore":"no_such"}]`)
	if env4.OK {
		t.Fatal("expected failure for unknown lore id")
	}
	if !strings.Contains(env4.Error.Message, "no_such") {
		t.Fatalf("error should mention id, got: %v", env4.Error.Message)
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

// TestLoreListAndShow exercises the lore list and lore show CLI commands.
func TestLoreListAndShow(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "g")

	// Define two lore entries: one tagged, one with a subject.
	runCLI(t, dir, "define", "lore", "set", "g", "founding",
		"--spec", `{"title":"The Founding","text":"Built in Year 312.","tags":["history","manor"],"when":"Year 312"}`)
	runCLI(t, dir, "define", "lore", "set", "g", "locket",
		"--spec", `{"title":"The Locket","text":"Silver locket.","subject":"player"}`)

	// lore list — shows both entries.
	env, _ := runCLI(t, dir, "lore", "list", "g")
	if !env.OK {
		t.Fatalf("lore list failed: %+v", env.Error)
	}
	b, _ := json.Marshal(env.Data)
	var listData map[string]any
	_ = json.Unmarshal(b, &listData)
	items, ok := listData["lore"].([]any)
	if !ok {
		t.Fatalf("lore key missing or wrong type: %v", listData)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 lore items, got %d: %v", len(items), items)
	}
	// Items are sorted by id: founding < locket.
	first := items[0].(map[string]any)
	if first["id"] != "founding" {
		t.Errorf("first item id = %v, want founding", first["id"])
	}
	second := items[1].(map[string]any)
	if second["id"] != "locket" {
		t.Errorf("second item id = %v, want locket", second["id"])
	}

	// lore list --tag history — only founding.
	envTag, _ := runCLI(t, dir, "lore", "list", "g", "--tag", "history")
	if !envTag.OK {
		t.Fatalf("lore list --tag failed: %+v", envTag.Error)
	}
	bTag, _ := json.Marshal(envTag.Data)
	var tagData map[string]any
	_ = json.Unmarshal(bTag, &tagData)
	tagItems := tagData["lore"].([]any)
	if len(tagItems) != 1 {
		t.Fatalf("expected 1 tagged item, got %d", len(tagItems))
	}
	if tagItems[0].(map[string]any)["id"] != "founding" {
		t.Errorf("tagged item id = %v, want founding", tagItems[0].(map[string]any)["id"])
	}

	// lore list --subject player — only locket.
	envSub, _ := runCLI(t, dir, "lore", "list", "g", "--subject", "player")
	if !envSub.OK {
		t.Fatalf("lore list --subject failed: %+v", envSub.Error)
	}
	bSub, _ := json.Marshal(envSub.Data)
	var subData map[string]any
	_ = json.Unmarshal(bSub, &subData)
	subItems := subData["lore"].([]any)
	if len(subItems) != 1 {
		t.Fatalf("expected 1 subject item, got %d", len(subItems))
	}
	if subItems[0].(map[string]any)["id"] != "locket" {
		t.Errorf("subject item id = %v, want locket", subItems[0].(map[string]any)["id"])
	}

	// lore show founding — full entry including text.
	envShow, _ := runCLI(t, dir, "lore", "show", "g", "founding")
	if !envShow.OK {
		t.Fatalf("lore show founding failed: %+v", envShow.Error)
	}
	bShow, _ := json.Marshal(envShow.Data)
	var showData map[string]any
	_ = json.Unmarshal(bShow, &showData)
	entry, ok := showData["entry"].(map[string]any)
	if !ok {
		t.Fatalf("entry missing from lore show response: %v", showData)
	}
	if entry["text"] != "Built in Year 312." {
		t.Errorf("text: got %v", entry["text"])
	}
	if entry["title"] != "The Founding" {
		t.Errorf("title: got %v", entry["title"])
	}
	if showData["id"] != "founding" {
		t.Errorf("id: got %v", showData["id"])
	}

	// lore show missing id — NOT_FOUND.
	envMiss, _ := runCLI(t, dir, "lore", "show", "g", "no_such")
	if envMiss.OK {
		t.Fatal("expected NOT_FOUND for unknown lore id")
	}
	if envMiss.Error == nil || envMiss.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND code, got: %+v", envMiss.Error)
	}
}
