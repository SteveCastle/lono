package engine

import (
	"fmt"
	"strings"
)

// evalGuard evaluates a guard tree. A nil guard is vacuously true.
func evalGuard(st *State, ctx *evalCtx, g *Guard) (bool, error) {
	if g == nil {
		return true, nil
	}
	switch {
	case len(g.And) > 0:
		for i := range g.And {
			ok, err := evalGuard(st, ctx, &g.And[i])
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case len(g.Or) > 0:
		for i := range g.Or {
			ok, err := evalGuard(st, ctx, &g.Or[i])
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case g.Not != nil:
		ok, err := evalGuard(st, ctx, g.Not)
		return !ok, err
	default:
		return evalLeaf(st, ctx, g)
	}
}

func evalLeaf(st *State, ctx *evalCtx, g *Guard) (bool, error) {
	// exists: true iff the path refers to something present (see pathExists).
	if g.Op == "exists" {
		return pathExists(st, ctx, g.Target), nil
	}
	// contains: target must be a []any; returns whether g.Value (string) is a member.
	if g.Op == "contains" {
		v, err := resolvePath(st, ctx, g.Target)
		if err != nil {
			return false, err
		}
		arr, ok := v.([]any)
		if !ok {
			return false, fmt.Errorf("contains: target %q is not a set/array", g.Target)
		}
		want, ok := g.Value.(string)
		if !ok {
			return false, fmt.Errorf("contains: value must be a string, got %T", g.Value)
		}
		for _, item := range arr {
			if item == want {
				return true, nil
			}
		}
		return false, nil
	}
	left, err := resolvePath(st, ctx, g.Target)
	if err != nil {
		return false, err
	}
	return compareValues(left, g.Op, g.Value)
}

// compareValues applies a comparison op to two values. Numeric ops coerce via
// toFloat; eq/ne use equalValues; in expects a []any right-hand side. Shared by
// guard leaves and derived-value attribute predicates.
func compareValues(left any, op string, right any) (bool, error) {
	switch op {
	case "eq":
		return equalValues(left, right), nil
	case "ne":
		return !equalValues(left, right), nil
	case "gt", "gte", "lt", "lte":
		l, lok := toFloat(left)
		r, rok := toFloat(right)
		if !lok || !rok {
			return false, fmt.Errorf("op %q needs numbers, got %v and %v", op, left, right)
		}
		switch op {
		case "gt":
			return l > r, nil
		case "gte":
			return l >= r, nil
		case "lt":
			return l < r, nil
		case "lte":
			return l <= r, nil
		}
	case "in":
		list, ok := right.([]any)
		if !ok {
			return false, fmt.Errorf("op in needs a list value")
		}
		for _, item := range list {
			if equalValues(left, item) {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("unknown op %q", op)
}

func equalValues(a, b any) bool {
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			return af == bf
		}
		return false
	}
	return a == b
}

// guardReferencesParam reports whether any leaf reads a param.* path. Used to
// decide whether an action can be listed as available without params.
func guardReferencesParam(g *Guard) bool {
	if g == nil {
		return false
	}
	if strings.HasPrefix(g.Target, "param.") {
		return true
	}
	for i := range g.And {
		if guardReferencesParam(&g.And[i]) {
			return true
		}
	}
	for i := range g.Or {
		if guardReferencesParam(&g.Or[i]) {
			return true
		}
	}
	return guardReferencesParam(g.Not)
}
