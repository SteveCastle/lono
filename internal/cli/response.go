package cli

import (
	"fmt"
	"time"

	"github.com/callsignmedia/lono/internal/engine"
	"github.com/callsignmedia/lono/internal/store"
)

// stateData builds the canonical {state, actions} payload for the LLM.
// clock is always included as a top-level key. Callers may pass additional
// keys via extra (e.g. rolls, fired, warnings).
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
	}
	for k, v := range extra {
		out[k] = v
	}
	return out, nil
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
