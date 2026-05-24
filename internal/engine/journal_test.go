package engine

import (
	"testing"
	"time"
)

// D1: Log model + record op tests.

func defForRecord() *Definition {
	return &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{
			"day": {Type: "int", Default: float64(1)},
		},
	}
}

func TestRecordAppendsEntry(t *testing.T) {
	def := defForRecord()
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, nil)

	before := time.Now().UTC()
	if err := applyEffect(def, st, ctx, Effect{Op: "record", Text: "Aria forgave you.", Tags: []string{"aria"}}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	after := time.Now().UTC()

	if len(st.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(st.Log))
	}
	e := st.Log[0]
	if e.Seq != 1 {
		t.Fatalf("expected Seq=1, got %d", e.Seq)
	}
	if e.Clock != st.Clock {
		t.Fatalf("expected Clock=%d, got %d", st.Clock, e.Clock)
	}
	if e.Text != "Aria forgave you." {
		t.Fatalf("expected text, got %q", e.Text)
	}
	if len(e.Tags) != 1 || e.Tags[0] != "aria" {
		t.Fatalf("expected tags [aria], got %v", e.Tags)
	}
	if e.TS.Before(before) || e.TS.After(after) {
		t.Fatalf("TS %v out of expected range [%v, %v]", e.TS, before, after)
	}
}

func TestRecordEmptyTextError(t *testing.T) {
	def := defForRecord()
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, nil)

	if err := applyEffect(def, st, ctx, Effect{Op: "record", Text: ""}); err == nil {
		t.Fatal("expected error for empty record text")
	}
	if len(st.Log) != 0 {
		t.Fatal("failed record must not append entry")
	}
}

func TestRecordTwoEntriesSeq(t *testing.T) {
	def := defForRecord()
	st, _ := NewInstance(def, "r", 1)
	ctx := newEvalCtx(nil, nil)

	if err := applyEffect(def, st, ctx, Effect{Op: "record", Text: "First event."}); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if err := applyEffect(def, st, ctx, Effect{Op: "record", Text: "Second event."}); err != nil {
		t.Fatalf("second record: %v", err)
	}

	if len(st.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(st.Log))
	}
	if st.Log[0].Seq != 1 {
		t.Fatalf("first entry Seq should be 1, got %d", st.Log[0].Seq)
	}
	if st.Log[1].Seq != 2 {
		t.Fatalf("second entry Seq should be 2, got %d", st.Log[1].Seq)
	}
}

func TestRecordClockedEntry(t *testing.T) {
	def := defForRecord()
	st, _ := NewInstance(def, "r", 1)
	st.Clock = 5
	ctx := newEvalCtx(nil, nil)

	if err := applyEffect(def, st, ctx, Effect{Op: "record", Text: "Late event."}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if st.Log[0].Clock != 5 {
		t.Fatalf("expected Clock=5 in entry, got %d", st.Log[0].Clock)
	}
}

// D1: validation — record with empty text in a transition is rejected.

func TestValidateRecordEffectRequiresText(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{"day": {Type: "int", Default: float64(1)}},
		Machines: map[string]Machine{
			"arc": {
				Initial: "a", States: []string{"a", "b"},
				Transitions: []Transition{
					{
						ID: "go", From: StateSet{"a"}, To: "b",
						Effects: []Effect{
							{Op: "record", Text: ""},
						},
					},
				},
			},
		},
	}
	errs := ValidateDefinition(def)
	found := false
	for _, e := range errs {
		if e.Path != "" {
			found = true
			break
		}
	}
	if len(errs) == 0 || !found {
		t.Fatalf("expected validation error for record with empty text, got %v", errs)
	}
}

func TestValidateRecordEffectValid(t *testing.T) {
	def := &Definition{
		ID: "g", Version: 1,
		World: map[string]VarSpec{"day": {Type: "int", Default: float64(1)}},
		Machines: map[string]Machine{
			"arc": {
				Initial: "a", States: []string{"a", "b"},
				Transitions: []Transition{
					{
						ID: "go", From: StateSet{"a"}, To: "b",
						Effects: []Effect{
							{Op: "record", Text: "Something happened.", Tags: []string{"npc"}},
						},
					},
				},
			},
		},
	}
	errs := ValidateDefinition(def)
	if len(errs) != 0 {
		t.Fatalf("expected valid record effect, got %v", errs)
	}
}
