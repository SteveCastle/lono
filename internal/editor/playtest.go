package editor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/callsignmedia/lono/internal/engine"
)

// session is an ephemeral, in-memory playtest of a definition. Sessions live
// only for the editor process — they are never persisted.
type session struct {
	def  *engine.Definition
	st   *engine.State
	born time.Time
}

const maxSessions = 64

// startRequest begins a playtest from a posted definition.
type startRequest struct {
	Definition json.RawMessage `json:"definition"`
	Seed       int64           `json:"seed"`
}

func (s *Server) handlePlaytestStart(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := decodeBody(r, &req); err != nil || len(req.Definition) == 0 {
		writeErr(w, http.StatusBadRequest, "body must be {\"definition\": {...}, \"seed\": n}")
		return
	}
	var def engine.Definition
	if err := json.Unmarshal(req.Definition, &def); err != nil {
		writeErr(w, http.StatusBadRequest, "definition is not valid JSON: %v", err)
		return
	}
	if errs := engine.ValidateDefinition(&def); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":      "definition is invalid — fix validation errors before playtesting",
			"validation": errs,
		})
		return
	}
	st, err := engine.StartInstance(&def, "playtest", req.Seed)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "start failed: %v", err)
		return
	}

	s.mu.Lock()
	// Keep the session table bounded: drop the oldest if at capacity.
	if len(s.sessions) >= maxSessions {
		var oldestID string
		var oldest time.Time
		for id, sess := range s.sessions {
			if oldestID == "" || sess.born.Before(oldest) {
				oldestID, oldest = id, sess.born
			}
		}
		delete(s.sessions, oldestID)
	}
	s.seq++
	id := fmt.Sprintf("pt%d", s.seq)
	s.sessions[id] = &session{def: &def, st: st, born: time.Now()}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"session": id, "view": buildView(&def, st)})
}

// getSession returns the live session for id, or writes a 404 and returns nil.
func (s *Server) getSession(w http.ResponseWriter, id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[id]
	if sess == nil {
		writeErr(w, http.StatusNotFound, "no playtest session %q (it may have expired — start a new one)", id)
		return nil
	}
	return sess
}

// actRequest performs a transition (global or attached) in a playtest.
type actRequest struct {
	Machine string          `json:"machine"`
	Action  string          `json:"action"`
	Params  map[string]any  `json:"params,omitempty"`
	Host    *engine.HostRef `json:"host,omitempty"`
}

func (s *Server) handlePlaytestAct(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(w, r.PathValue("session"))
	if sess == nil {
		return
	}
	var req actRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad body: %v", err)
		return
	}
	var (
		next *engine.State
		res  *engine.ActionResult
		err  error
	)
	if req.Host != nil {
		next, res, err = engine.PerformHostAction(sess.def, sess.st, req.Machine, req.Action, req.Params, req.Host)
	} else {
		next, res, err = engine.PerformAction(sess.def, sess.st, req.Machine, req.Action, req.Params)
	}
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	s.commit(r.PathValue("session"), next)
	writeJSON(w, http.StatusOK, map[string]any{"view": buildView(sess.def, next), "result": res})
}

type advanceRequest struct {
	N int `json:"n"`
}

func (s *Server) handlePlaytestAdvance(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(w, r.PathValue("session"))
	if sess == nil {
		return
	}
	var req advanceRequest
	_ = decodeBody(r, &req)
	if req.N <= 0 {
		req.N = 1
	}
	next, res, err := engine.AdvanceInstance(sess.def, sess.st, req.N)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	s.commit(r.PathValue("session"), next)
	writeJSON(w, http.StatusOK, map[string]any{"view": buildView(sess.def, next), "result": res})
}

type applyRequest struct {
	Ops []engine.Effect `json:"ops"`
}

func (s *Server) handlePlaytestApply(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(w, r.PathValue("session"))
	if sess == nil {
		return
	}
	var req applyRequest
	if err := decodeBody(r, &req); err != nil || len(req.Ops) == 0 {
		writeErr(w, http.StatusBadRequest, "body must be {\"ops\": [...]}")
		return
	}
	next, res, err := engine.ApplyOps(sess.def, sess.st, req.Ops)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	s.commit(r.PathValue("session"), next)
	writeJSON(w, http.StatusOK, map[string]any{"view": buildView(sess.def, next), "result": res})
}

func (s *Server) handlePlaytestEnd(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("session")
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ended": true})
}

// commit stores the new state for a session (engine ops never mutate in place).
func (s *Server) commit(id string, next *engine.State) {
	s.mu.Lock()
	if sess := s.sessions[id]; sess != nil {
		sess.st = next
	}
	s.mu.Unlock()
}

// buildView assembles the same {state, actions, derived, beats, endingReached,
// clock, log, discoveredLore} payload the CLI returns, for the playtest panel.
func buildView(def *engine.Definition, st *engine.State) map[string]any {
	actions, _ := engine.AvailableActions(def, st)
	return map[string]any{
		"state":          st,
		"actions":        actions,
		"derived":        engine.BuildDerivedView(def, st),
		"beats":          engine.ActiveBeats(def, st),
		"endingReached":  engine.EndingsReached(def, st),
		"clock":          st.Clock,
		"log":            st.Log,
		"discoveredLore": st.DiscoveredLore,
	}
}
