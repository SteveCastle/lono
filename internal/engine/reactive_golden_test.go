package engine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// loadReactive loads the reactive.json golden definition.
func loadReactive(t *testing.T) *Definition {
	t.Helper()
	b, err := os.ReadFile("../../testdata/reactive.json")
	if err != nil {
		t.Fatal(err)
	}
	var def Definition
	if err := json.Unmarshal(b, &def); err != nil {
		t.Fatal(err)
	}
	return &def
}

// TestReactiveDefinitionValid verifies the reactive game definition is
// internally consistent.
func TestReactiveDefinitionValid(t *testing.T) {
	if errs := ValidateDefinition(loadReactive(t)); len(errs) != 0 {
		t.Fatalf("reactive definition should be valid; errors: %v", errs)
	}
}

// TestReactivePlaythrough drives two deterministic playthroughs:
//
//   - successSeed (2): 1d20 roll = 11 (≥10) → loot increases, escape succeeds.
//   - failureSeed (1): 1d20 roll = 6  (<10) → alarm fires, on_alarm trigger fires,
//     scheduled capture arrives after AdvanceInstance(2), caught ending reached.
//
// Exercises: set (add_to/contains), compute ($path operand), if/then/else,
// roll.<store> path, trigger (on_alarm), schedule, cooldown, record/Log,
// len.<path> guard, EndingsReached.
func TestReactivePlaythrough(t *testing.T) {
	// Empirically determined seeds:
	//   successSeed=2 → first 1d20 roll = 11 (≥ 10) → success branch
	//   failureSeed=1 → first 1d20 roll = 6  (<  10) → failure branch
	const successSeed int64 = 2
	const failureSeed int64 = 1

	def := loadReactive(t)

	// ------------------------------------------------------------------
	// SUCCESS PATH (seed 2, roll 11 → crack safe succeeds)
	// ------------------------------------------------------------------
	t.Run("success_path", func(t *testing.T) {
		st, err := StartInstance(def, "success_run", successSeed)
		if err != nil {
			t.Fatalf("StartInstance: %v", err)
		}

		// --- Setup: player entity exists with empty clues set ---
		player := st.Entities["player"]
		if player == nil {
			t.Fatal("player entity not created by cast")
		}
		if player.Attrs["name"] != "Vex" {
			t.Fatalf("player.name: got %v, want Vex", player.Attrs["name"])
		}
		clues, ok := player.Attrs["clues"].([]any)
		if !ok {
			t.Fatalf("player.clues should be []any, got %T", player.Attrs["clues"])
		}
		if len(clues) != 0 {
			t.Fatalf("player.clues should start empty, got %v", clues)
		}

		// --- break_in: outside → inside ---
		st, _, err = PerformAction(def, st, "arc", "break_in", nil)
		if err != nil {
			t.Fatalf("break_in: %v", err)
		}
		if st.Machines["arc"] != "inside" {
			t.Fatalf("arc after break_in: got %q, want inside", st.Machines["arc"])
		}
		// Log entry from break_in.
		if !logContains(st, "You slip through the service entrance.") {
			t.Fatal("Log missing break_in entry")
		}

		// --- grab_clue: add "ledger" to player.clues ---
		st, _, err = PerformAction(def, st, "arc", "grab_clue", nil)
		if err != nil {
			t.Fatalf("grab_clue: %v", err)
		}
		player = st.Entities["player"]
		clues, _ = player.Attrs["clues"].([]any)
		if !sliceContainsStr(clues, "ledger") {
			t.Fatalf("player.clues after grab_clue: got %v, want [ledger]", clues)
		}
		// Log entry from grab_clue.
		if !logContains(st, "You pocket the ledger.") {
			t.Fatal("Log missing grab_clue entry")
		}

		// --- crack_safe: roll 1d20 = 11 → success branch ---
		// Guard: cooldown.crack eq 0 (initially 0 → passes).
		// Effects: cooldown crack+3; compute world.attempts+1; roll 1d20 store r;
		//          if roll.r >= 10: inc world.loot 5, record; else: alarm.
		st, res, err := PerformAction(def, st, "arc", "crack_safe", nil)
		if err != nil {
			t.Fatalf("crack_safe: %v", err)
		}

		// Roll must be 11 with this seed.
		if len(res.Rolls) != 1 || res.Rolls[0].Result != 11 {
			t.Fatalf("crack_safe rolls: got %v, want [{r 1d20 11}]", res.Rolls)
		}
		// Success branch: loot=5, alarm unchanged (false).
		if st.World["loot"] != float64(5) {
			t.Fatalf("world.loot: got %v, want 5", st.World["loot"])
		}
		if st.World["alarm"] != false {
			t.Fatalf("world.alarm: got %v, want false", st.World["alarm"])
		}
		// compute: attempts incremented via $path operand.
		if st.World["attempts"] != float64(1) {
			t.Fatalf("world.attempts: got %v, want 1", st.World["attempts"])
		}
		// cooldown: crack cooldown set (due = 0 + 3 = 3).
		if st.Cooldowns["crack"] != 3 {
			t.Fatalf("cooldown.crack: got %d, want 3", st.Cooldowns["crack"])
		}
		// on_alarm must NOT have fired (no alarm).
		if len(res.Fired) != 0 {
			t.Fatalf("on_alarm should not fire on success; Fired=%v", res.Fired)
		}
		// Log contains success message.
		if !logContains(st, "You crack the safe.") {
			t.Fatal("Log missing crack_safe success entry")
		}

		// --- crack_safe is now on cooldown: guard should fail ---
		_, _, errCD := PerformAction(def, st, "arc", "crack_safe", nil)
		if errCD == nil {
			t.Fatal("crack_safe should be blocked by cooldown guard")
		}

		// --- escape: guard requires loot>0 AND contains ledger AND len>=1 ---
		st, _, err = PerformAction(def, st, "arc", "escape", nil)
		if err != nil {
			t.Fatalf("escape: %v", err)
		}
		if st.Machines["arc"] != "escaped" {
			t.Fatalf("arc after escape: got %q, want escaped", st.Machines["arc"])
		}
		// Terminal ending: escaped.
		endings := EndingsReached(def, st)
		if len(endings) != 1 || endings[0].State != "escaped" {
			t.Fatalf("EndingsReached: got %+v, want [escaped]", endings)
		}
		// Log has escape entry.
		if !logContains(st, "You escape into the night.") {
			t.Fatal("Log missing escape entry")
		}
		// At least 4 log entries (break_in, grab_clue, crack_safe success, escape).
		if len(st.Log) < 4 {
			t.Fatalf("Log: got %d entries, want ≥4", len(st.Log))
		}
	})

	// ------------------------------------------------------------------
	// FAILURE PATH (seed 1, roll 6 → alarm fires, captured in 2 ticks)
	// ------------------------------------------------------------------
	t.Run("failure_path", func(t *testing.T) {
		st, err := StartInstance(def, "failure_run", failureSeed)
		if err != nil {
			t.Fatalf("StartInstance: %v", err)
		}

		// break_in
		st, _, err = PerformAction(def, st, "arc", "break_in", nil)
		if err != nil {
			t.Fatalf("break_in: %v", err)
		}

		// grab_clue
		st, _, err = PerformAction(def, st, "arc", "grab_clue", nil)
		if err != nil {
			t.Fatalf("grab_clue: %v", err)
		}

		// crack_safe: roll 1d20 = 6 → failure branch → alarm=true → on_alarm fires.
		st, res, err := PerformAction(def, st, "arc", "crack_safe", nil)
		if err != nil {
			t.Fatalf("crack_safe: %v", err)
		}

		// Roll must be 6 with this seed.
		if len(res.Rolls) != 1 || res.Rolls[0].Result != 6 {
			t.Fatalf("crack_safe rolls: got %v, want [{r 1d20 6}]", res.Rolls)
		}
		// Failure branch: loot=0, alarm=true.
		if st.World["loot"] != float64(0) {
			t.Fatalf("world.loot: got %v, want 0", st.World["loot"])
		}
		if st.World["alarm"] != true {
			t.Fatalf("world.alarm: got %v, want true", st.World["alarm"])
		}
		// on_alarm trigger must have fired during Settle after crack_safe.
		if !strSliceContains(res.Fired, "on_alarm") {
			t.Fatalf("on_alarm not fired after alarm set; Fired=%v", res.Fired)
		}
		// on_alarm schedules capture at clock+2 (clock=0 → Due=2).
		if len(st.Scheduled) != 1 || st.Scheduled[0].Due != 2 {
			t.Fatalf("Scheduled: got %+v, want [{Due:2 …}]", st.Scheduled)
		}
		// Log: alarm blares + guards mobilize entries from trigger.
		if !logContains(st, "The safe alarm blares.") {
			t.Fatal("Log missing alarm blares entry")
		}
		if !logContains(st, "Guards mobilize — 2 turns to escape.") {
			t.Fatal("Log missing guards mobilize entry (trigger record)")
		}

		// --- AdvanceInstance 2 ticks → scheduled capture fires → caught ---
		st, advRes, err := AdvanceInstance(def, st, 2)
		if err != nil {
			t.Fatalf("AdvanceInstance: %v", err)
		}
		_ = advRes
		if st.Machines["arc"] != "caught" {
			t.Fatalf("arc after advance 2: got %q, want caught", st.Machines["arc"])
		}
		if st.Clock != 2 {
			t.Fatalf("Clock after advance 2: got %d, want 2", st.Clock)
		}
		// Terminal ending: caught.
		endings := EndingsReached(def, st)
		if len(endings) != 1 || endings[0].State != "caught" {
			t.Fatalf("EndingsReached after caught: got %+v, want [caught]", endings)
		}
	})
}

// TestReactiveDeterministic verifies that the same seed always produces the
// same 1d20 roll for crack_safe.
func TestReactiveDeterministic(t *testing.T) {
	def := loadReactive(t)

	roll := func(seed int64) float64 {
		st, _ := StartInstance(def, "det", seed)
		st, _, _ = PerformAction(def, st, "arc", "break_in", nil)
		st, _, _ = PerformAction(def, st, "arc", "grab_clue", nil)
		_, res, _ := PerformAction(def, st, "arc", "crack_safe", nil)
		if res == nil || len(res.Rolls) == 0 {
			return -1
		}
		return res.Rolls[0].Result
	}

	// Success seed: same result on two independent runs.
	if roll(2) != roll(2) {
		t.Fatal("success seed: same seed must produce same roll")
	}
	// Failure seed: same result on two independent runs.
	if roll(1) != roll(1) {
		t.Fatal("failure seed: same seed must produce same roll")
	}
	// The two seeds produce different rolls.
	if roll(2) == roll(1) {
		t.Fatal("seeds 2 and 1 produced the same roll (unexpected collision)")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// logContains reports whether any log entry has the given text.
func logContains(st *State, text string) bool {
	for _, e := range st.Log {
		if strings.Contains(e.Text, text) {
			return true
		}
	}
	return false
}

// sliceContainsStr reports whether a []any contains the given string.
func sliceContainsStr(arr []any, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// strSliceContains reports whether a []string contains the given string.
func strSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
