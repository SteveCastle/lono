package engine

import "sort"

// ActiveBeat is a beat currently relevant to narrate.
type ActiveBeat struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Intent string `json:"intent,omitempty"`
}

// ActiveBeats returns the beats whose machineState binding matches, whose guard
// holds, and which are not already delivered (for one-shot beats). Results are
// sorted by id for deterministic output.
func ActiveBeats(def *Definition, st *State) []ActiveBeat {
	delivered := map[string]bool{}
	for _, id := range st.DeliveredBeats {
		delivered[id] = true
	}
	ctx := newEvalCtx(nil, nil)
	ctx.def = def

	var ids []string
	for id := range def.Beats {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := []ActiveBeat{}
	for _, id := range ids {
		b := def.Beats[id]
		if b.MachineState != nil && st.Machines[b.MachineState.Machine] != b.MachineState.State {
			continue
		}
		if b.Guard != nil {
			ok, err := evalGuard(st, ctx, b.Guard)
			if err != nil || !ok {
				continue
			}
		}
		if b.once() && delivered[id] {
			continue
		}
		out = append(out, ActiveBeat{ID: id, Text: b.Text, Intent: b.Intent})
	}
	return out
}

// Ending describes a terminal machine state the game has reached.
type Ending struct {
	Machine     string `json:"machine"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	Intent      string `json:"intent,omitempty"`
}

// EndingsReached reports every global machine currently in a terminal state.
// Sorted by machine name for deterministic output.
func EndingsReached(def *Definition, st *State) []Ending {
	var names []string
	for name := range def.Machines {
		names = append(names, name)
	}
	sort.Strings(names)

	out := []Ending{}
	for _, name := range names {
		m := def.Machines[name]
		if m.Attach != nil {
			continue // endings are reported for global machines
		}
		cur := st.Machines[name]
		if meta, ok := m.StateMeta[cur]; ok && meta.Terminal {
			out = append(out, Ending{Machine: name, State: cur, Description: meta.Description, Intent: meta.Intent})
		}
	}
	return out
}
