package store

import (
	"testing"

	"github.com/callsignmedia/lono/internal/engine"
)

func TestResolveDataDir(t *testing.T) {
	if got := resolveDataDir("/explicit"); got != "/explicit" {
		t.Fatalf("explicit flag ignored: %s", got)
	}
	t.Setenv("LONO_HOME", "/from/env")
	if got := resolveDataDir(""); got != "/from/env" {
		t.Fatalf("env not used: %s", got)
	}
	t.Setenv("LONO_HOME", "")
	if got := resolveDataDir(""); got != ".lono" {
		t.Fatalf("default wrong: %s", got)
	}
}

func TestDefinitionPersistence(t *testing.T) {
	s := Open(t.TempDir())
	def := &engine.Definition{ID: "g1", Name: "One", Version: 1}
	if err := s.SaveDefinition(def); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDefinition("g1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "One" {
		t.Fatalf("round-trip lost data: %+v", got)
	}
	ids, err := s.ListGames()
	if err != nil || len(ids) != 1 || ids[0] != "g1" {
		t.Fatalf("ListGames wrong: %v %v", ids, err)
	}
	if err := s.DeleteGame("g1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadDefinition("g1"); err == nil {
		t.Fatal("expected load error after delete")
	}
}

func TestInstanceAndSnapshotPersistence(t *testing.T) {
	s := Open(t.TempDir())
	st := &engine.State{ID: "run1", GameID: "g1", World: map[string]any{"day": float64(1)}}
	if err := s.SaveState(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadState("run1")
	if err != nil || got.GameID != "g1" {
		t.Fatalf("state round-trip: %v %+v", err, got)
	}
	ids, _ := s.ListInstances()
	if len(ids) != 1 || ids[0] != "run1" {
		t.Fatalf("ListInstances wrong: %v", ids)
	}

	snap := &Snapshot{ID: "s1", Label: "before fight", State: st}
	if err := s.SaveSnapshot("run1", snap); err != nil {
		t.Fatal(err)
	}
	metas, err := s.ListSnapshots("run1")
	if err != nil || len(metas) != 1 || metas[0].Label != "before fight" {
		t.Fatalf("ListSnapshots wrong: %v %v", metas, err)
	}
	loaded, err := s.LoadSnapshot("run1", "s1")
	if err != nil || loaded.State.GameID != "g1" {
		t.Fatalf("snapshot round-trip: %v %+v", err, loaded)
	}
}

func TestRejectsUnsafeIDs(t *testing.T) {
	s := Open(t.TempDir())
	if err := s.SaveDefinition(&engine.Definition{ID: "../evil"}); err == nil {
		t.Fatal("SaveDefinition should reject traversal id")
	}
	if _, err := s.LoadState("../../x"); err == nil {
		t.Fatal("LoadState should reject traversal id")
	}
	if err := s.SaveState(&engine.State{ID: "a/b"}); err == nil {
		t.Fatal("SaveState should reject id with separator")
	}
	if err := s.SaveSnapshot("ok", &Snapshot{ID: "..", State: &engine.State{ID: "ok"}}); err == nil {
		t.Fatal("SaveSnapshot should reject .. snapshot id")
	}
	// A normal id still round-trips.
	if err := s.SaveState(&engine.State{ID: "run1"}); err != nil {
		t.Fatalf("normal id should be accepted: %v", err)
	}
}
