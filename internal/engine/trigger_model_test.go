package engine

import (
	"encoding/json"
	"testing"
)

// TestTriggerModelRoundTrip verifies that Trigger, ScheduledItem, and the new
// Effect fields (In, Do, Key, Ticks) survive a JSON round-trip, and that the
// new State fields (Clock, Scheduled, Cooldowns, TriggerArmed, TriggerFired)
// are serialised/deserialised correctly.
func TestTriggerModelRoundTrip(t *testing.T) {
	src := `{
	  "id": "g1", "name": "Alarm Game", "version": 1,
	  "world": {"alarm": {"type": "bool", "default": false}},
	  "triggers": {
	    "raise_alarm": {
	      "when": {"target": "world.alarm", "op": "eq", "value": true},
	      "once": true,
	      "intent": "alarm fires once when raised",
	      "effects": [
	        {"op": "schedule", "in": 3,
	         "do": [{"op": "set", "target": "world.alarm", "value": false}]}
	      ]
	    },
	    "periodic_tick": {
	      "every": 5,
	      "intent": "fires every 5 ticks",
	      "effects": [{"op": "cooldown", "key": "confess", "ticks": 2}]
	    }
	  }
	}`

	var def Definition
	if err := json.Unmarshal([]byte(src), &def); err != nil {
		t.Fatalf("unmarshal def: %v", err)
	}

	// Verify raise_alarm trigger.
	ra, ok := def.Triggers["raise_alarm"]
	if !ok {
		t.Fatal("raise_alarm trigger missing")
	}
	if ra.When == nil || ra.When.Target != "world.alarm" || ra.When.Op != "eq" {
		t.Fatalf("raise_alarm.When: %+v", ra.When)
	}
	if ra.Once == nil || !*ra.Once {
		t.Fatalf("raise_alarm.Once: %v", ra.Once)
	}
	if len(ra.Effects) != 1 {
		t.Fatalf("raise_alarm effects count: %d", len(ra.Effects))
	}
	ef := ra.Effects[0]
	if ef.Op != "schedule" || ef.In != 3 {
		t.Fatalf("schedule effect: op=%q in=%d", ef.Op, ef.In)
	}
	if len(ef.Do) != 1 || ef.Do[0].Op != "set" {
		t.Fatalf("schedule.Do: %+v", ef.Do)
	}

	// Verify periodic_tick trigger.
	pt, ok := def.Triggers["periodic_tick"]
	if !ok {
		t.Fatal("periodic_tick trigger missing")
	}
	if pt.Every != 5 {
		t.Fatalf("periodic_tick.Every: %d", pt.Every)
	}
	if len(pt.Effects) != 1 {
		t.Fatalf("periodic_tick effects count: %d", len(pt.Effects))
	}
	cef := pt.Effects[0]
	if cef.Op != "cooldown" || cef.Key != "confess" || cef.Ticks != 2 {
		t.Fatalf("cooldown effect: %+v", cef)
	}

	// once() helper: nil Once defaults to true (one-shot).
	noOnce := Trigger{Effects: []Effect{{Op: "set", Target: "world.alarm", Value: false}}}
	if !noOnce.once() {
		t.Fatal("nil Once should default to once=true")
	}
	falseVal := false
	repeating := Trigger{Once: &falseVal, Effects: []Effect{{Op: "set", Target: "world.alarm", Value: false}}}
	if repeating.once() {
		t.Fatal("Once:false should not be once")
	}

	// Round-trip the definition.
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var def2 Definition
	if err := json.Unmarshal(b, &def2); err != nil {
		t.Fatalf("unmarshal2: %v", err)
	}
	if len(def2.Triggers) != 2 {
		t.Fatalf("triggers count after round-trip: %d", len(def2.Triggers))
	}

	// State round-trip with new v3 fields.
	st := &State{
		ID:    "r1",
		Clock: 7,
		Scheduled: []ScheduledItem{
			{Due: 10, Effects: []Effect{{Op: "set", Target: "world.alarm", Value: false}}},
		},
		Cooldowns:     map[string]int{"confess": 9},
		TriggerArmed:  map[string]bool{"raise_alarm": true},
		TriggerFired:  map[string]bool{"raise_alarm": true},
		World:         map[string]any{},
		Machines:      map[string]string{},
		Entities:      map[string]*Entity{},
		Relationships: []*Relationship{},
		History:       []HistoryEntry{},
	}
	sb, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	var st2 State
	if err := json.Unmarshal(sb, &st2); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if st2.Clock != 7 {
		t.Fatalf("Clock: %d", st2.Clock)
	}
	if len(st2.Scheduled) != 1 || st2.Scheduled[0].Due != 10 {
		t.Fatalf("Scheduled: %+v", st2.Scheduled)
	}
	if st2.Cooldowns["confess"] != 9 {
		t.Fatalf("Cooldowns: %v", st2.Cooldowns)
	}
	if !st2.TriggerArmed["raise_alarm"] {
		t.Fatalf("TriggerArmed: %v", st2.TriggerArmed)
	}
	if !st2.TriggerFired["raise_alarm"] {
		t.Fatalf("TriggerFired: %v", st2.TriggerFired)
	}
}
