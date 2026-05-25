package engine

import (
	"encoding/json"
	"time"
)

type State struct {
	ID             string             `json:"id"`
	GameID         string             `json:"gameId"`
	GameVersion    int                `json:"gameVersion"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	Seed           int64              `json:"seed"`
	RNGState       uint64             `json:"rngState"`
	World          map[string]any     `json:"world"`
	Machines       map[string]string  `json:"machines"`
	Entities       map[string]*Entity `json:"entities"`
	Relationships  []*Relationship    `json:"relationships"`
	History        []HistoryEntry     `json:"history"`
	DeliveredBeats []string           `json:"deliveredBeats,omitempty"`
	// v3 time + trigger state
	Clock        int             `json:"clock"`
	Scheduled    []ScheduledItem `json:"scheduled,omitempty"`
	Cooldowns    map[string]int  `json:"cooldowns,omitempty"`
	TriggerArmed map[string]bool `json:"triggerArmed,omitempty"`
	TriggerFired map[string]bool `json:"triggerFired,omitempty"`
	// v3 narrative journal
	Log []LogEntry `json:"log,omitempty"`
	// v4 lore
	DiscoveredLore []string `json:"discoveredLore,omitempty"`
}

// LogEntry is a single narrative journal record appended by the record effect.
type LogEntry struct {
	Seq   int       `json:"seq"`
	Clock int       `json:"clock"`
	TS    time.Time `json:"ts"`
	Tags  []string  `json:"tags,omitempty"`
	Text  string    `json:"text"`
}

// ScheduledItem holds effects to apply at a specific clock tick.
type ScheduledItem struct {
	Due     int      `json:"due"`
	Effects []Effect `json:"effects"`
}

type Entity struct {
	Type        string            `json:"type"`
	Attrs       map[string]any    `json:"attrs"`
	Inventory   map[string]int    `json:"inventory"`
	Machines    map[string]string `json:"machines,omitempty"`
	Equipped    map[string]string `json:"equipped,omitempty"`
	Description string            `json:"description,omitempty"`
}

type Relationship struct {
	Type     string            `json:"type"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Attrs    map[string]any    `json:"attrs"`
	Machines map[string]string `json:"machines,omitempty"`
}

type RollResult struct {
	Store  string  `json:"store,omitempty"`
	Dice   string  `json:"dice,omitempty"`
	Result float64 `json:"result"`
}

// CheckResult is the outcome of a `check` effect: a dice roll plus modifiers
// measured against a difficulty. Readable mid-action via check.<store>.<field>
// and surfaced in the action result so the narrator can see what the dice said.
type CheckResult struct {
	Store   string  `json:"store,omitempty"`
	Dice    string  `json:"dice,omitempty"`
	Roll    float64 `json:"roll"`    // the raw die result (before modifiers)
	Total   float64 `json:"total"`   // roll + sum of modifiers
	DC      float64 `json:"dc"`      // difficulty the total was measured against
	Margin  float64 `json:"margin"`  // total - dc (negative = failed, by how much)
	Success bool    `json:"success"` // total >= dc
}

type HistoryEntry struct {
	Seq     int            `json:"seq"`
	TS      time.Time      `json:"ts"`
	Kind    string         `json:"kind"` // action|apply|snapshot_restore
	Machine string         `json:"machine,omitempty"`
	Action  string         `json:"action,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Rolls   []RollResult   `json:"rolls,omitempty"`
}

// NewInstance creates a fresh instance with world/machine defaults applied.
// Entities/relationships/inventory come from def.Setup (applied by the caller
// via the engine's instance bootstrap; see engine.go StartInstance).
func NewInstance(def *Definition, instanceID string, seed int64) (*State, error) {
	now := time.Now().UTC()
	st := &State{
		ID:            instanceID,
		GameID:        def.ID,
		GameVersion:   def.Version,
		CreatedAt:     now,
		UpdatedAt:     now,
		Seed:          seed,
		RNGState:      uint64(seed),
		World:         map[string]any{},
		Machines:      map[string]string{},
		Entities:      map[string]*Entity{},
		Relationships: []*Relationship{},
		History:       []HistoryEntry{},
	}
	for name, spec := range def.World {
		st.World[name] = DefaultValue(spec)
	}
	for name, m := range def.Machines {
		if m.Attach != nil {
			continue // attached machines live per-host (Entity/Relationship.Machines), not globally
		}
		st.Machines[name] = m.Initial
	}
	return st, nil
}

// Clone returns a deep copy via JSON round-trip (safe and simple at this scale).
func (s *State) Clone() *State {
	b, _ := json.Marshal(s)
	var cp State
	_ = json.Unmarshal(b, &cp)
	return &cp
}
