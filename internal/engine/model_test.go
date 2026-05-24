package engine

import (
	"encoding/json"
	"testing"
)

func TestDefinitionRoundTrip(t *testing.T) {
	src := `{
	  "id": "g1", "name": "Game One", "version": 1,
	  "world": {"day": {"type": "int", "default": 1, "min": 1}},
	  "entityTypes": {"character": {"attributes": {"health": {"type": "int", "default": 100}}}},
	  "itemTypes": {"gold": {"maxStack": 1000}},
	  "relationshipTypes": {"trust": {"from": "character", "to": "character", "directed": true,
	    "attributes": {"value": {"type": "int", "default": 0}}}},
	  "machines": {"arc": {"initial": "intro", "states": ["intro", "end"],
	    "transitions": [{"id": "go", "from": "intro", "to": "end",
	      "effects": [{"op": "set", "target": "world.day", "value": 2}]}]}},
	  "setup": [{"op": "create_entity", "entityType": "character", "id": "player"}]
	}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if def.ID != "g1" || def.Version != 1 {
		t.Fatalf("bad header: %+v", def)
	}
	tr := def.Machines["arc"].Transitions[0]
	if !tr.From.Matches("intro") || tr.From.Matches("other") {
		t.Fatalf("StateSet.Matches broken: %v", tr.From)
	}
	if tr.To != "end" || tr.Effects[0].Op != "set" {
		t.Fatalf("bad transition: %+v", tr)
	}
}

func TestStateSetWildcardAndList(t *testing.T) {
	var s StateSet
	if err := json.Unmarshal([]byte(`"*"`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Matches("anything") {
		t.Fatal("wildcard should match")
	}
	if err := json.Unmarshal([]byte(`["a","b"]`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Matches("b") || s.Matches("c") {
		t.Fatal("list match broken")
	}
}

func TestAttachSpecRoundTrip(t *testing.T) {
	src := `{"id":"g","version":1,"machines":{
	  "romance_stage":{"attach":{"to":"relationshipType:romance"},
	    "initial":"strangers","states":["strangers","dating"],
	    "transitions":[{"id":"date","from":"strangers","to":"dating",
	      "guard":{"target":"this.affection","op":"gte","value":60}}]}}}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatal(err)
	}
	m := def.Machines["romance_stage"]
	if m.Attach == nil || m.Attach.To != "relationshipType:romance" {
		t.Fatalf("attach not parsed: %+v", m.Attach)
	}
}

func TestBeatRoundTrip(t *testing.T) {
	src := `{"id":"g","version":1,"beats":{
	  "aria_smile":{"text":"Aria smiles.","machineState":{"machine":"arc","state":"bar"},
	    "guard":{"target":"rel.romance.aria.player.affection","op":"gte","value":20},
	    "once":true,"intent":"first warmth"},
	  "rain":{"text":"It rains."}}}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatal(err)
	}
	b := def.Beats["aria_smile"]
	if b.Text != "Aria smiles." || b.MachineState == nil || b.MachineState.Machine != "arc" || b.MachineState.State != "bar" {
		t.Fatalf("beat: %+v", b)
	}
	if b.Guard == nil || b.Once == nil || *b.Once != true {
		t.Fatalf("beat guard/once: %+v", b)
	}
	if def.Beats["rain"].Once != nil {
		t.Fatalf("unset once should be nil: %+v", def.Beats["rain"])
	}
}

func TestNarrativeMetaRoundTrip(t *testing.T) {
	src := `{"id":"g","version":1,"description":"A heist story","intent":"noir tone",
	  "entityTypes":{"character":{"description":"a person","attributes":{}}},
	  "machines":{"arc":{"initial":"intro","states":["intro","ending_good"],
	    "stateMeta":{"ending_good":{"terminal":true,"ending":true,"description":"You win.","intent":"player kept the trust"}},
	    "transitions":[{"id":"go","from":"intro","to":"ending_good","description":"the finale","intent":"only if loot taken"}]}}}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatal(err)
	}
	if def.Description != "A heist story" || def.Intent != "noir tone" {
		t.Fatalf("game desc/intent: %+v", def)
	}
	if def.EntityTypes["character"].Description != "a person" {
		t.Fatalf("entity desc: %+v", def.EntityTypes["character"])
	}
	m := def.Machines["arc"]
	meta, ok := m.StateMeta["ending_good"]
	if !ok || !meta.Terminal || !meta.Ending || meta.Description != "You win." {
		t.Fatalf("stateMeta: %+v", m.StateMeta)
	}
	if m.Transitions[0].Description != "the finale" || m.Transitions[0].Intent != "only if loot taken" {
		t.Fatalf("transition desc/intent: %+v", m.Transitions[0])
	}
}

func TestDerivedSpecRoundTrip(t *testing.T) {
	src := `{
	  "id":"g","version":1,
	  "derived":{
	    "admirers":{"over":"relationships","where":{"type":"romance","to":"player",
	      "attrs":[{"attr":"attraction","op":"gte","value":80}]},"reduce":"count","intent":"strong admirers"},
	    "top_admirer":{"over":"relationships","where":{"type":"romance","to":"player"},"reduce":"argmax:attraction"},
	    "my_friends":{"over":"relationships","where":{"type":"friendship","from":"$self"},"reduce":"count"}
	  }
	}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	d := def.Derived["admirers"]
	if d.Over != "relationships" || d.Reduce != "count" || d.Where.To != "player" {
		t.Fatalf("bad derived: %+v", d)
	}
	if len(d.Where.Attrs) != 1 || d.Where.Attrs[0].Attr != "attraction" || d.Where.Attrs[0].Op != "gte" {
		t.Fatalf("bad attr pred: %+v", d.Where.Attrs)
	}
	if def.Derived["my_friends"].Where.From != "$self" {
		t.Fatalf("bad $self: %+v", def.Derived["my_friends"])
	}
}

func TestCastModelRoundTrip(t *testing.T) {
	src := `{
	  "id": "g1", "version": 1,
	  "entityTypes": {
	    "character": {"attributes": {"charm": {"type": "int", "default": 0}}}
	  },
	  "itemTypes": {
	    "sword": {"category": "weapon", "equippable": true, "maxStack": 1}
	  },
	  "relationshipTypes": {
	    "friendship": {"from": "character", "to": "character",
	      "attributes": {"closeness": {"type": "int", "default": 0}}}
	  },
	  "entities": {
	    "aria": {"type": "character", "attrs": {"charm": 8},
	             "inventory": {"sword": 1}, "equipped": {"hand": "sword"}},
	    "player": {"type": "character"}
	  },
	  "relationships": [
	    {"type": "friendship", "from": "aria", "to": "player", "attrs": {"closeness": 50}}
	  ]
	}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Verify entities.
	aria, ok := def.Entities["aria"]
	if !ok {
		t.Fatal("aria not in entities")
	}
	if aria.Type != "character" {
		t.Fatalf("aria.Type: %v", aria.Type)
	}
	if aria.Attrs["charm"] != float64(8) {
		t.Fatalf("aria.Attrs.charm: %v", aria.Attrs["charm"])
	}
	if aria.Inventory["sword"] != 1 {
		t.Fatalf("aria.Inventory.sword: %v", aria.Inventory["sword"])
	}
	if aria.Equipped["hand"] != "sword" {
		t.Fatalf("aria.Equipped.hand: %v", aria.Equipped["hand"])
	}
	if def.Entities["player"].Type != "character" {
		t.Fatalf("player.Type: %v", def.Entities["player"].Type)
	}
	// Verify relationships.
	if len(def.Relationships) != 1 {
		t.Fatalf("expected 1 relationship, got %d", len(def.Relationships))
	}
	rel := def.Relationships[0]
	if rel.Type != "friendship" || rel.From != "aria" || rel.To != "player" {
		t.Fatalf("rel header: %+v", rel)
	}
	if rel.Attrs["closeness"] != float64(50) {
		t.Fatalf("rel.Attrs.closeness: %v", rel.Attrs["closeness"])
	}
	// Round-trip: marshal and unmarshal again.
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var def2 Definition
	if err := json.Unmarshal(b, &def2); err != nil {
		t.Fatalf("unmarshal2: %v", err)
	}
	if def2.Entities["aria"].Inventory["sword"] != 1 {
		t.Fatalf("round-trip inventory: %v", def2.Entities["aria"].Inventory["sword"])
	}
	if len(def2.Relationships) != 1 || def2.Relationships[0].Attrs["closeness"] != float64(50) {
		t.Fatalf("round-trip relationships: %+v", def2.Relationships)
	}
}

func TestEquipmentModelRoundTrip(t *testing.T) {
	src := `{"id":"g","version":1,
	  "itemTypes":{"silk_dress":{"category":"dress","equippable":true,
	    "attributes":{"style":8,"warmth":2,"color":"midnight-blue"},"description":"A silk dress."}},
	  "entityTypes":{"character":{"attributes":{},
	    "slots":{"torso":{"accepts":["dress","top"],"description":"worn on the torso"}}}}}`
	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatal(err)
	}
	it := def.ItemTypes["silk_dress"]
	if it.Category != "dress" || !it.Equippable || it.Attributes["style"] != float64(8) {
		t.Fatalf("item type: %+v", it)
	}
	slot := def.EntityTypes["character"].Slots["torso"]
	if len(slot.Accepts) != 2 || slot.Accepts[0] != "dress" {
		t.Fatalf("slot: %+v", slot)
	}
}
