package engine

import "sort"

// SettleResult reports what happened during a Settle call.
type SettleResult struct {
	Fired    []string
	Warnings []string
}

// Settle fires non-periodic triggers to a fixpoint after a mutation.
//
// Edge-triggered: a When-trigger fires only on a rising edge (guard
// false→true), tracked via st.TriggerArmed. A trigger whose guard is
// currently true AND was already armed does not re-fire. When the guard
// goes false during a settle, armed is reset so a future rise can fire again.
//
// once:true (the default) fires the trigger at most once per instance
// (tracked via st.TriggerFired). once:false re-fires on every rising edge.
//
// Periodic triggers (Every>0) are not fired here; they fire during Advance.
//
// The algorithm is capped at 100 iterations; if the fixpoint is not reached
// within that budget, a "TRIGGER_LOOP" warning is appended and settling stops.
func Settle(def *Definition, st *State, ctx *evalCtx) SettleResult {
	if st.TriggerArmed == nil {
		st.TriggerArmed = map[string]bool{}
	}
	if st.TriggerFired == nil {
		st.TriggerFired = map[string]bool{}
	}

	// Collect non-periodic trigger ids and sort for determinism.
	var ids []string
	for id, trig := range def.Triggers {
		if trig.Every == 0 && trig.When != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	const iterCap = 100
	var fired []string
	var warnings []string

	settled := false
	for iter := 0; iter < iterCap; iter++ {
		progressed := false
		for _, id := range ids {
			trig := def.Triggers[id]

			held, err := evalGuard(st, ctx, trig.When)
			if err != nil {
				// Treat guard errors as guard-not-held; disarm.
				held = false
			}

			if !held {
				// Guard is false: disarm so the next true→false→true edge fires.
				st.TriggerArmed[id] = false
				continue
			}

			// Guard is true.
			if st.TriggerArmed[id] {
				// Already counted as currently-true (armed); skip.
				continue
			}

			// Guard was previously false (or this is the first settle); rising edge.
			if trig.once() && st.TriggerFired[id] {
				// Once trigger already fired: arm (guard is held) but do not re-fire.
				st.TriggerArmed[id] = true
				continue
			}

			// Fire: apply each effect. On error record a warning and stop this
			// trigger's remaining effects, but continue settling others.
			for _, ef := range trig.Effects {
				if err := applyEffect(def, st, ctx, ef); err != nil {
					warnings = append(warnings, "trigger "+id+": "+err.Error())
					break
				}
			}

			st.TriggerFired[id] = true
			st.TriggerArmed[id] = true
			fired = append(fired, id)
			progressed = true
		}
		if !progressed {
			settled = true
			break
		}
	}

	if !settled {
		warnings = append(warnings, "TRIGGER_LOOP")
	}

	return SettleResult{Fired: fired, Warnings: warnings}
}
