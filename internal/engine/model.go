package engine

import (
	"encoding/json"
	"fmt"
)

// Definition is the full set of rules for a game (the persisted definition.json).
type Definition struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description,omitempty"`
	Intent            string                 `json:"intent,omitempty"`
	Version           int                    `json:"version"`
	World             map[string]VarSpec     `json:"world,omitempty"`
	EntityTypes       map[string]EntityType  `json:"entityTypes,omitempty"`
	ItemTypes         map[string]ItemType    `json:"itemTypes,omitempty"`
	RelationshipTypes map[string]RelType     `json:"relationshipTypes,omitempty"`
	Machines          map[string]Machine     `json:"machines,omitempty"`
	Derived           map[string]DerivedSpec `json:"derived,omitempty"`
	Beats             map[string]Beat        `json:"beats,omitempty"`
	Triggers          map[string]Trigger     `json:"triggers,omitempty"`
	Entities          map[string]EntityInit  `json:"entities,omitempty"`
	Relationships     []RelInit              `json:"relationships,omitempty"`
	Setup             []Effect               `json:"setup,omitempty"`
}

// Trigger is a reactive rule that fires automatically when its condition arises.
// A trigger must have a When guard (for reactive/edge firing) and/or an Every
// value (for periodic firing during Advance).
type Trigger struct {
	When    *Guard   `json:"when,omitempty"`
	Every   int      `json:"every,omitempty"`
	Once    *bool    `json:"once,omitempty"` // nil means true (one-shot, matches Beat convention)
	Effects []Effect `json:"effects"`
	Intent  string   `json:"intent,omitempty"`
}

// once reports whether a trigger fires at most once ever (the default when unset).
func (t Trigger) once() bool { return t.Once == nil || *t.Once }

// EntityInit is the declarative starting configuration for a cast member.
type EntityInit struct {
	Type      string            `json:"type"`
	Attrs     map[string]any    `json:"attrs,omitempty"`
	Inventory map[string]int    `json:"inventory,omitempty"`
	Equipped  map[string]string `json:"equipped,omitempty"`
}

// RelInit is the declarative starting configuration for a relationship between
// two cast members.
type RelInit struct {
	Type  string         `json:"type"`
	From  string         `json:"from"`
	To    string         `json:"to"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type EntityType struct {
	Description string              `json:"description,omitempty"`
	Intent      string              `json:"intent,omitempty"`
	Attributes  map[string]VarSpec  `json:"attributes,omitempty"`
	Slots       map[string]SlotSpec `json:"slots,omitempty"`
}

// SlotSpec is an equipment slot on an entity type. It holds at most one item;
// Accepts lists the item categories that fit.
type SlotSpec struct {
	Accepts     []string `json:"accepts,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ItemType struct {
	Description string         `json:"description,omitempty"`
	Intent      string         `json:"intent,omitempty"`
	MaxStack    *int           `json:"maxStack,omitempty"`
	Category    string         `json:"category,omitempty"`
	Equippable  bool           `json:"equippable,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

type RelType struct {
	Description string             `json:"description,omitempty"`
	Intent      string             `json:"intent,omitempty"`
	From        string             `json:"from"`
	To          string             `json:"to"`
	Directed    bool               `json:"directed"`
	Attributes  map[string]VarSpec `json:"attributes,omitempty"`
}

type Machine struct {
	Attach      *AttachSpec          `json:"attach,omitempty"`
	Description string               `json:"description,omitempty"`
	Intent      string               `json:"intent,omitempty"`
	Initial     string               `json:"initial"`
	States      []string             `json:"states"`
	StateMeta   map[string]StateMeta `json:"stateMeta,omitempty"`
	Transitions []Transition         `json:"transitions,omitempty"`
}

// StateMeta is optional narrative + flags for a machine state. A terminal state
// is reported as a reached "ending" when current.
type StateMeta struct {
	Description string `json:"description,omitempty"`
	Intent      string `json:"intent,omitempty"`
	Terminal    bool   `json:"terminal,omitempty"`
	Ending      bool   `json:"ending,omitempty"`
}

// AttachSpec attaches a machine template to a host type. To is
// "relationshipType:<name>" or "entityType:<name>".
type AttachSpec struct {
	To string `json:"to"`
}

// AttachKind splits To into ("relationshipType"|"entityType", name). Returns
// ("","") if malformed.
func (a *AttachSpec) AttachKind() (kind, name string) {
	if a == nil {
		return "", ""
	}
	if i := indexByteStr(a.To, ':'); i >= 0 {
		return a.To[:i], a.To[i+1:]
	}
	return "", ""
}

func indexByteStr(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

type Transition struct {
	ID          string             `json:"id"`
	Description string             `json:"description,omitempty"`
	Intent      string             `json:"intent,omitempty"`
	From        StateSet           `json:"from"`
	To          string             `json:"to"`
	Params      map[string]VarSpec `json:"params,omitempty"`
	Guard       *Guard             `json:"guard,omitempty"`
	Effects     []Effect           `json:"effects,omitempty"`
}

// DerivedSpec defines a named aggregate query over the entity/relationship graph.
// It is recomputed on read and never stored.
type DerivedSpec struct {
	Over   string    `json:"over"` // "relationships" | "entities"
	Where  WhereSpec `json:"where"`
	Reduce string    `json:"reduce"` // count|any|sum:<attr>|min:<attr>|max:<attr>|argmax:<attr>|argmin:<attr>
	Intent string    `json:"intent,omitempty"`
}

// WhereSpec filters the collection a DerivedSpec aggregates over.
type WhereSpec struct {
	Type  string     `json:"type,omitempty"` // relationship type, or entity type for over:entities
	From  any        `json:"from,omitempty"` // relationship endpoint: literal id, "$self", or {"$path":"<p>"}
	To    any        `json:"to,omitempty"`   // relationship endpoint: literal id, "$self", or {"$path":"<p>"}
	Attrs []AttrPred `json:"attrs,omitempty"`
}

// AttrPred is a predicate on a relationship/entity attribute.
type AttrPred struct {
	Attr  string `json:"attr"`
	Op    string `json:"op"` // eq|ne|gt|gte|lt|lte|in
	Value any    `json:"value,omitempty"`
}

// Beat is an authored narrative unit the engine surfaces as "active" when its
// (optional) machineState binding matches and its (optional) guard holds.
type Beat struct {
	Text         string           `json:"text"`
	MachineState *MachineStateRef `json:"machineState,omitempty"`
	Guard        *Guard           `json:"guard,omitempty"`
	Once         *bool            `json:"once,omitempty"` // nil means true (one-shot)
	Intent       string           `json:"intent,omitempty"`
}

// MachineStateRef binds a beat to a global machine being in a given state.
type MachineStateRef struct {
	Machine string `json:"machine"`
	State   string `json:"state"`
}

// once reports whether a beat is one-shot (the default when unset).
func (b Beat) once() bool { return b.Once == nil || *b.Once }

// Guard is either a combinator (and/or/not) or a leaf (target/op/value).
type Guard struct {
	And    []Guard `json:"and,omitempty"`
	Or     []Guard `json:"or,omitempty"`
	Not    *Guard  `json:"not,omitempty"`
	Target string  `json:"target,omitempty"`
	Op     string  `json:"op,omitempty"`
	Value  any     `json:"value,omitempty"`
}

// Effect is a tagged union over Op; only the fields relevant to Op are used.
type Effect struct {
	Op     string `json:"op"`
	Target string `json:"target,omitempty"`
	Value  any    `json:"value,omitempty"`
	// inventory / equipment
	Entity string `json:"entity,omitempty"`
	Item   string `json:"item,omitempty"`
	Count  int    `json:"count,omitempty"`
	Slot   string `json:"slot,omitempty"`
	// create/destroy entity
	EntityType string         `json:"entityType,omitempty"`
	ID         string         `json:"id,omitempty"`
	Attrs      map[string]any `json:"attrs,omitempty"`
	// relationship
	RelType string   `json:"relType,omitempty"`
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
	Attr    string   `json:"attr,omitempty"`
	By      *float64 `json:"by,omitempty"`
	// machine
	Machine string `json:"machine,omitempty"`
	State   string `json:"state,omitempty"`
	// roll
	Dice  string `json:"dice,omitempty"`
	Store string `json:"store,omitempty"`
	// narrative
	Beat string `json:"beat,omitempty"`
	// compute
	Fn string `json:"fn,omitempty"`
	A  any    `json:"a,omitempty"`
	B  any    `json:"b,omitempty"`
	// if / then / else
	When *Guard   `json:"when,omitempty"`
	Then []Effect `json:"then,omitempty"`
	Else []Effect `json:"else,omitempty"`
	// schedule: enqueue Do to fire in In ticks
	In int      `json:"in,omitempty"`
	Do []Effect `json:"do,omitempty"`
	// cooldown: set a named cooldown expiring in Ticks ticks
	Key   string `json:"key,omitempty"`
	Ticks int    `json:"ticks,omitempty"`
	// record: append a narrative journal entry
	Text string   `json:"text,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// StateSet is a transition's "from": a single state, a list, or "*".
type StateSet []string

func (s *StateSet) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*s = StateSet{single}
		return nil
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return fmt.Errorf("from must be string or []string: %w", err)
	}
	*s = StateSet(list)
	return nil
}

func (s StateSet) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

func (s StateSet) Matches(state string) bool {
	for _, x := range s {
		if x == "*" || x == state {
			return true
		}
	}
	return false
}
