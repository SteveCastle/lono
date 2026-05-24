package cli

import "testing"

func TestEquipViaApplyAndGuard(t *testing.T) {
	dir := t.TempDir()
	def := `{"id":"g","version":1,
	  "itemTypes":{"silk_dress":{"category":"dress","equippable":true,"attributes":{"style":8}}},
	  "entityTypes":{"character":{"attributes":{},"slots":{"torso":{"accepts":["dress"]}}}},
	  "machines":{"arc":{"initial":"a","states":["a","b"],
	    "transitions":[{"id":"impress","from":"a","to":"b",
	      "guard":{"target":"worn.player.torso.style","op":"gte","value":5}}]}},
	  "setup":[{"op":"create_entity","entityType":"character","id":"player"}]}`
	if env, _ := runCLI(t, dir, "game", "import", "--spec", def); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "r1", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}
	// Before equipping, the worn-guarded action is unavailable (guard errors -> disabled).
	if env, _ := runCLI(t, dir, "do", "r1", "arc", "impress"); env.OK {
		t.Fatal("impress should fail before an outfit is worn")
	}
	// Equip via apply.
	env, _ := runCLI(t, dir, "apply", "r1", "--ops", `[{"op":"equip","entity":"player","slot":"torso","item":"silk_dress"}]`)
	if !env.OK {
		t.Fatalf("equip failed: %+v", env.Error)
	}
	player := env.Data.(map[string]any)["state"].(map[string]any)["entities"].(map[string]any)["player"].(map[string]any)
	if player["equipped"].(map[string]any)["torso"] != "silk_dress" {
		t.Fatalf("equipped not in output: %v", player["equipped"])
	}
	// Now the worn-guarded action succeeds.
	env, _ = runCLI(t, dir, "do", "r1", "arc", "impress")
	if !env.OK {
		t.Fatalf("impress should succeed once dressed: %+v", env.Error)
	}
}
