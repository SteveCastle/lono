// Package editor hosts a local web app for authoring lono game definitions.
// It serves an embedded single-page editor plus a JSON API that reads and writes
// *.lono.json files in a working directory, validates them with the engine, and
// runs in-memory playtests so a designer can exercise a game without leaving the
// editor.
package editor

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"sync"
)

//go:embed web
var webFS embed.FS

// Server hosts the editor for a single working directory.
type Server struct {
	dir string

	mu       sync.Mutex
	sessions map[string]*session // ephemeral playtest sessions
	seq      int
}

// NewServer returns an editor server rooted at dir (where *.lono.json live).
func NewServer(dir string) *Server {
	return &Server{dir: dir, sessions: map[string]*session{}}
}

// Handler returns the http.Handler serving both the API and the embedded UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// --- API: vocabulary + files ---
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/games", s.handleListGames)
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/{file}", s.handleGetGame)
	mux.HandleFunc("PUT /api/games/{file}", s.handleSaveGame)
	mux.HandleFunc("DELETE /api/games/{file}", s.handleDeleteGame)
	mux.HandleFunc("POST /api/validate", s.handleValidate)

	// --- API: playtest ---
	mux.HandleFunc("POST /api/playtest", s.handlePlaytestStart)
	mux.HandleFunc("POST /api/playtest/{session}/act", s.handlePlaytestAct)
	mux.HandleFunc("POST /api/playtest/{session}/advance", s.handlePlaytestAdvance)
	mux.HandleFunc("POST /api/playtest/{session}/apply", s.handlePlaytestApply)
	mux.HandleFunc("DELETE /api/playtest/{session}", s.handlePlaytestEnd)

	// --- static SPA (embedded) ---
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // embed path is a compile-time constant
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	return mux
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildMeta(s.dir))
}

// --- file-name safety -------------------------------------------------------

var fileNameRe = regexp.MustCompile(`^[A-Za-z0-9 ._-]+\.lono\.json$`)

// safePath resolves a request {file} param to an absolute path inside the
// working dir, rejecting traversal and anything not named *.lono.json.
func (s *Server) safePath(name string) (string, error) {
	base := filepath.Base(name) // strip any directory components
	if base != name || !fileNameRe.MatchString(base) {
		return "", fmt.Errorf("invalid file name %q (must be a *.lono.json file with no path separators)", name)
	}
	return filepath.Join(s.dir, base), nil
}

// --- JSON helpers -----------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]any{"error": fmt.Sprintf(format, args...)})
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
