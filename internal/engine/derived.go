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

	// Gather the attribute maps of matching items, and a parallel id for arg*.
	var matches []map[string]any
	var ids []string

	switch spec.Over {
	case "relationships":
		for _, r := range st.Relationships {
			if spec.Where.Type != "" && r.Type != spec.Where.Type {
				continue
			}
			if !endpointMatches(spec.Where.From, r.From, self) || !endpointMatches(spec.Where.To, r.To, self) {
				continue
			}
			if attrsMatch(spec.Where.Attrs, r.Attrs) {
				matches = append(matches, r.Attrs)
				ids = append(ids, r.From)
			}
		}
	case "entities":
		// Stable order by id for deterministic arg* results.
		for _, id := range sortedEntityIDs(st) {
			e := st.Entities[id]
			if spec.Where.Type != "" && e.Type != spec.Where.Type {
				continue
			}
			if attrsMatch(spec.Where.Attrs, e.Attrs) {
				matches = append(matches, e.Attrs)
				ids = append(ids, id)
			}
		}
	default:
		return nil, fmt.Errorf("derived: unknown over %q", spec.Over)
	}

	switch verb {
	case "count":
		return float64(len(matches)), nil
	case "any":
		return len(matches) > 0, nil
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

// splitReduce splits "argmax:attraction" into ("argmax","attraction").
func splitReduce(reduce string) (verb, attr string) {
	if i := strings.IndexByte(reduce, ':'); i >= 0 {
		return reduce[:i], reduce[i+1:]
	}
	return reduce, ""
}

// endpointMatches reports whether a relationship endpoint satisfies a where
// clause endpoint. "" matches anything; "$self" matches the self id.
func endpointMatches(want, actual, self string) bool {
	switch want {
	case "":
		return true
	case "$self":
		return actual == self
	default:
		return actual == want
	}
}

// attrsMatch reports whether all predicates hold against an attribute map.
// A missing attribute fails the predicate (no error).
func attrsMatch(preds []AttrPred, attrs map[string]any) bool {
	for _, p := range preds {
		left, ok := attrs[p.Attr]
		if !ok {
			return false
		}
		match, err := compareValues(left, p.Op, p.Value)
		if err != nil || !match {
			return false
		}
	}
	return true
}

// DerivedView holds computed derived values for the CLI state output: global
// values, and per-entity values (those whose where references $self) keyed by
// entity id.
type DerivedView struct {
	Global   map[string]any            `json:"global,omitempty"`
	ByEntity map[string]map[string]any `json:"byEntity,omitempty"`
}

func isSelfDerived(spec DerivedSpec) bool {
	return spec.Where.From == "$self" || spec.Where.To == "$self"
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
