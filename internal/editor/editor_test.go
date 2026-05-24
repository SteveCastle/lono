package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleDef is a tiny valid definition used across tests.
const sampleDef = `{
  "id": "t",
  "name": "Test",
  "version": 1,
  "machines": {
    "arc": {
      "initial": "start",
      "states": ["start", "end"],
      "stateMeta": {"end": {"description": "done", "terminal": true, "ending": true}},
      "transitions": [{"id": "finish", "from": "start", "to": "end"}]
    }
  }
}`

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	srv := httptest.NewServer(NewServer(dir).Handler())
	t.Cleanup(srv.Close)
	return srv, dir
}

// doJSON performs a request and decodes the JSON body, returning status + map.
func doJSON(t *testing.T, method, url string, body any) (int, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer res.Body.Close()
	var m map[string]any
	raw, _ := io.ReadAll(res.Body)
	if len(raw) > 0 {
		// Some endpoints return arrays; wrap so callers can still read status.
		if err := json.Unmarshal(raw, &m); err != nil {
			m = map[string]any{"_raw": string(raw)}
		}
	}
	return res.StatusCode, m
}

func TestMeta(t *testing.T) {
	srv, _ := newTestServer(t)
	status, m := doJSON(t, "GET", srv.URL+"/api/meta", nil)
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	if m["version"] == "" {
		t.Error("missing version")
	}
	ops, _ := m["effectOps"].([]any)
	if len(ops) < 20 {
		t.Errorf("expected the full op catalog, got %d", len(ops))
	}
	// Every advertised op must be one the engine actually understands. We assert a
	// representative spread is present.
	want := map[string]bool{"set": false, "roll": false, "if": false, "move": false, "discover": false, "schedule": false}
	for _, o := range ops {
		op := o.(map[string]any)["op"].(string)
		if _, ok := want[op]; ok {
			want[op] = true
		}
	}
	for op, found := range want {
		if !found {
			t.Errorf("op %q missing from meta", op)
		}
	}
}

func TestCreateListGetValidate(t *testing.T) {
	srv, dir := newTestServer(t)

	// create from template
	status, _ := doJSON(t, "POST", srv.URL+"/api/games", map[string]any{"id": "mygame"})
	if status != 200 {
		t.Fatalf("create status %d", status)
	}
	if _, err := os.Stat(filepath.Join(dir, "mygame.lono.json")); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	// duplicate create rejected
	if status, _ := doJSON(t, "POST", srv.URL+"/api/games", map[string]any{"id": "mygame"}); status != http.StatusConflict {
		t.Errorf("duplicate create: want 409, got %d", status)
	}

	// list shows it as valid
	res, err := http.Get(srv.URL + "/api/games")
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if len(list) != 1 || list[0]["file"] != "mygame.lono.json" || list[0]["valid"] != true {
		t.Errorf("unexpected list: %v", list)
	}

	// get returns the definition + empty validation
	status, m := doJSON(t, "GET", srv.URL+"/api/games/mygame.lono.json", nil)
	if status != 200 {
		t.Fatalf("get status %d", status)
	}
	if v, _ := m["validation"].([]any); len(v) != 0 {
		t.Errorf("template should be valid, got %v", v)
	}
}

func TestSaveAndValidate(t *testing.T) {
	srv, dir := newTestServer(t)
	doJSON(t, "POST", srv.URL+"/api/games", map[string]any{"id": "g", "definition": json.RawMessage(sampleDef)})

	// save a definition missing its id -> persisted, but reported invalid
	bad := map[string]any{"name": "no id", "version": 1}
	status, m := doJSON(t, "PUT", srv.URL+"/api/games/g.lono.json", map[string]any{"definition": bad})
	if status != 200 {
		t.Fatalf("save status %d", status)
	}
	if v, _ := m["validation"].([]any); len(v) == 0 {
		t.Error("expected validation errors for a definition with no id")
	}
	// the file is still written (WIP saves)
	b, err := os.ReadFile(filepath.Join(dir, "g.lono.json"))
	if err != nil || !strings.Contains(string(b), "no id") {
		t.Errorf("WIP not persisted: %v / %s", err, b)
	}

	// validate endpoint (no save) on the good sample
	status, m = doJSON(t, "POST", srv.URL+"/api/validate", map[string]any{"definition": json.RawMessage(sampleDef)})
	if status != 200 {
		t.Fatalf("validate status %d", status)
	}
	if v, _ := m["validation"].([]any); len(v) != 0 {
		t.Errorf("sample should validate clean, got %v", v)
	}
}

// The Map/Scenes UI stashes editor-only data under a top-level "_editor" key.
// The engine must ignore it: it survives a save round-trip and never affects
// validation or playtest.
func TestEditorMetadataPassthrough(t *testing.T) {
	srv, dir := newTestServer(t)
	doJSON(t, "POST", srv.URL+"/api/games", map[string]any{"id": "g"})

	withMeta := map[string]any{
		"id": "g", "name": "G", "version": 1,
		"machines": map[string]any{
			"arc": map[string]any{
				"initial": "start", "states": []string{"start", "end"},
				"stateMeta":   map[string]any{"end": map[string]any{"terminal": true, "ending": true}},
				"transitions": []any{map[string]any{"id": "finish", "from": "start", "to": "end"}},
			},
		},
		"_editor": map[string]any{
			"map":    map[string]any{"placeType": "location", "positions": map[string]any{"a": map[string]any{"x": 1, "y": 2}}},
			"scenes": map[string]any{"s1": map[string]any{"name": "opening"}},
		},
	}
	if status, m := doJSON(t, "PUT", srv.URL+"/api/games/g.lono.json", map[string]any{"definition": withMeta}); status != 200 {
		t.Fatalf("save status %d", status)
	} else if v, _ := m["validation"].([]any); len(v) != 0 {
		t.Errorf("definition with _editor should validate clean, got %v", v)
	}

	// the saved file still carries _editor verbatim
	b, _ := os.ReadFile(filepath.Join(dir, "g.lono.json"))
	if !strings.Contains(string(b), "_editor") || !strings.Contains(string(b), "opening") {
		t.Errorf("_editor not persisted: %s", b)
	}

	// and it plays without complaint
	if status, _ := doJSON(t, "POST", srv.URL+"/api/playtest", map[string]any{"definition": withMeta}); status != 200 {
		t.Errorf("playtest with _editor: want 200, got %d", status)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, bad := range []string{"..%2f..%2fgo.mod", "evil.txt", "sub%2fx.lono.json"} {
		status, _ := doJSON(t, "GET", srv.URL+"/api/games/"+bad, nil)
		if status != http.StatusBadRequest && status != http.StatusNotFound {
			t.Errorf("path %q: expected rejection, got %d", bad, status)
		}
	}
}

func TestPlaytestFlow(t *testing.T) {
	srv, _ := newTestServer(t)

	// invalid definition cannot be playtested
	if status, _ := doJSON(t, "POST", srv.URL+"/api/playtest", map[string]any{"definition": map[string]any{"name": "x"}}); status != http.StatusUnprocessableEntity {
		t.Errorf("invalid playtest: want 422, got %d", status)
	}

	// start a session
	status, m := doJSON(t, "POST", srv.URL+"/api/playtest", map[string]any{"definition": json.RawMessage(sampleDef), "seed": 1})
	if status != 200 {
		t.Fatalf("playtest start status %d (%v)", status, m)
	}
	sess, _ := m["session"].(string)
	if sess == "" {
		t.Fatal("no session id")
	}

	// act: finish -> reaches the "end" ending
	status, m = doJSON(t, "POST", srv.URL+"/api/playtest/"+sess+"/act", map[string]any{"machine": "arc", "action": "finish"})
	if status != 200 {
		t.Fatalf("act status %d (%v)", status, m)
	}
	view := m["view"].(map[string]any)
	endings, _ := view["endingReached"].([]any)
	if len(endings) != 1 || endings[0].(map[string]any)["state"] != "end" {
		t.Errorf("expected the 'end' ending, got %v", endings)
	}

	// an illegal action is rejected
	if status, _ := doJSON(t, "POST", srv.URL+"/api/playtest/"+sess+"/act", map[string]any{"machine": "arc", "action": "finish"}); status != http.StatusUnprocessableEntity {
		t.Errorf("re-finish from terminal: want 422, got %d", status)
	}

	// unknown session is a 404
	if status, _ := doJSON(t, "POST", srv.URL+"/api/playtest/nope/advance", map[string]any{"n": 1}); status != http.StatusNotFound {
		t.Errorf("unknown session: want 404, got %d", status)
	}
}
