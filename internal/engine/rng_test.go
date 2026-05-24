package engine

import "testing"

func TestRNGDeterministic(t *testing.T) {
	a := &RNG{state: 12345}
	b := &RNG{state: 12345}
	for i := 0; i < 100; i++ {
		if a.IntInRange(1, 6) != b.IntInRange(1, 6) {
			t.Fatal("same seed must produce same sequence")
		}
	}
}

func TestRNGInRangeBounds(t *testing.T) {
	r := &RNG{state: 7}
	for i := 0; i < 1000; i++ {
		v := r.IntInRange(1, 6)
		if v < 1 || v > 6 {
			t.Fatalf("out of range: %d", v)
		}
	}
}

func TestRollDice(t *testing.T) {
	r := &RNG{state: 99}
	v, err := r.RollDice("2d6+1")
	if err != nil {
		t.Fatal(err)
	}
	if v < 3 || v > 13 { // 2..12 + 1
		t.Fatalf("2d6+1 out of range: %d", v)
	}
	if _, err := r.RollDice("garbage"); err == nil {
		t.Fatal("expected parse error")
	}
}
