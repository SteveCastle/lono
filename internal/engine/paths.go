package engine

import (
	"fmt"
	"strings"
)

// evalCtx carries per-action context for guards and effects.
type evalCtx struct {
	def      *Definition
	host     *hostRef
	params   map[string]any
	rolls    map[string]float64
	checks   map[string]CheckResult
	rng      *RNG
	record   []RollResult
	checkLog []CheckResult
}

// hostRef binds `this.*` to the relationship or entity an attached-machine
// transition is operating on.
type hostRef struct {
	kind string // "relationship" | "entity"
	id   string // entity id (entity host)
	ent  *Entity
	rel  *Relationship
}

func newEvalCtx(params map[string]any, rng *RNG) *evalCtx {
	return &evalCtx{params: params, rolls: map[string]float64{}, checks: map[string]CheckResult{}, rng: rng}
}

// splitPath splits a dotted path into its segments.
func splitPath(path string) []string { return strings.Split(path, ".") }

// pathExists reports whether a path refers to something present, for the
// `exists` guard op. Missing entities/relationships/world vars/params are
// absent. Inventory paths always resolve (missing items read as 0), so for
// inventory "exists" means "holds at least one" (count > 0).
func pathExists(st *State, ctx *evalCtx, path string) bool {
	v, err := resolvePath(st, ctx, path)
	if err != nil {
		return false
	}
	if strings.HasPrefix(path, "inventory.") {
		f, _ := toFloat(v)
		return f > 0
	}
	if strings.HasPrefix(path, "equipped.") {
		s, _ := v.(string)
		return s != ""
	}
	return true
}

// resolveDerived computes a derived value by name. self is "" for a global read
// or the entity id for a per-entity read.
func resolveDerived(ctx *evalCtx, st *State, name, self string) (any, error) {
	if ctx == nil || ctx.def == nil {
		return nil, fmt.Errorf("derived %q not available in this context", name)
	}
	spec, ok := ctx.def.Derived[name]
	if !ok {
		return nil, fmt.Errorf("unknown derived value %q", name)
	}
	isSelf := spec.Where.From == "$self" || spec.Where.To == "$self"
	if isSelf && self == "" {
		return nil, fmt.Errorf("derived %q is per-entity; read it as entity.<id>.derived.%s", name, name)
	}
	return computeDerived(ctx.def, st, spec, self)
}

// resolvePath reads a dotted path against the state. Returns an error if the
// path is malformed or points at a missing entity/relationship.
func resolvePath(st *State, ctx *evalCtx, path string) (any, error) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	switch parts[0] {
	case "clock":
		// clock → current tick count as float64.
		if len(parts) != 1 {
			return nil, fmt.Errorf("bad clock path %q: expected just \"clock\"", path)
		}
		return float64(st.Clock), nil
	case "cooldown":
		// cooldown.<key> → remaining ticks until expiry (0 if already expired or absent).
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad cooldown path %q: expected cooldown.<key>", path)
		}
		due := st.Cooldowns[parts[1]]
		remaining := due - st.Clock
		if remaining < 0 {
			remaining = 0
		}
		return float64(remaining), nil
	case "roll":
		// roll.<store> reads a stored roll result from ctx.
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad roll path %q: expected roll.<store>", path)
		}
		if ctx == nil {
			return nil, fmt.Errorf("roll path %q not available outside an action context", path)
		}
		v, ok := ctx.rolls[parts[1]]
		if !ok {
			return nil, fmt.Errorf("no stored roll %q", parts[1])
		}
		return v, nil
	case "check":
		// check.<store>.<field> reads a stored skill-check result from ctx.
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad check path %q: expected check.<store>.<field>", path)
		}
		if ctx == nil {
			return nil, fmt.Errorf("check path %q not available outside an action context", path)
		}
		cr, ok := ctx.checks[parts[1]]
		if !ok {
			return nil, fmt.Errorf("no stored check %q", parts[1])
		}
		switch parts[2] {
		case "success":
			return cr.Success, nil
		case "margin":
			return cr.Margin, nil
		case "total":
			return cr.Total, nil
		case "roll":
			return cr.Roll, nil
		case "dc":
			return cr.DC, nil
		default:
			return nil, fmt.Errorf("unknown check field %q (want success|margin|total|roll|dc)", parts[2])
		}
	case "len":
		if len(parts) < 2 {
			return nil, fmt.Errorf("bad len path %q: missing inner path", path)
		}
		innerPath := strings.Join(parts[1:], ".")
		inner, err := resolvePath(st, ctx, innerPath)
		if err != nil {
			return nil, fmt.Errorf("len: %w", err)
		}
		arr, ok := inner.([]any)
		if !ok {
			return nil, fmt.Errorf("len: %q is not an array (got %T)", innerPath, inner)
		}
		return float64(len(arr)), nil
	case "this":
		return resolveThis(st, ctx, parts)
	case "world":
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad world path %q", path)
		}
		v, ok := st.World[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown world var %q", parts[1])
		}
		return v, nil
	case "derived":
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad derived path %q", path)
		}
		return resolveDerived(ctx, st, parts[1], "")
	case "entity":
		e, ok := st.Entities[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown entity %q", parts[1])
		}
		if len(parts) == 4 && parts[2] == "derived" {
			return resolveDerived(ctx, st, parts[3], parts[1])
		}
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad entity path %q", path)
		}
		v, ok := e.Attrs[parts[2]]
		if !ok {
			return nil, fmt.Errorf("unknown attr %q on %q", parts[2], parts[1])
		}
		return v, nil
	case "inventory":
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad inventory path %q", path)
		}
		e, ok := st.Entities[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown entity %q", parts[1])
		}
		return float64(e.Inventory[parts[2]]), nil // missing item => 0
	case "rel":
		if len(parts) != 5 {
			return nil, fmt.Errorf("bad rel path %q", path)
		}
		r := findRelationship(st, parts[1], parts[2], parts[3])
		if r == nil {
			return nil, fmt.Errorf("no relationship %s %s->%s", parts[1], parts[2], parts[3])
		}
		v, ok := r.Attrs[parts[4]]
		if !ok {
			return nil, fmt.Errorf("unknown rel attr %q", parts[4])
		}
		return v, nil
	case "machine":
		if len(parts) != 3 || parts[2] != "state" {
			return nil, fmt.Errorf("bad machine path %q", path)
		}
		s, ok := st.Machines[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown machine %q", parts[1])
		}
		return s, nil
	case "param":
		if len(parts) != 2 || ctx == nil {
			return nil, fmt.Errorf("bad param path %q", path)
		}
		v, ok := ctx.params[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown param %q", parts[1])
		}
		return v, nil
	case "equipped":
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad equipped path %q", path)
		}
		e, ok := st.Entities[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown entity %q", parts[1])
		}
		return e.Equipped[parts[2]], nil // "" if slot empty
	case "worn":
		if len(parts) != 4 {
			return nil, fmt.Errorf("bad worn path %q", path)
		}
		e, ok := st.Entities[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown entity %q", parts[1])
		}
		item := e.Equipped[parts[2]]
		if item == "" {
			return nil, fmt.Errorf("slot %q on %q is empty", parts[2], parts[1])
		}
		return itemTypeAttr(ctx, item, parts[3])
	case "itemtype":
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad itemtype path %q", path)
		}
		return itemTypeAttr(ctx, parts[1], parts[2])
	default:
		return nil, fmt.Errorf("unknown path root %q", parts[0])
	}
}

// itemTypeAttr reads a static attribute off an item type (needs ctx.def).
func itemTypeAttr(ctx *evalCtx, item, attr string) (any, error) {
	if ctx == nil || ctx.def == nil {
		return nil, fmt.Errorf("item type attributes not available in this context")
	}
	it, ok := ctx.def.ItemTypes[item]
	if !ok {
		return nil, fmt.Errorf("unknown item type %q", item)
	}
	v, ok := it.Attributes[attr]
	if !ok {
		return nil, fmt.Errorf("item type %q has no attribute %q", item, attr)
	}
	return v, nil
}

func resolveThis(st *State, ctx *evalCtx, parts []string) (any, error) {
	if ctx == nil || ctx.host == nil {
		return nil, fmt.Errorf("this.* used outside an attached-machine context")
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("bad this path")
	}
	h := ctx.host
	switch h.kind {
	case "entity":
		switch parts[1] {
		case "id":
			return h.id, nil
		case "inventory":
			if len(parts) != 3 {
				return nil, fmt.Errorf("bad this.inventory path")
			}
			return float64(h.ent.Inventory[parts[2]]), nil
		case "machine":
			if len(parts) != 3 {
				return nil, fmt.Errorf("bad this.machine path")
			}
			s, ok := h.ent.Machines[parts[2]]
			if !ok {
				return nil, fmt.Errorf("unknown attached machine %q", parts[2])
			}
			return s, nil
		case "equipped":
			if len(parts) != 3 {
				return nil, fmt.Errorf("bad this.equipped path")
			}
			return h.ent.Equipped[parts[2]], nil
		default:
			v, ok := h.ent.Attrs[parts[1]]
			if !ok {
				return nil, fmt.Errorf("unknown attr %q on this", parts[1])
			}
			return v, nil
		}
	case "relationship":
		switch parts[1] {
		case "from":
			if len(parts) == 2 {
				return h.rel.From, nil
			}
			return endpointAttr(st, h.rel.From, parts[2])
		case "to":
			if len(parts) == 2 {
				return h.rel.To, nil
			}
			return endpointAttr(st, h.rel.To, parts[2])
		case "machine":
			if len(parts) != 3 {
				return nil, fmt.Errorf("bad this.machine path")
			}
			s, ok := h.rel.Machines[parts[2]]
			if !ok {
				return nil, fmt.Errorf("unknown attached machine %q", parts[2])
			}
			return s, nil
		default:
			v, ok := h.rel.Attrs[parts[1]]
			if !ok {
				return nil, fmt.Errorf("unknown rel attr %q on this", parts[1])
			}
			return v, nil
		}
	}
	return nil, fmt.Errorf("bad host kind %q", h.kind)
}

func endpointAttr(st *State, id, attr string) (any, error) {
	e, ok := st.Entities[id]
	if !ok {
		return nil, fmt.Errorf("unknown endpoint entity %q", id)
	}
	v, ok := e.Attrs[attr]
	if !ok {
		return nil, fmt.Errorf("unknown attr %q on %q", attr, id)
	}
	return v, nil
}

func findRelationship(st *State, relType, from, to string) *Relationship {
	for _, r := range st.Relationships {
		if r.Type == relType && r.From == from && r.To == to {
			return r
		}
	}
	return nil
}
