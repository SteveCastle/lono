package cli

import (
	"fmt"
	"time"

	"github.com/callsignmedia/lono/internal/engine"
	"github.com/callsignmedia/lono/internal/store"
)

// stateData builds the canonical {state, actions} payload for the LLM.
// clock is always included as a top-level key. A convenience "log" key holds
// the most recent 10 journal entries so callers see them without navigating
// into state. Callers may pass additional keys via extra (e.g. rolls, fired, warnings).
func stateData(def *engine.Definition, st *engine.State, extra map[string]any) (map[string]any, error) {
	actions, err := engine.AvailableActions(def, st)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"state":         st,
		"actions":       actions,
		"derived":       engine.BuildDerivedView(def, st),
		"beats":         engine.ActiveBeats(def, st),
		"endingReached": engine.EndingsReached(def, st),
		"clock":         st.Clock,
		"log":           lastNLog(st.Log, 10),
	}
	for k, v := range extra {
		out[k] = v
	}
	return out, nil
}

// lastNLog returns the last n entries of log, or all entries if len(log) <= n.
func lastNLog(log []engine.LogEntry, n int) []engine.LogEntry {
	if len(log) <= n {
		return log
	}
	return log[len(log)-n:]
}

// loadDefForInstance loads the instance state and its game definition together.
func loadDefForInstance(s *store.Store, instanceID string) (*engine.Definition, *engine.State, error) {
	st, err := s.LoadState(instanceID)
	if err != nil {
		return nil, nil, err
	}
	def, err := s.LoadDefinition(st.GameID)
	if err != nil {
		return nil, nil, err
	}
	return def, st, nil
}

func newInstanceID(gameID string) string {
	return fmt.Sprintf("%s-%d", gameID, time.Now().UnixNano())
}
