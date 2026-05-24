package engine

import (
	"fmt"
	"sort"
	"strings"
)

// computeDerived evaluates a derived value against the state. `self` substitutes
// for "$self" in the where-clause (empty for global derived values).
func computeDerived(def *Definition, st *State, spec DerivedSpec, self string) (any, error) {
	verb, attr := splitReduce(spec.Reduce)

	// Resolve endpoint specifiers to concrete string ids ("" = wildcard).
	fromWant, err := resolveEndpointSpec(st, def, spec.Where.From, self)
	if err != nil {
		return nil, fmt.Errorf("derived where.from: %w", err)
	}
	toWant, err := resolveEndpointSpec(st, def, spec.Where.To, self)
	if err != nil {
		return nil, fmt.Errorf("derived where.to: %w", err)
	}

	// "Anchored" = the original spec (before resolution) was non-empty.
	fromAnchored := !isEmptyEndpoint(spec.Where.From)
	toAnchored := !isEmptyEndpoint(spec.Where.To)

	// Gather attribute maps of matching items, and parallel ids for arg*/list.
	var matches []map[string]any
	var ids []string

	switch spec.Over {
	case "relationships":
		for _, r := range st.Relationships {
			if spec.Where.Type != "" && r.Type != spec.Where.Type {
				continue
			}
			if fromWant != "" && r.From != fromWant {
				continue
			}
			if toWant != "" && r.To != toWant {
				continue
			}
			if !attrsMatchWithPath(def, st, spec.Where.Attrs, r.Attrs) {
				continue
			}
			matches = append(matches, r.Attrs)
			// Counterpart rule: return the endpoint OPPOSITE the anchored one.
			// to-anchored → return from; from-anchored → return to; neither → from.
			var counterpart string
			switch {
			case toAnchored && !fromAnchored:
				counterpart = r.From
			case fromAnchored && !toAnchored:
				counterpart = r.To
			default:
				counterpart = r.From
			}
			ids = append(ids, counterpart)
		}
	case "entities":
		// Stable order by id for deterministic arg* results.
		for _, id := range sortedEntityIDs(st) {
			e := st.Entities[id]
			if spec.Where.Type != "" && e.Type != spec.Where.Type {
				continue
			}
			if !attrsMatchWithPath(def, st, spec.Where.Attrs, e.Attrs) {
				continue
			}
			matches = append(matches, e.Attrs)
			ids = append(ids, id)
		}
	default:
		return nil, fmt.Errorf("derived: unknown over %q", spec.Over)
	}

	switch verb {
	case "count":
		return float64(len(matches)), nil
	case "any":
		return len(matches) > 0, nil
	case "list":
		result := make([]any, len(ids))
		for i, id := range ids {
			result[i] = id
		}
		return result, nil
	case "sum":
		var s float64
		for _, m := range matches {
			f, _ := toFloat(m[attr])
			s += f
		}
		return s, nil
	case "min", "max":
		if len(matches) == 0 {
			return float64(0), nil
		}
		best, _ := toFloat(matches[0][attr])
		for _, m := range matches[1:] {
			f, _ := toFloat(m[attr])
			if (verb == "max" && f > best) || (verb == "min" && f < best) {
				best = f
			}
		}
		return best, nil
	case "argmax", "argmin":
		if len(matches) == 0 {
			return "", nil
		}
		bestI := 0
		best, _ := toFloat(matches[0][attr])
		for i := 1; i < len(matches); i++ {
			f, _ := toFloat(matches[i][attr])
			if (verb == "argmax" && f > best) || (verb == "argmin" && f < best) {
				best, bestI = f, i
			}
		}
		return ids[bestI], nil
	default:
		return nil, fmt.Errorf("derived: unknown reduce %q", spec.Reduce)
	}
}

// isEmptyEndpoint reports whether an endpoint spec value is empty/nil/blank.
func isEmptyEndpoint(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}

// resolveEndpointSpec converts a where.from/where.to specifier (any) to a
// concrete string id, or "" meaning "match anything". Handles:
//   - nil or ""         → "" (wildcard)
//   - "$self"           → self
//   - {"$path":"<p>"}  → resolvePath result as string
//   - any other string  → literal id
func resolveEndpointSpec(st *State, def *Definition, spec any, self string) (string, error) {
	if isEmptyEndpoint(spec) {
		return "", nil
	}
	// $path map form: {"$path":"<p>"}
	if m, ok := spec.(map[string]any); ok {
		if p, ok := m["$path"].(string); ok {
			ctx := &evalCtx{def: def}
			val, err := resolvePath(st, ctx, p)
			if err != nil {
				return "", fmt.Errorf("$path %q: %w", p, err)
			}
			s, ok := val.(string)
			if !ok {
				return "", fmt.Errorf("$path %q resolved to %T, need string", p, val)
			}
			return s, nil
		}
		return "", fmt.Errorf("unknown endpoint map form %v", spec)
	}
	s, ok := spec.(string)
	if !ok {
		return "", fmt.Errorf("endpoint must be a string or {\"$path\":\"…\"}, got %T", spec)
	}
	if s == "$self" {
		return self, nil
	}
	return s, nil
}

// resolvePredValue resolves an AttrPred.Value for comparison: {"$path":"p"}
// resolves via resolvePath; anything else passes through unchanged.
func resolvePredValue(def *Definition, st *State, v any) any {
	if m, ok := v.(map[string]any); ok {
		if p, ok := m["$path"].(string); ok {
			ctx := &evalCtx{def: def}
			val, err := resolvePath(st, ctx, p)
			if err != nil {
				return nil
			}
			return val
		}
	}
	return v
}

// attrsMatchWithPath is like attrsMatch but resolves $path in predicate values.
func attrsMatchWithPath(def *Definition, st *State, preds []AttrPred, attrs map[string]any) bool {
	for _, p := range preds {
		left, ok := attrs[p.Attr]
		if !ok {
			return false
		}
		right := resolvePredValue(def, st, p.Value)
		match, err := compareValues(left, p.Op, right)
		if err != nil || !match {
			return false
		}
	}
	return true
}

// splitReduce splits "argmax:attraction" into ("argmax","attraction").
func splitReduce(reduce string) (verb, attr string) {
	if i := strings.IndexByte(reduce, ':'); i >= 0 {
		return reduce[:i], reduce[i+1:]
	}
	return reduce, ""
}

// DerivedView holds computed derived values for the CLI state output: global
// values, and per-entity values (those whose where references $self) keyed by
// entity id.
type DerivedView struct {
	Global   map[string]any            `json:"global,omitempty"`
	ByEntity map[string]map[string]any `json:"byEntity,omitempty"`
}

// isSelfDerived reports whether a derived spec should be computed per-entity
// (i.e. its where clause references "$self" in from or to).
func isSelfDerived(spec DerivedSpec) bool {
	isStrEq := func(v any, want string) bool {
		s, ok := v.(string)
		return ok && s == want
	}
	return isStrEq(spec.Where.From, "$self") || isStrEq(spec.Where.To, "$self")
}

// BuildDerivedView computes every derived value: global ones once, per-entity
// ones for each entity. Computation errors are skipped (the value is omitted).
func BuildDerivedView(def *Definition, st *State) DerivedView {
	view := DerivedView{Global: map[string]any{}, ByEntity: map[string]map[string]any{}}
	for name, spec := range def.Derived {
		if isSelfDerived(spec) {
			for _, id := range sortedEntityIDs(st) {
				v, err := computeDerived(def, st, spec, id)
				if err != nil {
					continue
				}
				if view.ByEntity[id] == nil {
					view.ByEntity[id] = map[string]any{}
				}
				view.ByEntity[id][name] = v
			}
		} else {
			v, err := computeDerived(def, st, spec, "")
			if err != nil {
				continue
			}
			view.Global[name] = v
		}
	}
	return view
}

func sortedEntityIDs(st *State) []string {
	ids := make([]string, 0, len(st.Entities))
	for id := range st.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
