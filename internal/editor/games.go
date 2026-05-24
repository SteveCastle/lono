package editor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/callsignmedia/lono/internal/engine"
)

// gameSummary is one row in the editor's file list.
type gameSummary struct {
	File       string `json:"file"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Valid      bool   `json:"valid"`
	ErrorCount int    `json:"errorCount"`
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read dir: %v", err)
		return
	}
	out := []gameSummary{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lono.json") {
			continue
		}
		sum := gameSummary{File: e.Name()}
		if b, err := os.ReadFile(filepath.Join(s.dir, e.Name())); err == nil {
			var def engine.Definition
			if json.Unmarshal(b, &def) == nil {
				sum.ID = def.ID
				sum.Name = def.Name
				errs := engine.ValidateDefinition(&def)
				sum.Valid = len(errs) == 0
				sum.ErrorCount = len(errs)
			}
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	writeJSON(w, http.StatusOK, out)
}

// gamePayload is the response for reading/saving a single file.
type gamePayload struct {
	File       string                   `json:"file"`
	Definition json.RawMessage          `json:"definition"`
	Validation []engine.ValidationError `json:"validation"`
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	path, err := s.safePath(r.PathValue("file"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, gamePayload{
		File:       filepath.Base(path),
		Definition: json.RawMessage(b),
		Validation: validateBytes(b),
	})
}

// saveRequest carries a definition object to write.
type saveRequest struct {
	Definition json.RawMessage `json:"definition"`
}

func (s *Server) handleSaveGame(w http.ResponseWriter, r *http.Request) {
	path, err := s.safePath(r.PathValue("file"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	var req saveRequest
	if err := decodeBody(r, &req); err != nil || len(req.Definition) == 0 {
		writeErr(w, http.StatusBadRequest, "body must be {\"definition\": {...}}")
		return
	}
	pretty, err := prettyJSON(req.Definition)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "definition is not valid JSON: %v", err)
		return
	}
	// Save WIP even when invalid — but report problems so the UI can surface them.
	if err := os.WriteFile(path, pretty, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "write: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"saved":      true,
		"file":       filepath.Base(path),
		"validation": validateBytes(pretty),
	})
}

// createRequest creates a new file, optionally seeded with a definition.
type createRequest struct {
	File       string          `json:"file"`
	ID         string          `json:"id"`
	Definition json.RawMessage `json:"definition,omitempty"`
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body: %v", err)
		return
	}
	file := req.File
	if file == "" && req.ID != "" {
		file = req.ID + ".lono.json"
	}
	path, err := s.safePath(file)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	if _, err := os.Stat(path); err == nil {
		writeErr(w, http.StatusConflict, "file %q already exists", filepath.Base(path))
		return
	}
	var body []byte
	if len(req.Definition) > 0 {
		body, err = prettyJSON(req.Definition)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "definition is not valid JSON: %v", err)
			return
		}
	} else {
		id := req.ID
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), ".lono.json")
		}
		body, _ = json.MarshalIndent(starterTemplate(id), "", "  ")
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "write: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": true, "file": filepath.Base(path)})
}

func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	path, err := s.safePath(r.PathValue("file"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := os.Remove(path); err != nil {
		writeErr(w, http.StatusNotFound, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "file": filepath.Base(path)})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req saveRequest
	if err := decodeBody(r, &req); err != nil || len(req.Definition) == 0 {
		writeErr(w, http.StatusBadRequest, "body must be {\"definition\": {...}}")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"validation": validateBytes(req.Definition)})
}

// validateBytes parses JSON into a Definition and returns engine validation
// errors. A parse failure is reported as a single synthetic error so the UI can
// show it the same way.
func validateBytes(b []byte) []engine.ValidationError {
	var def engine.Definition
	if err := json.Unmarshal(b, &def); err != nil {
		return []engine.ValidationError{{Path: "(json)", Message: err.Error()}}
	}
	errs := engine.ValidateDefinition(&def)
	if errs == nil {
		errs = []engine.ValidationError{}
	}
	return errs
}

// prettyJSON re-indents raw JSON for stable, diff-friendly files.
func prettyJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
