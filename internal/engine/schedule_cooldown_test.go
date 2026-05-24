package engine

import "testing"

// helper: a minimal state with Clock set.
func stateWithClock(clock int) *State {
	return &State{
		Clock:         clock,
		World:         map[string]any{},
		Machines:      map[string]string{},
		Entities:      map[string]*Entity{},
		Relationships: []*Relationship{},
		History:       []HistoryEntry{},
	}
}

// --- schedule op ---

func TestScheduleAppends(t *testing.T) {
	st := stateWithClock(3)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	do := []Effect{{Op: "set", Target: "world.alarm", Value: false}}
	if err := applyEffect(def, st, ctx, Effect{Op: "schedule", In: 5, Do: do}); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if len(st.Scheduled) != 1 {
		t.Fatalf("expected 1 scheduled item, got %d", len(st.Scheduled))
	}
	item := st.Scheduled[0]
	if item.Due != 8 { // clock(3) + in(5)
		t.Fatalf("Due: got %d, want 8", item.Due)
	}
	if len(item.Effects) != 1 || item.Effects[0].Op != "set" {
		t.Fatalf("Effects: %+v", item.Effects)
	}
}

func TestScheduleErrorOnZeroIn(t *testing.T) {
	st := stateWithClock(0)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	err := applyEffect(def, st, ctx, Effect{Op: "schedule", In: 0, Do: []Effect{{Op: "set", Target: "world.x", Value: 1}}})
	if err == nil {
		t.Fatal("expected error for in=0")
	}
}

func TestScheduleErrorOnNegativeIn(t *testing.T) {
	st := stateWithClock(0)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	err := applyEffect(def, st, ctx, Effect{Op: "schedule", In: -1, Do: []Effect{}})
	if err == nil {
		t.Fatal("expected error for in=-1")
	}
}

// --- cooldown op ---

func TestCooldownSetsDue(t *testing.T) {
	st := stateWithClock(4)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	if err := applyEffect(def, st, ctx, Effect{Op: "cooldown", Key: "confess", Ticks: 3}); err != nil {
		t.Fatalf("cooldown: %v", err)
	}
	if st.Cooldowns == nil {
		t.Fatal("Cooldowns map is nil")
	}
	if st.Cooldowns["confess"] != 7 { // clock(4) + ticks(3)
		t.Fatalf("Cooldowns[confess]: got %d, want 7", st.Cooldowns["confess"])
	}
}

func TestCooldownInitialisesMap(t *testing.T) {
	st := stateWithClock(0)
	st.Cooldowns = nil // ensure nil starts OK
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	if err := applyEffect(def, st, ctx, Effect{Op: "cooldown", Key: "k", Ticks: 1}); err != nil {
		t.Fatalf("cooldown: %v", err)
	}
	if st.Cooldowns["k"] != 1 {
		t.Fatalf("Cooldowns[k]: %d", st.Cooldowns["k"])
	}
}

func TestCooldownErrorOnEmptyKey(t *testing.T) {
	st := stateWithClock(0)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	if err := applyEffect(def, st, ctx, Effect{Op: "cooldown", Key: "", Ticks: 2}); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCooldownErrorOnZeroTicks(t *testing.T) {
	st := stateWithClock(0)
	def := &Definition{ID: "g", Version: 1}
	ctx := newEvalCtx(nil, &RNG{state: 1})
	ctx.def = def

	if err := applyEffect(def, st, ctx, Effect{Op: "cooldown", Key: "k", Ticks: 0}); err == nil {
		t.Fatal("expected error for ticks=0")
	}
}

// --- clock path ---

func TestClockPath(t *testing.T) {
	st := stateWithClock(12)
	ctx := newEvalCtx(nil, &RNG{state: 1})

	v, err := resolvePath(st, ctx, "clock")
	if err != nil {
		t.Fatalf("resolvePath clock: %v", err)
	}
	f, ok := v.(float64)
	if !ok || f != 12 {
		t.Fatalf("clock path: got %v (%T), want 12.0", v, v)
	}
}

func TestClockPathGuard(t *testing.T) {
	st := stateWithClock(5)
	ctx := newEvalCtx(nil, &RNG{state: 1})

	g := &Guard{Target: "clock", Op: "gte", Value: float64(5)}
	ok, err := evalGuard(st, ctx, g)
	if err != nil {
		t.Fatalf("evalGuard: %v", err)
	}
	if !ok {
		t.Fatal("clock gte 5 should be true at clock=5")
	}

	g2 := &Guard{Target: "clock", Op: "gt", Value: float64(5)}
	ok2, err := evalGuard(st, ctx, g2)
	if err != nil {
		t.Fatalf("evalGuard: %v", err)
	}
	if ok2 {
		t.Fatal("clock gt 5 should be false at clock=5")
	}
}

// --- cooldown path ---

func TestCooldownPathCountdown(t *testing.T) {
	st := stateWithClock(4)
	st.Cooldowns = map[string]int{"confess": 7} // expires at tick 7

	ctx := newEvalCtx(nil, &RNG{state: 1})

	// At clock=4, due=7 → remaining = 3
	v, err := resolvePath(st, ctx, "cooldown.confess")
	if err != nil {
		t.Fatalf("resolvePath cooldown.confess: %v", err)
	}
	f, ok := v.(float64)
	if !ok || f != 3 {
		t.Fatalf("cooldown.confess: got %v, want 3", v)
	}

	// At clock=7 (due) → remaining = 0
	st.Clock = 7
	v2, err := resolvePath(st, ctx, "cooldown.confess")
	if err != nil {
		t.Fatalf("resolvePath cooldown.confess at expiry: %v", err)
	}
	f2, _ := v2.(float64)
	if f2 != 0 {
		t.Fatalf("at expiry: got %v, want 0", v2)
	}

	// Past expiry (clock > due) → remaining clamped to 0
	st.Clock = 10
	v3, err := resolvePath(st, ctx, "cooldown.confess")
	if err != nil {
		t.Fatalf("resolvePath cooldown.confess past expiry: %v", err)
	}
	f3, _ := v3.(float64)
	if f3 != 0 {
		t.Fatalf("past expiry: got %v, want 0", v3)
	}
}

func TestCooldownPathAbsent(t *testing.T) {
	// A key that was never set should return 0 (no error).
	st := stateWithClock(0)
	ctx := newEvalCtx(nil, &RNG{state: 1})

	v, err := resolvePath(st, ctx, "cooldown.unknown")
	if err != nil {
		t.Fatalf("absent cooldown key should not error: %v", err)
	}
	f, ok := v.(float64)
	if !ok || f != 0 {
		t.Fatalf("absent cooldown key: got %v, want 0", v)
	}
}

func TestCooldownPathGuardGate(t *testing.T) {
	// Typical gate: cooldown.confess eq 0 → ready to use.
	st := stateWithClock(10)
	st.Cooldowns = map[string]int{"confess": 12} // not yet expired

	ctx := newEvalCtx(nil, &RNG{state: 1})

	g := &Guard{Target: "cooldown.confess", Op: "eq", Value: float64(0)}
	ok, err := evalGuard(st, ctx, g)
	if err != nil {
		t.Fatalf("evalGuard: %v", err)
	}
	if ok {
		t.Fatal("cooldown not expired yet, guard should be false")
	}

	// After expiry.
	st.Clock = 12
	ok2, err := evalGuard(st, ctx, g)
	if err != nil {
		t.Fatalf("evalGuard after expiry: %v", err)
	}
	if !ok2 {
		t.Fatal("cooldown expired, guard should be true")
	}
}
