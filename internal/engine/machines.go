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
	var out []ActionInfo
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
	} else if !ok {
		info.Enabled, info.Reason = false, "guard not satisfied"
	}
	return info
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
