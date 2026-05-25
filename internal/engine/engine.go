package engine

import (
	"fmt"
	"sort"
	"time"
)

type ActionResult struct {
	Rolls    []RollResult  `json:"rolls,omitempty"`
	Checks   []CheckResult `json:"checks,omitempty"`
	Fired    []string      `json:"fired,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

// StartInstance creates a new instance, seeds the first-class cast, and then
// applies the definition's setup ops.
func StartInstance(def *Definition, instanceID string, seed int64) (*State, error) {
	st, err := NewInstance(def, instanceID, seed)
	if err != nil {
		return nil, err
	}

	// Seed the first-class cast before running def.Setup so that setup ops can
	// reference cast entities and relationships by id.
	if len(def.Entities) > 0 || len(def.Relationships) > 0 {
		ctx := newEvalCtx(nil, &RNG{state: st.RNGState})
		ctx.def = def
		if err := seedCast(def, st, ctx); err != nil {
			return nil, err
		}
		st.RNGState = ctx.rng.state
	}

	if len(def.Setup) > 0 {
		ctx := newEvalCtx(nil, &RNG{state: st.RNGState})
		ctx.def = def
		for i, e := range def.Setup {
			if err := applyEffect(def, st, ctx, e); err != nil {
				return nil, fmt.Errorf("setup op %d (%s): %w", i, e.Op, err)
			}
		}
		st.RNGState = ctx.rng.state
	}
	return st, nil
}

// seedCast builds and applies the effects implied by def.Entities and
// def.Relationships. Entity ids are processed in sorted order for determinism.
func seedCast(def *Definition, st *State, ctx *evalCtx) error {
	// Sort entity IDs for determinism.
	ids := make([]string, 0, len(def.Entities))
	for id := range def.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		e := def.Entities[id]
		// 1. Create the entity.
		if err := applyEffect(def, st, ctx, Effect{
			Op: "create_entity", EntityType: e.Type, ID: id, Attrs: e.Attrs,
		}); err != nil {
			return fmt.Errorf("cast entity %q (create): %w", id, err)
		}
		// 1a. Seed per-instance description (not carried by create_entity effect).
		if e.Description != "" {
			st.Entities[id].Description = e.Description
		}
		// 2. Add inventory items (sorted for determinism).
		invKeys := make([]string, 0, len(e.Inventory))
		for item := range e.Inventory {
			invKeys = append(invKeys, item)
		}
		sort.Strings(invKeys)
		for _, item := range invKeys {
			n := e.Inventory[item]
			if n <= 0 {
				continue
			}
			if err := applyEffect(def, st, ctx, Effect{
				Op: "add_item", Entity: id, Item: item, Count: n,
			}); err != nil {
				return fmt.Errorf("cast entity %q inventory %q: %w", id, item, err)
			}
		}
		// 3. Equip items (sorted for determinism).
		slotKeys := make([]string, 0, len(e.Equipped))
		for slot := range e.Equipped {
			slotKeys = append(slotKeys, slot)
		}
		sort.Strings(slotKeys)
		for _, slot := range slotKeys {
			item := e.Equipped[slot]
			if item == "" {
				continue
			}
			if err := applyEffect(def, st, ctx, Effect{
				Op: "equip", Entity: id, Slot: slot, Item: item,
			}); err != nil {
				return fmt.Errorf("cast entity %q equip slot %q: %w", id, slot, err)
			}
		}
	}

	// 4. Create relationships.
	for i, r := range def.Relationships {
		if err := applyEffect(def, st, ctx, Effect{
			Op: "set_relationship", RelType: r.Type, From: r.From, To: r.To, Attrs: r.Attrs,
		}); err != nil {
			return fmt.Errorf("cast relationship[%d] (%s %s->%s): %w", i, r.Type, r.From, r.To, err)
		}
	}
	return nil
}

// validateParams checks supplied params against the transition's declared params.
func validateParams(decl map[string]VarSpec, params map[string]any) error {
	for name, spec := range decl {
		v, ok := params[name]
		if !ok {
			return fmt.Errorf("missing required param %q", name)
		}
		if err := ValidateValue(spec, v); err != nil {
			return fmt.Errorf("param %q: %w", name, err)
		}
	}
	for name := range params {
		if _, ok := decl[name]; !ok {
			return fmt.Errorf("unknown param %q", name)
		}
	}
	return nil
}

// PerformAction validates and applies an action atomically, returning a new
// state. The input state is never mutated.
func PerformAction(def *Definition, st *State, machine, action string, params map[string]any) (*State, *ActionResult, error) {
	_, tr, err := findTransition(def, st, machine, action)
	if err != nil {
		return nil, nil, err
	}
	if err := validateParams(tr.Params, params); err != nil {
		return nil, nil, err
	}
	work := st.Clone()
	ctx := newEvalCtx(params, &RNG{state: work.RNGState})
	ctx.def = def

	ok, err := evalGuard(work, ctx, tr.Guard)
	if err != nil {
		return nil, nil, fmt.Errorf("guard error: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("guard not satisfied for action %q", action)
	}
	for i, e := range tr.Effects {
		if err := applyEffect(def, work, ctx, e); err != nil {
			return nil, nil, fmt.Errorf("effect %d (%s): %w", i, e.Op, err)
		}
	}
	work.Machines[machine] = tr.To
	sr := Settle(def, work, ctx)
	work.RNGState = ctx.rng.state
	work.UpdatedAt = time.Now().UTC()
	work.History = append(work.History, HistoryEntry{
		Seq: len(work.History) + 1, TS: work.UpdatedAt, Kind: "action",
		Machine: machine, Action: action, Params: params, Rolls: ctx.record,
	})
	return work, &ActionResult{Rolls: ctx.record, Checks: ctx.checkLog, Fired: sr.Fired, Warnings: sr.Warnings}, nil
}

// HostRef identifies the host instance for an attached-machine action.
type HostRef struct {
	Kind string `json:"kind"` // "entity" | "relationship"
	ID   string `json:"id,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// PerformHostAction performs an attached-machine transition on a specific host
// (entity or relationship), atomically. The input state is never mutated.
func PerformHostAction(def *Definition, st *State, machine, action string, params map[string]any, host *HostRef) (*State, *ActionResult, error) {
	m, ok := def.Machines[machine]
	if !ok || m.Attach == nil {
		return nil, nil, fmt.Errorf("%q is not an attached machine", machine)
	}
	if host == nil {
		return nil, nil, fmt.Errorf("attached machine %q requires a host", machine)
	}
	work := st.Clone()

	// Resolve the host within the clone and build its hostRef + current state.
	hostCtx := &hostRef{kind: host.Kind}
	var cur string
	switch host.Kind {
	case "entity":
		e, ok := work.Entities[host.ID]
		if !ok {
			return nil, nil, fmt.Errorf("unknown entity %q", host.ID)
		}
		hostCtx.id, hostCtx.ent = host.ID, e
		cur = e.Machines[machine]
	case "relationship":
		r := findRelationship(work, machineRelType(m), host.From, host.To)
		if r == nil {
			return nil, nil, fmt.Errorf("no %s relationship %s->%s", machineRelType(m), host.From, host.To)
		}
		hostCtx.rel = r
		cur = r.Machines[machine]
	default:
		return nil, nil, fmt.Errorf("bad host kind %q", host.Kind)
	}

	tr, ok := transitionFrom(m, action, cur)
	if !ok {
		return nil, nil, fmt.Errorf("action %q not available from state %q", action, cur)
	}
	if err := validateParams(tr.Params, params); err != nil {
		return nil, nil, err
	}
	ctx := newEvalCtx(params, &RNG{state: work.RNGState})
	ctx.def = def
	ctx.host = hostCtx

	ok2, err := evalGuard(work, ctx, tr.Guard)
	if err != nil {
		return nil, nil, fmt.Errorf("guard error: %w", err)
	}
	if !ok2 {
		return nil, nil, fmt.Errorf("guard not satisfied for action %q", action)
	}
	for i, e := range tr.Effects {
		if err := applyEffect(def, work, ctx, e); err != nil {
			return nil, nil, fmt.Errorf("effect %d (%s): %w", i, e.Op, err)
		}
	}
	// Advance the host's machine state.
	if host.Kind == "entity" {
		hostCtx.ent.Machines[machine] = tr.To
	} else {
		hostCtx.rel.Machines[machine] = tr.To
	}
	sr := Settle(def, work, ctx)
	work.RNGState = ctx.rng.state
	work.UpdatedAt = time.Now().UTC()
	work.History = append(work.History, HistoryEntry{
		Seq: len(work.History) + 1, TS: work.UpdatedAt, Kind: "action",
		Machine: machine, Action: action, Params: params, Rolls: ctx.record,
	})
	return work, &ActionResult{Rolls: ctx.record, Checks: ctx.checkLog, Fired: sr.Fired, Warnings: sr.Warnings}, nil
}

// transitionFrom finds a transition by id whose From matches the current state.
func transitionFrom(m Machine, action, cur string) (Transition, bool) {
	for _, tr := range m.Transitions {
		if tr.ID == action && tr.From.Matches(cur) {
			return tr, true
		}
	}
	return Transition{}, false
}

// SettleInstance runs the reactive trigger fixpoint on st in place (used after a
// raw --force write so consequences still react). Returns fired triggers + warnings.
func SettleInstance(def *Definition, st *State) *ActionResult {
	ctx := newEvalCtx(nil, &RNG{state: st.RNGState})
	ctx.def = def
	res := Settle(def, st, ctx)
	st.RNGState = ctx.rng.state
	return &ActionResult{Fired: res.Fired, Warnings: res.Warnings}
}

// ApplyOps applies an ad-hoc list of effect ops atomically (the "send updates"
// path). The input state is never mutated.
func ApplyOps(def *Definition, st *State, ops []Effect) (*State, *ActionResult, error) {
	work := st.Clone()
	ctx := newEvalCtx(nil, &RNG{state: work.RNGState})
	ctx.def = def
	for i, e := range ops {
		if err := applyEffect(def, work, ctx, e); err != nil {
			return nil, nil, fmt.Errorf("op %d (%s): %w", i, e.Op, err)
		}
	}
	sr := Settle(def, work, ctx)
	work.RNGState = ctx.rng.state
	work.UpdatedAt = time.Now().UTC()
	work.History = append(work.History, HistoryEntry{
		Seq: len(work.History) + 1, TS: work.UpdatedAt, Kind: "apply", Rolls: ctx.record,
	})
	return work, &ActionResult{Rolls: ctx.record, Checks: ctx.checkLog, Fired: sr.Fired, Warnings: sr.Warnings}, nil
}
