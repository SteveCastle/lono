package engine

import "fmt"

type ActionInfo struct {
	Machine        string             `json:"machine"`
	Action         string             `json:"action"`
	From           string             `json:"from"`
	To             string             `json:"to"`
	Host           *HostRef           `json:"host,omitempty"`
	Enabled        bool               `json:"enabled"`
	RequiresParams bool               `json:"requiresParams,omitempty"`
	Params         map[string]VarSpec `json:"params,omitempty"`
	Reason         string             `json:"reason,omitempty"`
}

// AvailableActions lists every transition whose From matches its machine's
// current state. Param-independent guards are evaluated now; param-gated guards
// are deferred to PerformAction and reported via RequiresParams.
func AvailableActions(def *Definition, st *State) ([]ActionInfo, error) {
	out := []ActionInfo{} // always a JSON array, never null, even with no actions
	for mName, m := range def.Machines {
		if m.Attach != nil {
			out = append(out, attachedActions(def, st, mName, m)...)
			continue
		}
		cur := st.Machines[mName]
		for _, tr := range m.Transitions {
			if !tr.From.Matches(cur) {
				continue
			}
			out = append(out, buildActionInfo(def, st, mName, cur, tr, nil))
		}
	}
	return out, nil
}

// buildActionInfo evaluates a transition's availability for output. host is nil
// for global machines, or the bound host for attached machines.
func buildActionInfo(def *Definition, st *State, mName, cur string, tr Transition, host *hostRef) ActionInfo {
	info := ActionInfo{Machine: mName, Action: tr.ID, From: cur, To: tr.To, Params: tr.Params, Enabled: true}
	if host != nil {
		info.Host = hostRefToPublic(host)
	}
	if guardReferencesParam(tr.Guard) {
		info.RequiresParams = true
		return info
	}
	ctx := newEvalCtx(nil, nil)
	ctx.def = def
	ctx.host = host
	ok, err := evalGuard(st, ctx, tr.Guard)
	if err != nil {
		info.Enabled, info.Reason = false, err.Error()
		return info
	}
	if !ok {
		info.Enabled, info.Reason = false, "guard not satisfied"
		return info
	}
	// Effect-aware enabledness: the guard alone can pass while an effect would
	// fail (e.g. a `move via:"exit"` with no edge, or a locked door). Dry-run the
	// effects against a throwaway clone so we never advertise a move the engine
	// would reject. Skipped for actions that take params (we can't supply them
	// here) — those stay guard-only + RequiresParams.
	if len(tr.Params) == 0 {
		if feasible, reason := transitionFeasible(def, st, tr, host); !feasible {
			info.Enabled, info.Reason = false, reason
		}
	}
	return info
}

// transitionFeasible reports whether tr's effects can be applied against the
// current state without error. It runs them on a deep clone with a feasibility
// RNG seeded from the live RNG state (so any `roll` matches what real execution
// would draw), and discards the clone — the live state is never mutated.
func transitionFeasible(def *Definition, st *State, tr Transition, host *hostRef) (bool, string) {
	if len(tr.Effects) == 0 {
		return true, ""
	}
	work := st.Clone()
	ctx := newEvalCtx(nil, &RNG{state: work.RNGState})
	ctx.def = def
	if host != nil {
		ctx.host = rebindHost(work, host)
		if ctx.host == nil {
			return true, "" // host vanished in the clone; don't block on that
		}
	}
	for _, e := range tr.Effects {
		if err := applyEffect(def, work, ctx, e); err != nil {
			return false, err.Error()
		}
	}
	return true, ""
}

// rebindHost re-resolves a host into a cloned state, so `this.*` effect writes
// land on the clone rather than the live entity/relationship.
func rebindHost(work *State, h *hostRef) *hostRef {
	switch h.kind {
	case "entity":
		e, ok := work.Entities[h.id]
		if !ok {
			return nil
		}
		return &hostRef{kind: "entity", id: h.id, ent: e}
	case "relationship":
		r := findRelationship(work, h.rel.Type, h.rel.From, h.rel.To)
		if r == nil {
			return nil
		}
		return &hostRef{kind: "relationship", rel: r}
	}
	return nil
}

// attachedActions enumerates the available transitions for every host instance
// of an attached machine.
func attachedActions(def *Definition, st *State, mName string, m Machine) []ActionInfo {
	var out []ActionInfo
	kind, typ := m.Attach.AttachKind()
	switch kind {
	case "entityType":
		for _, id := range sortedEntityIDs(st) {
			e := st.Entities[id]
			if e.Type != typ {
				continue
			}
			host := &hostRef{kind: "entity", id: id, ent: e}
			cur := e.Machines[mName]
			for _, tr := range m.Transitions {
				if tr.From.Matches(cur) {
					out = append(out, buildActionInfo(def, st, mName, cur, tr, host))
				}
			}
		}
	case "relationshipType":
		for _, r := range st.Relationships {
			if r.Type != typ {
				continue
			}
			host := &hostRef{kind: "relationship", rel: r}
			cur := r.Machines[mName]
			for _, tr := range m.Transitions {
				if tr.From.Matches(cur) {
					out = append(out, buildActionInfo(def, st, mName, cur, tr, host))
				}
			}
		}
	}
	return out
}

// hostRefToPublic converts the internal hostRef to the exported HostRef for output.
func hostRefToPublic(h *hostRef) *HostRef {
	if h.kind == "entity" {
		return &HostRef{Kind: "entity", ID: h.id}
	}
	return &HostRef{Kind: "relationship", From: h.rel.From, To: h.rel.To}
}

// findTransition returns the transition with id `action` in `machine` whose
// From matches the current state, or an error.
func findTransition(def *Definition, st *State, machine, action string) (Machine, Transition, error) {
	m, ok := def.Machines[machine]
	if !ok {
		return Machine{}, Transition{}, fmt.Errorf("unknown machine %q", machine)
	}
	if m.Attach != nil {
		return Machine{}, Transition{}, fmt.Errorf("machine %q is attached; address a host with do --entity/--rel", machine)
	}
	cur := st.Machines[machine]
	for _, tr := range m.Transitions {
		if tr.ID == action {
			if !tr.From.Matches(cur) {
				return Machine{}, Transition{}, fmt.Errorf("action %q not available from state %q", action, cur)
			}
			return m, tr, nil
		}
	}
	return Machine{}, Transition{}, fmt.Errorf("unknown action %q in machine %q", action, machine)
}
