package engine

import (
	"encoding/json"
	"testing"
)

// navDef builds a small Definition exercising the key structural shapes.
func navDef() *Definition {
	return &Definition{
		ID:      "nav",
		Name:    "Nav Test Game",
		Version: 1,
		World: map[string]VarSpec{
			"day":   {Type: "int", Default: float64(1)},
			"alarm": {Type: "bool", Default: false},
		},
		EntityTypes: map[string]EntityType{
			"character": {
				Attributes: map[string]VarSpec{
					"health": {Type: "int", Default: float64(100)},
				},
			},
		},
		RelationshipTypes: map[string]RelType{
			"trust": {From: "character", To: "character", Directed: true,
				Attributes: map[string]VarSpec{"value": {Type: "int", Default: float64(0)}}},
		},
		Machines: map[string]Machine{
			"arc": {
				Initial: "intro",
				States:  []string{"intro", "end"},
				Transitions: []Transition{
					{ID: "finish", From: StateSet{"intro"}, To: "end"},
				},
			},
		},
	}
}

// navState builds a small State with entities and a relationship.
func navState() *State {
	st, _ := NewInstance(navDef(), "run1", 42)
	st.Entities["aria"] = &Entity{
		Type:      "character",
		Attrs:     map[string]any{"health": float64(80)},
		Inventory: map[string]int{},
	}
	st.Entities["player"] = &Entity{
		Type:      "character",
		Attrs:     map[string]any{"health": float64(100)},
		Inventory: map[string]int{},
	}
	st.Relationships = []*Relationship{{
		Type:  "trust",
		From:  "aria",
		To:    "player",
		Attrs: map[string]any{"affection": float64(42)},
	}}
	st.Machines["arc"] = "intro"
	return st
}

// --- GetNode ---

func TestGetNodeScalarPath(t *testing.T) {
	root := map[string]any{"foo": "bar", "nested": map[string]any{"x": float64(1)}}

	v, err := GetNode(root, "foo")
	if err != nil || v != "bar" {
		t.Fatalf("foo: got %v, err %v", v, err)
	}

	v, err = GetNode(root, "nested/x")
	if err != nil || v != float64(1) {
		t.Fatalf("nested/x: got %v, err %v", v, err)
	}
}

func TestGetNodeEmptyPathReturnsRoot(t *testing.T) {
	root := map[string]any{"k": "v"}
	v, err := GetNode(root, "")
	if err != nil || v == nil {
		t.Fatalf("empty path should return root, got %v, %v", v, err)
	}
}

func TestGetNodeIDArray(t *testing.T) {
	arr := []any{
		map[string]any{"id": "alpha", "val": float64(1)},
		map[string]any{"id": "beta", "val": float64(2)},
	}
	root := map[string]any{"items": arr}

	v, err := GetNode(root, "items/beta")
	if err != nil {
		t.Fatalf("items/beta: %v", err)
	}
	m := v.(map[string]any)
	if m["val"] != float64(2) {
		t.Fatalf("wrong element: %v", m)
	}

	// Missing id → errNoSuchPath
	_, err = GetNode(root, "items/gamma")
	if !IsNoSuchPath(err) {
		t.Fatalf("expected errNoSuchPath, got %v", err)
	}
}

func TestGetNodeNumericIndex(t *testing.T) {
	arr := []any{"a", "b", "c"}
	root := map[string]any{"list": arr}

	v, err := GetNode(root, "list/1")
	if err != nil || v != "b" {
		t.Fatalf("list/1: got %v, %v", v, err)
	}

	_, err = GetNode(root, "list/99")
	if !IsNoSuchPath(err) {
		t.Fatalf("out-of-range index should be errNoSuchPath, got %v", err)
	}
}

func TestGetNodeMissingKey(t *testing.T) {
	root := map[string]any{"a": "A"}
	_, err := GetNode(root, "bogus")
	if !IsNoSuchPath(err) {
		t.Fatalf("missing key should be errNoSuchPath, got %v", err)
	}
}

func TestGetNodeScalarWithRemainingSegments(t *testing.T) {
	root := map[string]any{"name": "hello"}
	_, err := GetNode(root, "name/sub")
	if !IsNoSuchPath(err) {
		t.Fatalf("descend into scalar should be errNoSuchPath, got %v", err)
	}
}

// --- GetDefNode ---

func TestGetDefNodeName(t *testing.T) {
	def := navDef()
	v, err := GetDefNode(def, "name")
	if err != nil || v != "Nav Test Game" {
		t.Fatalf("name: got %v, %v", v, err)
	}
}

func TestGetDefNodeWorldVar(t *testing.T) {
	def := navDef()
	v, err := GetDefNode(def, "world/day")
	if err != nil || v == nil {
		t.Fatalf("world/day: got %v, %v", v, err)
	}
}

func TestGetDefNodeMachine(t *testing.T) {
	def := navDef()
	v, err := GetDefNode(def, "machines/arc")
	if err != nil {
		t.Fatalf("machines/arc: %v", err)
	}
	m := v.(map[string]any)
	if m["initial"] != "intro" {
		t.Fatalf("wrong machine: %v", m)
	}
}

func TestGetDefNodeTransitionByID(t *testing.T) {
	def := navDef()
	v, err := GetDefNode(def, "machines/arc/transitions/finish")
	if err != nil {
		t.Fatalf("machines/arc/transitions/finish: %v", err)
	}
	tr := v.(map[string]any)
	if tr["id"] != "finish" {
		t.Fatalf("wrong transition: %v", tr)
	}
	if tr["to"] != "end" {
		t.Fatalf("wrong to: %v", tr)
	}
}

func TestGetDefNodeEntityTypeAttribute(t *testing.T) {
	def := navDef()
	v, err := GetDefNode(def, "entityTypes/character/attributes/health")
	if err != nil {
		t.Fatalf("entityTypes/character/attributes/health: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil health spec")
	}
}

func TestGetDefNodeMissingPath(t *testing.T) {
	def := navDef()
	_, err := GetDefNode(def, "machines/arc/transitions/nope")
	if !IsNoSuchPath(err) {
		t.Fatalf("missing transition should be errNoSuchPath, got %v", err)
	}

	_, err = GetDefNode(def, "bogus/path")
	if !IsNoSuchPath(err) {
		t.Fatalf("bogus path should be errNoSuchPath, got %v", err)
	}
}

// --- TreeView ---

func TestTreeViewScalar(t *testing.T) {
	if v := TreeView("hello", 2); v != nil {
		t.Fatalf("scalar should yield nil, got %v", v)
	}
}

func TestTreeViewIDArray(t *testing.T) {
	arr := []any{
		map[string]any{"id": "a"},
		map[string]any{"id": "b"},
	}
	v := TreeView(arr, 2)
	ids, ok := v.([]string)
	if !ok || len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("id-array TreeView wrong: %v", v)
	}
}

func TestTreeViewPlainArray(t *testing.T) {
	arr := []any{"x", "y", "z"}
	v := TreeView(arr, 2)
	if v != 3 {
		t.Fatalf("plain array TreeView should return length 3, got %v", v)
	}
}

func TestTreeViewMapDepthOne(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{"x": 1},
		"b": "scalar",
	}
	v := TreeView(m, 1)
	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("depth-1 map should return map, got %T", v)
	}
	// At depth 1, children are at depth 0: map → sorted keys list, scalar → nil.
	if _, ok := out["a"]; !ok {
		t.Fatal("key 'a' missing from depth-1 tree")
	}
	if _, ok := out["b"]; !ok {
		t.Fatal("key 'b' missing from depth-1 tree")
	}
}

func TestTreeViewMapDepthZero(t *testing.T) {
	m := map[string]any{"z": 1, "a": 2, "m": 3}
	v := TreeView(m, 0)
	keys, ok := v.([]string)
	if !ok {
		t.Fatalf("depth-0 map should return sorted keys, got %T: %v", v, v)
	}
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "m" || keys[2] != "z" {
		t.Fatalf("keys not sorted: %v", keys)
	}
}

func TestTreeViewDefDepth2(t *testing.T) {
	def := navDef()
	defMap, err := marshalToMap(def)
	if err != nil {
		t.Fatal(err)
	}
	v := TreeView(defMap, 2)
	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("def tree should be map, got %T", v)
	}
	// At depth 2 we should see top-level keys and their children's keys.
	if _, ok := out["machines"]; !ok {
		t.Fatal("machines key missing from depth-2 tree")
	}
}

// marshalToMap is a test helper that round-trips through JSON.
func marshalToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// --- GetStateNode ---

func TestGetStateNodeEntityAttr(t *testing.T) {
	def := navDef()
	st := navState()
	v, err := GetStateNode(def, st, "entities/aria/attrs/health")
	if err != nil {
		t.Fatalf("entities/aria/attrs/health: %v", err)
	}
	if v != float64(80) {
		t.Fatalf("expected 80, got %v", v)
	}
}

func TestGetStateNodeRelationshipAttr(t *testing.T) {
	def := navDef()
	st := navState()
	v, err := GetStateNode(def, st, "relationships/trust/aria/player/attrs/affection")
	if err != nil {
		t.Fatalf("relationships/trust/aria/player/attrs/affection: %v", err)
	}
	if v != float64(42) {
		t.Fatalf("expected 42, got %v", v)
	}
}

func TestGetStateNodeRelationshipMissing(t *testing.T) {
	def := navDef()
	st := navState()
	_, err := GetStateNode(def, st, "relationships/trust/aria/nobody/attrs/affection")
	if !IsNoSuchPath(err) {
		t.Fatalf("missing relationship should be errNoSuchPath, got %v", err)
	}
}

func TestGetStateNodeRelationshipRoot(t *testing.T) {
	def := navDef()
	st := navState()
	// Exactly 4 segments: returns the whole relationship element.
	v, err := GetStateNode(def, st, "relationships/trust/aria/player")
	if err != nil {
		t.Fatalf("relationships/trust/aria/player: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil relationship")
	}
}

func TestGetStateNodeMissingEntity(t *testing.T) {
	def := navDef()
	st := navState()
	_, err := GetStateNode(def, st, "entities/ghost/attrs/health")
	if !IsNoSuchPath(err) {
		t.Fatalf("missing entity should be errNoSuchPath, got %v", err)
	}
}

// --- GetStateNode computed views ---

// navDefWithDerived builds a def with a derived value, a beat, and a machine
// for testing runtime view paths.
func navDefWithDerived() *Definition {
	def := navDef()
	def.Derived = map[string]DerivedSpec{
		"totalHealth": {
			Over:   "entities",
			Reduce: "sum:health",
			Where:  WhereSpec{Type: "character"},
		},
	}
	def.Beats = map[string]Beat{
		"intro_beat": {
			MachineState: &MachineStateRef{Machine: "arc", State: "intro"},
			Text:         "Welcome!",
		},
	}
	return def
}

func TestGetStateNodeDerived(t *testing.T) {
	def := navDefWithDerived()
	st := navState()
	v, err := GetStateNode(def, st, "derived")
	if err != nil {
		t.Fatalf("derived: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil derived view")
	}
	// Should be a map with "global" and/or "byEntity".
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map for derived, got %T", v)
	}
	if _, ok := m["global"]; !ok {
		t.Fatal("expected 'global' key in derived view")
	}
}

func TestGetStateNodeDerivedSubPath(t *testing.T) {
	def := navDefWithDerived()
	st := navState()
	v, err := GetStateNode(def, st, "derived/global/totalHealth")
	if err != nil {
		t.Fatalf("derived/global/totalHealth: %v", err)
	}
	// aria=80 + player=100 = 180
	if v != float64(180) {
		t.Fatalf("expected totalHealth=180, got %v", v)
	}
}

func TestGetStateNodeBeats(t *testing.T) {
	def := navDefWithDerived()
	st := navState()
	v, err := GetStateNode(def, st, "beats")
	if err != nil {
		t.Fatalf("beats: %v", err)
	}
	// Should be a slice (JSON array).
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any for beats, got %T", v)
	}
	// intro_beat should be active since machine arc is in "intro".
	if len(arr) == 0 {
		t.Fatal("expected at least one active beat")
	}
}

func TestGetStateNodeActions(t *testing.T) {
	def := navDefWithDerived()
	st := navState()
	v, err := GetStateNode(def, st, "actions")
	if err != nil {
		t.Fatalf("actions: %v", err)
	}
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any for actions, got %T", v)
	}
	// The "finish" transition should be available from "intro".
	if len(arr) == 0 {
		t.Fatal("expected at least one available action")
	}
}

func TestGetStateNodeEndingReached(t *testing.T) {
	def := navDefWithDerived()
	st := navState()
	v, err := GetStateNode(def, st, "endingReached")
	if err != nil {
		t.Fatalf("endingReached: %v", err)
	}
	// No terminal state reached yet; should be an empty slice.
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any for endingReached, got %T", v)
	}
	_ = arr // may be empty
}
