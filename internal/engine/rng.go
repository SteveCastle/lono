package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// RNG is a deterministic splitmix64 generator; its state is serializable so
// rolls and snapshot restores are reproducible.
type RNG struct {
	state uint64
}

func (r *RNG) next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// IntInRange returns an int in [min, max] inclusive.
func (r *RNG) IntInRange(min, max int) int {
	if max <= min {
		return min
	}
	span := uint64(max - min + 1)
	return min + int(r.next()%span)
}

// RollDice parses "NdM", "NdM+K", or "NdM-K" and returns the total.
func (r *RNG) RollDice(spec string) (int, error) {
	mod := 0
	body := spec
	if i := strings.IndexAny(spec, "+-"); i >= 0 {
		m, err := strconv.Atoi(strings.TrimSpace(spec[i:]))
		if err != nil {
			return 0, fmt.Errorf("bad dice modifier in %q", spec)
		}
		mod = m
		body = spec[:i]
	}
	parts := strings.SplitN(strings.TrimSpace(body), "d", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad dice spec %q (want NdM)", spec)
	}
	n, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	sides, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || n < 1 || sides < 1 {
		return 0, fmt.Errorf("bad dice spec %q", spec)
	}
	total := mod
	for i := 0; i < n; i++ {
		total += r.IntInRange(1, sides)
	}
	return total, nil
}
