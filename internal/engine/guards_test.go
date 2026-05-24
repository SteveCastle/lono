package engine

import "testing"

func TestEvalGuard(t *testing.T) {
	st := stateWithData() // day=3, player.health=80, gold=50, trust.aria.player.value=5
	ctx := &evalCtx{}
	cases := []struct {
		name  string
		guard Guard
		want  bool
	}{
		{"gte true", Guard{Target: "world.day", Op: "gte", Value: float64(3)}, true},
		{"gt false", Guard{Target: "world.day", Op: "gt", Value: float64(3)}, false},
		{"inventory gt zero (has_item)", Guard{Target: "inventory.player.gold", Op: "gt", Value: float64(0)}, true},
		{"rel gte", Guard{Target: "rel.trust.aria.player.value", Op: "gte", Value: float64(5)}, true},
		{"in", Guard{Target: "machine.arc.state", Op: "in", Value: []any{"intro", "end"}}, true},
		{"exists missing", Guard{Target: "entity.ghost.health", Op: "exists"}, false},
		{"and", Guard{And: []Guard{
			{Target: "world.day", Op: "gte", Value: float64(1)},
			{Target: "entity.player.health", Op: "gt", Value: float64(0)},
		}}, true},
		{"or", Guard{Or: []Guard{
			{Target: "world.day", Op: "gt", Value: float64(99)},
			{Target: "entity.player.health", Op: "gt", Value: float64(0)},
		}}, true},
		{"not", Guard{Not: &Guard{Target: "world.day", Op: "eq", Value: float64(99)}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalGuard(st, ctx, &c.guard)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestGuardReferencesParam(t *testing.T) {
	if !guardReferencesParam(&Guard{Target: "param.x", Op: "gt", Value: float64(0)}) {
		t.Fatal("should detect param ref")
	}
	if guardReferencesParam(&Guard{And: []Guard{{Target: "world.day", Op: "eq", Value: 1}}}) {
		t.Fatal("should not detect param ref")
	}
}

func TestCompareValues(t *testing.T) {
	cases := []struct {
		left  any
		op    string
		right any
		want  bool
	}{
		{float64(5), "eq", float64(5), true},
		{float64(5), "ne", float64(6), true},
		{float64(5), "gt", float64(4), true},
		{float64(5), "gte", float64(5), true},
		{float64(3), "lt", float64(4), true},
		{float64(3), "lte", float64(3), true},
		{"a", "in", []any{"a", "b"}, true},
		{"c", "in", []any{"a", "b"}, false},
	}
	for _, c := range cases {
		got, err := compareValues(c.left, c.op, c.right)
		if err != nil {
			t.Fatalf("compareValues(%v,%q,%v): %v", c.left, c.op, c.right, err)
		}
		if got != c.want {
			t.Fatalf("compareValues(%v,%q,%v)=%v want %v", c.left, c.op, c.right, got, c.want)
		}
	}
	if _, err := compareValues(1, "bogus", 1); err == nil {
		t.Fatal("expected error for unknown op")
	}
}

func TestExistsSemantics(t *testing.T) {
	st := stateWithData() // player exists with inventory gold=50 (no potion); trust aria->player exists
	ctx := &evalCtx{}
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"entity present", "entity.player.health", true},
		{"entity missing", "entity.ghost.health", false},
		{"inventory held", "inventory.player.gold", true},
		{"inventory absent (count 0)", "inventory.player.potion", false},
		{"relationship present", "rel.trust.aria.player.value", true},
		{"relationship missing", "rel.trust.player.aria.value", false},
		{"world var present", "world.day", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalGuard(st, ctx, &Guard{Target: c.target, Op: "exists"})
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Fatalf("exists %s = %v, want %v", c.target, got, c.want)
			}
		})
	}
}
