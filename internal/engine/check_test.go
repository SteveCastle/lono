package engine

import "testing"

// checkDef has a character with a designer-defined skill attribute used as a
// check modifier, plus an opponent whose attribute drives an opposed check.
func checkDef() *Definition {
	return &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{"opened": {Type: "bool", Default: false}},
		EntityTypes: map[string]EntityType{
			"character": {Attributes: map[string]VarSpec{
				"lockpicking": {Type: "int", Default: float64(0)},
				"perception":  {Type: "int", Default: float64(0)},
			}},
		},
	}
}

func checkState() *State {
	st, _ := NewInstance(checkDef(), "r", 1)
	st.Entities["player"] = &Entity{Type: "character", Attrs: map[string]any{"lockpicking": float64(7)}, Inventory: map[string]int{}}
	st.Entities["guard"] = &Entity{Type: "character", Attrs: map[string]any{"perception": float64(12)}, Inventory: map[string]int{}}
	return st
}

// rollFor returns a fixed-seed RNG; we read the natural roll it produces so the
// assertions don't depend on the splitmix64 internals.
func TestCheckSuccessAndMargin(t *testing.T) {
	def := checkDef()
	st := checkState()
	ctx := newEvalCtx(nil, &RNG{state: 1})

	// 1d20 + lockpicking(7) vs DC 8. Whatever the die is, total = die+7 and the
	// stored result must be internally consistent and drive success.
	if err := applyEffect(def, st, ctx, Effect{Op: "check", Dice: "1d20", Mods: []any{map[string]any{"$path": "entity.player.lockpicking"}}, DC: float64(8), Store: "pick"}); err != nil {
		t.Fatal(err)
	}
	cr, ok := ctx.checks["pick"]
	if !ok {
		t.Fatal("check result not stored")
	}
	if cr.Total != cr.Roll+7 {
		t.Fatalf("total %v should be roll %v + 7", cr.Total, cr.Roll)
	}
	if cr.DC != 8 || cr.Margin != cr.Total-8 {
		t.Fatalf("dc/margin wrong: dc=%v margin=%v total=%v", cr.DC, cr.Margin, cr.Total)
	}
	if cr.Success != (cr.Total >= 8) {
		t.Fatalf("success %v inconsistent with total %v vs dc 8", cr.Success, cr.Total)
	}
	if len(ctx.checkLog) != 1 {
		t.Fatalf("check should be recorded for the action result, got %d", len(ctx.checkLog))
	}
}

func TestCheckPathReadsAndBranch(t *testing.T) {
	def := checkDef()
	st := checkState()
	ctx := newEvalCtx(nil, &RNG{state: 1})
	// Force an unbeatable DC so success is deterministically false.
	if err := applyEffect(def, st, ctx, Effect{Op: "check", Dice: "1d20", DC: float64(999), Store: "x"}); err != nil {
		t.Fatal(err)
	}
	// check.<store>.* are readable as paths.
	for path, want := range map[string]any{
		"check.x.success": false,
		"check.x.dc":      float64(999),
	} {
		got, err := resolvePath(st, ctx, path)
		if err != nil {
			t.Fatalf("resolve %s: %v", path, err)
		}
		if got != want {
			t.Fatalf("%s = %v, want %v", path, got, want)
		}
	}
	// an `if` branches on the failed check.
	if err := applyEffect(def, st, ctx, Effect{Op: "if",
		When: &Guard{Target: "check.x.success", Op: "eq", Value: true},
		Then: []Effect{{Op: "set", Target: "world.opened", Value: true}},
		Else: []Effect{{Op: "set", Target: "world.opened", Value: false}},
	}); err != nil {
		t.Fatal(err)
	}
	if st.World["opened"] != false {
		t.Fatalf("else branch should have run (check failed): opened=%v", st.World["opened"])
	}
}

func TestCheckOpposedViaPathDC(t *testing.T) {
	def := checkDef()
	st := checkState()
	ctx := newEvalCtx(nil, &RNG{state: 1})
	// Opposed: dc is the guard's perception attribute (12), no flat dc literal.
	if err := applyEffect(def, st, ctx, Effect{Op: "check", Dice: "1d6",
		Mods: []any{map[string]any{"$path": "entity.player.lockpicking"}},
		DC:   map[string]any{"$path": "entity.guard.perception"}, Store: "sneak"}); err != nil {
		t.Fatal(err)
	}
	if ctx.checks["sneak"].DC != 12 {
		t.Fatalf("opposed dc should resolve to guard.perception 12, got %v", ctx.checks["sneak"].DC)
	}
}

func TestCheckValidation(t *testing.T) {
	bad := []Effect{
		{Op: "check", Store: "x", DC: float64(5)},   // no dice
		{Op: "check", Dice: "1d20", DC: float64(5)}, // no store
		{Op: "check", Dice: "1d20", Store: "x"},     // no dc
	}
	for i, e := range bad {
		if errs := validateEffect("t", e); len(errs) == 0 {
			t.Errorf("case %d: expected a validation error for %+v", i, e)
		}
	}
	good := Effect{Op: "check", Dice: "1d20", Store: "x", DC: float64(10), Mods: []any{float64(2)}}
	if errs := validateEffect("t", good); len(errs) != 0 {
		t.Errorf("valid check rejected: %v", errs)
	}
}
