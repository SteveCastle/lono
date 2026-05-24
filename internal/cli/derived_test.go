package cli

import "testing"

func TestDefineDerivedAndOutput(t *testing.T) {
	dir := t.TempDir()
	// A game with a romance relationship and a global derived admirer count.
	def := `{"id":"g","version":1,
		"entityTypes":{"character":{"attributes":{}}},
		"relationshipTypes":{"romance":{"from":"character","to":"character",
		  "attributes":{"attraction":{"type":"int","default":0}}}},
		"machines":{"arc":{"initial":"a","states":["a"]}},
		"setup":[
		  {"op":"create_entity","entityType":"character","id":"player"},
		  {"op":"create_entity","entityType":"character","id":"aria"},
		  {"op":"set_relationship","relType":"romance","from":"aria","to":"player","attrs":{"attraction":90}}
		]}`
	if env, _ := runCLI(t, dir, "game", "import", "--spec", def); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
	env, _ := runCLI(t, dir, "define", "derived", "set", "g", "admirers", "--spec",
		`{"over":"relationships","where":{"type":"romance","to":"player","attrs":[{"attr":"attraction","op":"gte","value":80}]},"reduce":"count"}`)
	if !env.OK {
		t.Fatalf("define derived failed: %+v", env.Error)
	}
	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}
	env, _ = runCLI(t, dir, "state", "run1")
	if !env.OK {
		t.Fatalf("state failed: %+v", env.Error)
	}
	derived := env.Data.(map[string]any)["derived"].(map[string]any)
	global := derived["global"].(map[string]any)
	if global["admirers"].(float64) != 1 {
		t.Fatalf("expected admirers=1 in output, got %v", global["admirers"])
	}
}
