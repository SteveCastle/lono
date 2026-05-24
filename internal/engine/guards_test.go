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

func stateWithSetAttr() *State {
	st, _ := NewInstance(miniDef(), "r", 1)
	st.Entities["player"] = &Entity{
		Type:      "character",
		Attrs:     map[string]any{"health": float64(80), "clues": []any{"alibi", "motive"}},
		Inventory: map[string]int{},
	}
	return st
}

func TestContainsGuard(t *testing.T) {
	st := stateWithSetAttr()
	ctx := &evalCtx{}

	// contains: present element -> true.
	g := Guard{Target: "entity.player.clues", Op: "contains", Value: "alibi"}
	got, err := evalGuard(st, ctx, &g)
	if err != nil {
		t.Fatalf("contains err: %v", err)
	}
	if !got {
		t.Fatal("contains present element should return true")
	}

	// contains: absent element -> false.
	g2 := Guard{Target: "entity.player.clues", Op: "contains", Value: "weapon"}
	got2, err := evalGuard(st, ctx, &g2)
	if err != nil {
		t.Fatalf("contains err: %v", err)
	}
	if got2 {
		t.Fatal("contains absent element should return false")
	}

	// contains: non-array target -> error.
	g3 := Guard{Target: "entity.player.health", Op: "contains", Value: "x"}
	if _, err := evalGuard(st, ctx, &g3); err == nil {
		t.Fatal("contains on non-array should error")
	}
}

func TestLenPathGuard(t *testing.T) {
	st := stateWithSetAttr() // player.clues = ["alibi","motive"]
	ctx := &evalCtx{}

	// len.entity.player.clues should be 2.
	g := Guard{Target: "len.entity.player.clues", Op: "gte", Value: float64(2)}
	got, err := evalGuard(st, ctx, &g)
	if err != nil {
		t.Fatalf("len guard err: %v", err)
	}
	if !got {
		t.Fatal("len gte 2 should be true for 2-element set")
	}

	// len == 1 should be false for 2-element set.
	g2 := Guard{Target: "len.entity.player.clues", Op: "eq", Value: float64(1)}
	got2, err := evalGuard(st, ctx, &g2)
	if err != nil {
		t.Fatalf("len guard err: %v", err)
	}
	if got2 {
		t.Fatal("len eq 1 should be false for 2-element set")
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
