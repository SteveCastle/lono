package cli

import "testing"

func seedDatingGame(t *testing.T, dir string) {
	def := `{"id":"g","version":1,
	  "entityTypes":{"character":{"attributes":{}}},
	  "relationshipTypes":{"romance":{"from":"character","to":"character",
	    "attributes":{"affection":{"type":"int","default":0,"min":-100,"max":100}}}},
	  "machines":{
	    "arc":{"initial":"start","states":["start"]},
	    "romance_stage":{"attach":{"to":"relationshipType:romance"},
	      "initial":"friends","states":["friends","dating"],
	      "transitions":[{"id":"start_dating","from":"friends","to":"dating",
	        "guard":{"target":"this.affection","op":"gte","value":50},
	        "effects":[{"op":"inc","target":"this.affection","value":5}]}]}},
	  "setup":[
	    {"op":"create_entity","entityType":"character","id":"player"},
	    {"op":"create_entity","entityType":"character","id":"aria"},
	    {"op":"set_relationship","relType":"romance","from":"aria","to":"player","attrs":{"affection":60}}
	  ]}`
	if env, _ := runCLI(t, dir, "game", "import", "--spec", def); !env.OK {
		t.Fatalf("import failed: %+v", env.Error)
	}
}

func TestDoRelationshipAttachedMachine(t *testing.T) {
	dir := t.TempDir()
	seedDatingGame(t, dir)
	if env, _ := runCLI(t, dir, "play", "start", "g", "--id", "run1", "--seed", "1"); !env.OK {
		t.Fatalf("start failed: %+v", env.Error)
	}
	env, _ := runCLI(t, dir, "do", "run1", "romance_stage", "start_dating", "--rel", "aria,player")
	if !env.OK {
		t.Fatalf("attached do failed: %+v", env.Error)
	}
	rels := env.Data.(map[string]any)["state"].(map[string]any)["relationships"].([]any)
	r := rels[0].(map[string]any)
	machines := r["machines"].(map[string]any)
	if machines["romance_stage"] != "dating" {
		t.Fatalf("romance_stage did not advance: %v", machines)
	}
}
