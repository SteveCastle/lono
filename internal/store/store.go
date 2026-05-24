package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/callsignmedia/lono/internal/engine"
)

type Store struct{ root string }

// validID rejects ids that are not a single safe path segment, preventing
// path traversal from untrusted (e.g. LLM-supplied) game/instance/snapshot ids.
func validID(kind, id string) error {
	if id == "" {
		return fmt.Errorf("%s id must not be empty", kind)
	}
	if id == "." || id == ".." || id != filepath.Base(id) || strings.ContainsAny(id, `/\:`) {
		return fmt.Errorf("invalid %s id %q: must be a single path segment without separators", kind, id)
	}
	return nil
}

// resolveDataDir picks the data directory: flag, then $LONO_HOME, then ./.lono.
func resolveDataDir(flag string) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("LONO_HOME"); env != "" {
		return env
	}
	return ".lono"
}

func Open(dataDir string) *Store { return &Store{root: resolveDataDir(dataDir)} }

func (s *Store) gameDir(id string) string  { return filepath.Join(s.root, "games", id) }
func (s *Store) gamePath(id string) string { return filepath.Join(s.gameDir(id), "definition.json") }

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	// Unique temp file in the same dir so concurrent writers can't collide on
	// the temp name; rename is atomic (replaces an existing file on Windows too).
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (s *Store) SaveDefinition(def *engine.Definition) error {
	if err := validID("game", def.ID); err != nil {
		return err
	}
	return writeJSON(s.gamePath(def.ID), def)
}

func (s *Store) LoadDefinition(id string) (*engine.Definition, error) {
	if err := validID("game", id); err != nil {
		return nil, err
	}
	var def engine.Definition
	if err := readJSON(s.gamePath(id), &def); err != nil {
		return nil, fmt.Errorf("load game %q: %w", id, err)
	}
	return &def, nil
}

func (s *Store) ListGames() ([]string, error) {
	return listDir(filepath.Join(s.root, "games"))
}

func (s *Store) DeleteGame(id string) error {
	if err := validID("game", id); err != nil {
		return err
	}
	return os.RemoveAll(s.gameDir(id))
}

func listDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

type Snapshot struct {
	ID        string        `json:"id"`
	Label     string        `json:"label,omitempty"`
	Parent    string        `json:"parent,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	State     *engine.State `json:"state"`
}

type SnapshotMeta struct {
	ID        string    `json:"id"`
	Label     string    `json:"label,omitempty"`
	Parent    string    `json:"parent,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) instanceDir(id string) string    { return filepath.Join(s.root, "instances", id) }
func (s *Store) statePath(id string) string      { return filepath.Join(s.instanceDir(id), "state.json") }
func (s *Store) snapDir(id string) string        { return filepath.Join(s.instanceDir(id), "snapshots") }
func (s *Store) snapPath(id, snap string) string { return filepath.Join(s.snapDir(id), snap+".json") }

func (s *Store) SaveState(st *engine.State) error {
	if err := validID("instance", st.ID); err != nil {
		return err
	}
	return writeJSON(s.statePath(st.ID), st)
}

func (s *Store) LoadState(id string) (*engine.State, error) {
	if err := validID("instance", id); err != nil {
		return nil, err
	}
	var st engine.State
	if err := readJSON(s.statePath(id), &st); err != nil {
		return nil, fmt.Errorf("load instance %q: %w", id, err)
	}
	return &st, nil
}

func (s *Store) ListInstances() ([]string, error) {
	return listDir(filepath.Join(s.root, "instances"))
}

func (s *Store) SaveSnapshot(instanceID string, snap *Snapshot) error {
	if err := validID("instance", instanceID); err != nil {
		return err
	}
	if err := validID("snapshot", snap.ID); err != nil {
		return err
	}
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}
	return writeJSON(s.snapPath(instanceID, snap.ID), snap)
}

func (s *Store) LoadSnapshot(instanceID, snapID string) (*Snapshot, error) {
	if err := validID("instance", instanceID); err != nil {
		return nil, err
	}
	if err := validID("snapshot", snapID); err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := readJSON(s.snapPath(instanceID, snapID), &snap); err != nil {
		return nil, fmt.Errorf("load snapshot %q/%q: %w", instanceID, snapID, err)
	}
	return &snap, nil
}

func (s *Store) ListSnapshots(instanceID string) ([]SnapshotMeta, error) {
	if err := validID("instance", instanceID); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.snapDir(instanceID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		snap, err := s.LoadSnapshot(instanceID, e.Name()[:len(e.Name())-5])
		if err != nil {
			return nil, err
		}
		out = append(out, SnapshotMeta{ID: snap.ID, Label: snap.Label, Parent: snap.Parent, CreatedAt: snap.CreatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
