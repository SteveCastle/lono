package cli

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// seedAlarmGame creates a game with:
//   - world var "alarm" (bool, default false)
//   - world var "lockdown" (bool, default false)
//   - world var "clock" — tracked via st.Clock, not a var (no need)
//   - trigger "raise": when alarm=true → set lockdown=true (once)
//   - machine "arc": start → end (terminal)
func seedAlarmGame(t *testing.T, dir string) {
	t.Helper()
	spec := `{
		"id":"ag","version":1,
		"world":{
			"alarm":  {"type":"bool","default":false},
			"lockdown":{"type":"bool","default":false}
		},
		"machines":{"arc":{"initial":"start","states":["start","end"],
			"transitions":[{"id":"finish","from":"start","to":"end"}]}},
		"triggers":{
			"raise":{
				"when":{"target":"world.alarm","op":"eq","value":true},
				"effects":[{"op":"set","target":"world.lockdown","value":true}],
				"once":true
			}
		}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		b, _ := json.Marshal(env)
		t.Fatalf("seedAlarmGame import failed: %s", b)
	}
}

// ---------------------------------------------------------------------------
// Test: define trigger set / rm
// ---------------------------------------------------------------------------

func TestDefineTriggerSetRm(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "gt")

	// Add a world var so the trigger can reference it.
	env, _ := runCLI(t, dir, "define", "var", "set", "gt", "alarm",
		"--spec", `{"type":"bool","default":false}`)
	if !env.OK {
		t.Fatalf("define var alarm: %+v", env.Error)
	}
	env, _ = runCLI(t, dir, "define", "var", "set", "gt", "lockdown",
		"--spec", `{"type":"bool","default":false}`)
	if !env.OK {
		t.Fatalf("define var lockdown: %+v", env.Error)
	}

	// define trigger set: add a trigger.
	trigSpec := `{"when":{"target":"world.alarm","op":"eq","value":true},"effects":[{"op":"set","target":"world.lockdown","value":true}],"once":true}`
	env, _ = runCLI(t, dir, "define", "trigger", "set", "gt", "raise", "--spec", trigSpec)
	if !env.OK {
		t.Fatalf("define trigger set: %+v", env.Error)
	}
	// Definition returned should contain the trigger.
	def := env.Data.(map[string]any)
	triggers, ok := def["triggers"].(map[string]any)
	if !ok || triggers["raise"] == nil {
		t.Fatalf("triggers.raise not in definition: %+v", def)
	}

	// define trigger rm: remove it.
	env, _ = runCLI(t, dir, "define", "trigger", "rm", "gt", "raise")
	if !env.OK {
		t.Fatalf("define trigger rm: %+v", env.Error)
	}
	def2 := env.Data.(map[string]any)
	triggers2, _ := def2["triggers"].(map[string]any)
	if triggers2["raise"] != nil {
		t.Fatalf("trigger raise still present after rm: %+v", def2)
	}
}

// ---------------------------------------------------------------------------
// Test: trigger defined via define fires after apply sets alarm
// ---------------------------------------------------------------------------

func TestDefineTriggerFiresAfterApply(t *testing.T) {
	dir := t.TempDir()

	runCLI(t, dir, "game", "create", "g")
	runCLI(t, dir, "define", "var", "set", "g", "alarm",
		"--spec", `{"type":"bool","default":false}`)
	runCLI(t, dir, "define", "var", "set", "g", "lockdown",
		"--spec", `{"type":"bool","default":false}`)
	runCLI(t, dir, "define", "trigger", "set", "g", "raise",
		"--spec", `{"when":{"target":"world.alarm","op":"eq","value":true},"effects":[{"op":"set","target":"world.lockdown","value":true}],"once":true}`)
	runCLI(t, dir, "play", "start", "g", "--id", "r1", "--seed", "1")

	// apply sets alarm=true; Settle should fire "raise" and set lockdown=true.
	env, _ := runCLI(t, dir, "apply", "r1", "--ops", `[{"op":"set","target":"world.alarm","value":true}]`)
	if !env.OK {
		t.Fatalf("apply: %+v", env.Error)
	}

	data := env.Data.(map[string]any)

	// world.lockdown must be true (trigger fired).
	st := data["state"].(map[string]any)
	world := st["world"].(map[string]any)
	if world["lockdown"] != true {
		t.Fatalf("lockdown: got %v, want true", world["lockdown"])
	}

	// fired must contain "raise".
	fired, _ := data["fired"].([]any)
	foundRaise := false
	for _, f := range fired {
		if f == "raise" {
			foundRaise = true
		}
	}
	if !foundRaise {
		t.Fatalf("fired does not contain 'raise': %v", fired)
	}
}

// ---------------------------------------------------------------------------
// Test: advance increments clock
// ---------------------------------------------------------------------------

func TestAdvanceIncrementsClockCommand(t *testing.T) {
	dir := t.TempDir()
	seedAlarmGame(t, dir)
	runCLI(t, dir, "play", "start", "ag", "--id", "run1", "--seed", "1")

	env, _ := runCLI(t, dir, "advance", "run1", "3")
	if !env.OK {
		t.Fatalf("advance 3: %+v", env.Error)
	}
	data := env.Data.(map[string]any)

	// clock should be 3.
	clock, _ := data["clock"].(float64)
	if clock != 3 {
		t.Fatalf("clock: got %v, want 3", clock)
	}

	// state.clock should also be 3.
	st := data["state"].(map[string]any)
	stClock, _ := st["clock"].(float64)
	if stClock != 3 {
		t.Fatalf("state.clock: got %v, want 3", stClock)
	}
}

// ---------------------------------------------------------------------------
// Test: advance default n=1
// ---------------------------------------------------------------------------

func TestAdvanceDefaultOne(t *testing.T) {
	dir := t.TempDir()
	seedAlarmGame(t, dir)
	runCLI(t, dir, "play", "start", "ag", "--id", "run2", "--seed", "1")

	env, _ := runCLI(t, dir, "advance", "run2")
	if !env.OK {
		t.Fatalf("advance (default n=1): %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	clock, _ := data["clock"].(float64)
	if clock != 1 {
		t.Fatalf("clock: got %v, want 1", clock)
	}
}

// ---------------------------------------------------------------------------
// Test: advance fires scheduled bad-ending trigger
// ---------------------------------------------------------------------------

func TestAdvanceScheduledTriggerReachesEnding(t *testing.T) {
	dir := t.TempDir()
	// Game with a scheduled effect (set via apply) and a trigger that fires on
	// the resulting state. We'll use a simpler setup: a trigger fires when
	// world.caught=true, and we advance after scheduling that.
	spec := `{
		"id":"se","version":1,
		"world":{"caught":{"type":"bool","default":false}},
		"machines":{"arc":{"initial":"free","states":["free","jail"],
			"transitions":[{"id":"escape","from":"free","to":"jail"}]}},
		"triggers":{
			"arrest":{
				"when":{"target":"world.caught","op":"eq","value":true},
				"effects":[{"op":"set_machine_state","machine":"arc","state":"jail"}],
				"once":true
			}
		}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if !env.OK {
		b, _ := json.Marshal(env)
		t.Fatalf("import failed: %s", b)
	}
	runCLI(t, dir, "play", "start", "se", "--id", "se1", "--seed", "1")

	// Schedule caught=true to fire at tick 2 via apply.
	env, _ = runCLI(t, dir, "apply", "se1", "--ops",
		`[{"op":"schedule","in":2,"do":[{"op":"set","target":"world.caught","value":true}]}]`)
	if !env.OK {
		t.Fatalf("apply schedule: %+v", env.Error)
	}

	// Advance 2 ticks: scheduled fires at tick 2, then Settle fires arrest trigger.
	env, _ = runCLI(t, dir, "advance", "se1", "2")
	if !env.OK {
		t.Fatalf("advance 2: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	st := data["state"].(map[string]any)
	machines := st["machines"].(map[string]any)
	if machines["arc"] != "jail" {
		t.Fatalf("arc machine: got %v, want jail", machines["arc"])
	}
	// fired should include "arrest".
	fired, _ := data["fired"].([]any)
	foundArrest := false
	for _, f := range fired {
		if f == "arrest" {
			foundArrest = true
		}
	}
	if !foundArrest {
		t.Fatalf("fired does not contain 'arrest': %v", fired)
	}
}

// ---------------------------------------------------------------------------
// Test: stateData always includes clock
// ---------------------------------------------------------------------------

func TestStateDataAlwaysIncludesClock(t *testing.T) {
	dir := t.TempDir()
	seedAlarmGame(t, dir)
	runCLI(t, dir, "play", "start", "ag", "--id", "ck1", "--seed", "1")

	// Regular state command should include clock.
	env, _ := runCLI(t, dir, "state", "ck1")
	if !env.OK {
		t.Fatalf("state: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	if _, hasClock := data["clock"]; !hasClock {
		t.Fatalf("stateData missing 'clock' field: %+v", data)
	}
}

// ---------------------------------------------------------------------------
// Test: validate rejects trigger without when or every
// ---------------------------------------------------------------------------

func TestValidateTriggerNeedsWhenOrEvery(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "vt")

	// A trigger with neither when nor every — validation should fail.
	// We use game import to bypass define trigger's validation path and
	// go directly to ValidateDefinition.
	spec := `{
		"id":"vt","version":1,
		"triggers":{"bad":{"effects":[]}}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if env.OK {
		t.Fatal("expected validation failure for trigger without when or every")
	}
}

// ---------------------------------------------------------------------------
// Test: validate rejects schedule with in=0
// ---------------------------------------------------------------------------

func TestValidateScheduleRequiresPositiveIn(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "vs")

	spec := `{
		"id":"vs","version":1,
		"world":{"x":{"type":"bool","default":false}},
		"triggers":{
			"bad":{
				"when":{"target":"world.x","op":"eq","value":true},
				"effects":[{"op":"schedule","in":0,"do":[]}]
			}
		}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if env.OK {
		t.Fatal("expected validation failure for schedule with in=0")
	}
}

// ---------------------------------------------------------------------------
// Test: validate rejects cooldown with empty key
// ---------------------------------------------------------------------------

func TestValidateCooldownRequiresKey(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "game", "create", "vc")

	spec := `{
		"id":"vc","version":1,
		"world":{"x":{"type":"bool","default":false}},
		"triggers":{
			"bad":{
				"when":{"target":"world.x","op":"eq","value":true},
				"effects":[{"op":"cooldown","key":"","ticks":3}]
			}
		}
	}`
	env, _ := runCLI(t, dir, "game", "import", "--spec", spec)
	if env.OK {
		t.Fatal("expected validation failure for cooldown with empty key")
	}
}

// ---------------------------------------------------------------------------
// Test: apply output includes fired and warnings when present
// ---------------------------------------------------------------------------

func TestApplyOutputIncludesFiredAndWarnings(t *testing.T) {
	dir := t.TempDir()
	seedAlarmGame(t, dir)
	runCLI(t, dir, "play", "start", "ag", "--id", "fw1", "--seed", "1")

	// Apply sets alarm; trigger raise fires.
	env, _ := runCLI(t, dir, "apply", "fw1", "--ops", `[{"op":"set","target":"world.alarm","value":true}]`)
	if !env.OK {
		t.Fatalf("apply: %+v", env.Error)
	}
	data := env.Data.(map[string]any)
	fired, hasFired := data["fired"]
	_ = hasFired
	// fired should be present and include "raise".
	firedSlice, ok := fired.([]any)
	if !ok || len(firedSlice) == 0 {
		t.Fatalf("apply: fired not in output or empty: %+v", data)
	}
	if firedSlice[0] != "raise" {
		t.Fatalf("apply: fired[0]=%v, want raise", firedSlice[0])
	}
}
