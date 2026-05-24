package engine

import (
	"fmt"
	"testing"
)

// defWithTriggers builds a minimal Definition with given world vars and triggers.
func defWithTriggers(world map[string]VarSpec, triggers map[string]Trigger) *Definition {
	return &Definition{
		ID:       "g",
		Version:  1,
		World:    world,
		Triggers: triggers,
	}
}

// stateWithWorld builds a State with preset world values.
func stateWithWorld(vals map[string]any) *State {
	return &State{
		Clock:         0,
		World:         vals,
		Machines:      map[string]string{},
		Entities:      map[string]*Entity{},
		Relationships: []*Relationship{},
		History:       []HistoryEntry{},
	}
}

// newSettleCtx builds the evalCtx used in Settle tests.
func newSettleCtx(def *Definition) *evalCtx {
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def
	return ctx
}

// ---------------------------------------------------------------------------
// (a) Rising edge fires once; staying true does not refire on second Settle.
// ---------------------------------------------------------------------------

func TestSettleRisingEdgeFiresOnce(t *testing.T) {
	def := defWithTriggers(
		map[string]VarSpec{"alarm": {Type: "bool", Default: false}},
		map[string]Trigger{
			"raise": {
				When:    &Guard{Target: "world.alarm", Op: "eq", Value: true},
				Effects: []Effect{{Op: "set", Target: "world.alarm", Value: true}}, // idempotent side-effect
			},
		},
	)
	st := stateWithWorld(map[string]any{"alarm": true})
	ctx := newSettleCtx(def)

	// First Settle: guard true, not armed → fires.
	r1 := Settle(def, st, ctx)
	if len(r1.Fired) != 1 || r1.Fired[0] != "raise" {
		t.Fatalf("first settle: expected [raise], got %v", r1.Fired)
	}
	if len(r1.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", r1.Warnings)
	}

	// Second Settle: guard still true, trigger already armed → does NOT fire.
	r2 := Settle(def, st, ctx)
	if len(r2.Fired) != 0 {
		t.Fatalf("second settle (stays true): expected no firing, got %v", r2.Fired)
	}
}

// ---------------------------------------------------------------------------
// (b) once:false refires after guard goes false then true again.
// ---------------------------------------------------------------------------

func TestSettleOnceFalseRefiresOnEdge(t *testing.T) {
	falseVal := false
	def := defWithTriggers(
		map[string]VarSpec{"flag": {Type: "bool", Default: false}},
		map[string]Trigger{
			"repeatable": {
				Once:    &falseVal,
				When:    &Guard{Target: "world.flag", Op: "eq", Value: true},
				Effects: []Effect{}, // no side-effects needed
			},
		},
	)
	st := stateWithWorld(map[string]any{"flag": true})
	ctx := newSettleCtx(def)

	// First settle: fires.
	r1 := Settle(def, st, ctx)
	if len(r1.Fired) != 1 {
		t.Fatalf("first settle: want 1 fired, got %v", r1.Fired)
	}

	// Guard goes false.
	st.World["flag"] = false
	r2 := Settle(def, st, ctx)
	if len(r2.Fired) != 0 {
		t.Fatalf("guard-false settle: want 0 fired, got %v", r2.Fired)
	}
	// Armed should now be false (disarmed).
	if st.TriggerArmed["repeatable"] {
		t.Fatal("trigger should be disarmed when guard is false")
	}

	// Guard goes true again → new rising edge → fires again.
	st.World["flag"] = true
	r3 := Settle(def, st, ctx)
	if len(r3.Fired) != 1 {
		t.Fatalf("re-edge settle: want 1 fired, got %v", r3.Fired)
	}
}

// ---------------------------------------------------------------------------
// (c) once:true fires at most once even across multiple edges.
// ---------------------------------------------------------------------------

func TestSettleOnceTrueFiresAtMostOnce(t *testing.T) {
	def := defWithTriggers(
		map[string]VarSpec{"flag": {Type: "bool", Default: false}},
		map[string]Trigger{
			"one_shot": {
				// Once is nil → once() returns true (one-shot).
				When:    &Guard{Target: "world.flag", Op: "eq", Value: true},
				Effects: []Effect{},
			},
		},
	)
	st := stateWithWorld(map[string]any{"flag": true})
	ctx := newSettleCtx(def)

	// First Settle: fires.
	r1 := Settle(def, st, ctx)
	if len(r1.Fired) != 1 {
		t.Fatalf("first settle: want 1 fired, got %v", r1.Fired)
	}

	// Guard goes false → disarms.
	st.World["flag"] = false
	Settle(def, st, ctx)

	// Guard goes true again → rising edge, but once=true and already fired → no fire.
	st.World["flag"] = true
	r3 := Settle(def, st, ctx)
	if len(r3.Fired) != 0 {
		t.Fatalf("once-true second edge: want 0 fired, got %v", r3.Fired)
	}
}

// ---------------------------------------------------------------------------
// (d) Cascade: trigger X sets a var that makes trigger Y's guard true.
//     Both fire in a single Settle call.
// ---------------------------------------------------------------------------

func TestSettleCascade(t *testing.T) {
	def := defWithTriggers(
		map[string]VarSpec{
			"x_on": {Type: "bool", Default: false},
			"y_on": {Type: "bool", Default: false},
		},
		map[string]Trigger{
			// "alpha" fires when x_on=true; its effect sets y_on=true.
			"alpha": {
				When: &Guard{Target: "world.x_on", Op: "eq", Value: true},
				Effects: []Effect{
					{Op: "set", Target: "world.y_on", Value: true},
				},
			},
			// "beta" fires when y_on=true (set by alpha).
			"beta": {
				When:    &Guard{Target: "world.y_on", Op: "eq", Value: true},
				Effects: []Effect{},
			},
		},
	)

	st := stateWithWorld(map[string]any{"x_on": true, "y_on": false})
	ctx := newSettleCtx(def)

	r := Settle(def, st, ctx)
	if len(r.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", r.Warnings)
	}

	// Both alpha and beta should have fired (in one Settle).
	fired := map[string]bool{}
	for _, id := range r.Fired {
		fired[id] = true
	}
	if !fired["alpha"] {
		t.Fatal("alpha did not fire")
	}
	if !fired["beta"] {
		t.Fatal("beta did not fire (cascade failed)")
	}

	// y_on should be true now.
	if st.World["y_on"] != true {
		t.Fatalf("y_on: %v", st.World["y_on"])
	}
}

// ---------------------------------------------------------------------------
// (e) Loop cap: a step-chain where each trigger fires in a separate outer
//     iteration.  After 100 outer iterations the cap is hit and Settle
//     returns a TRIGGER_LOOP warning before the chain finishes.
//
// Construction: 102 triggers named so they sort in REVERSE step order
// (trig_101 fires first in sort, trig_000 last). world.step starts at 0.
//
// In each outer iteration the trigger for the CURRENT step value is the
// only one whose guard turns true while having armed=false: all earlier
// (higher-numbered) triggers were already processed this pass when step
// was lower, so they are disarmed but not re-fired; the matching trigger is
// the last (lowest name) to be processed and fires, incrementing step.
// This produces exactly one firing per outer iteration → 101 outer
// iterations needed for steps 0..100, exceeding cap=100.
// ---------------------------------------------------------------------------

func TestSettleLoopCap(t *testing.T) {
	// chainLen outer iterations required to drain: must be > iterCap (100).
	const chainLen = 101

	falseVal := false

	world := map[string]VarSpec{
		"step": {Type: "int", Default: float64(0)},
	}

	// Triggers are named "trig_NNN" where NNN = (chainLen - i), so that
	// the trigger for step i sorts AFTER the trigger for step i+1.
	// Sorted order (ascending): trig for step chainLen, ..., step 1, step 0.
	// Effect: the trigger for the current step value is always processed LAST
	// in the inner loop, preventing forward cascading within a single pass.
	triggers := map[string]Trigger{}
	for i := 0; i <= chainLen; i++ {
		name := fmt.Sprintf("trig_%03d", chainLen-i)
		n := float64(i)
		triggers[name] = Trigger{
			Once: &falseVal,
			When: &Guard{Target: "world.step", Op: "eq", Value: n},
			Effects: []Effect{
				{Op: "set", Target: "world.step", Value: n + 1},
			},
		}
	}

	def := defWithTriggers(world, triggers)
	st := stateWithWorld(map[string]any{"step": float64(0)})
	ctx := newSettleCtx(def)

	r := Settle(def, st, ctx)

	hasLoop := false
	for _, w := range r.Warnings {
		if w == "TRIGGER_LOOP" {
			hasLoop = true
			break
		}
	}
	if !hasLoop {
		t.Fatalf("expected TRIGGER_LOOP warning, got warnings=%v fired=%v", r.Warnings, r.Fired)
	}
}
