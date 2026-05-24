package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// errNoSuchPath is returned when a path does not resolve to any node.
var errNoSuchPath = errors.New("no such path")

// IsNoSuchPath reports whether err is (or wraps) errNoSuchPath.
func IsNoSuchPath(err error) bool { return errors.Is(err, errNoSuchPath) }

// GetNode walks root along a "/"-delimited path and returns the node at that
// position. Leading and trailing "/" are trimmed; an empty path returns root.
//
// Walking rules:
//   - map[string]any: index by the next segment (missing key → errNoSuchPath).
//   - []any of map[string]any elements that each have an "id" string field:
//     match the element whose "id" == segment (missing → errNoSuchPath).
//   - other []any: parse segment as a non-negative integer index
//     (out-of-range or non-integer → errNoSuchPath).
//   - scalar with remaining segments: errNoSuchPath.
func GetNode(root any, path string) (any, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return root, nil
	}
	segments := strings.Split(path, "/")
	cur := root
	for i, seg := range segments {
		switch v := cur.(type) {
		case map[string]any:
			val, ok := v[seg]
			if !ok {
				return nil, fmt.Errorf("%w: %q not found at %q", errNoSuchPath, seg, strings.Join(segments[:i], "/"))
			}
			cur = val
		case []any:
			if isIDArray(v) {
				elem, ok := findByID(v, seg)
				if !ok {
					return nil, fmt.Errorf("%w: no element with id %q", errNoSuchPath, seg)
				}
				cur = elem
			} else {
				idx, err := strconv.Atoi(seg)
				if err != nil || idx < 0 || idx >= len(v) {
					return nil, fmt.Errorf("%w: index %q out of range", errNoSuchPath, seg)
				}
				cur = v[idx]
			}
		default:
			// scalar with remaining path segments
			return nil, fmt.Errorf("%w: cannot descend into scalar at %q", errNoSuchPath, strings.Join(segments[:i], "/"))
		}
	}
	return cur, nil
}

// isIDArray reports whether arr is a []any whose elements are all
// map[string]any objects with a non-empty "id" string field.
func isIDArray(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	for _, elem := range arr {
		m, ok := elem.(map[string]any)
		if !ok {
			return false
		}
		id, ok := m["id"].(string)
		if !ok || id == "" {
			return false
		}
	}
	return true
}

// findByID returns the first element in arr whose "id" field equals id.
func findByID(arr []any, id string) (any, bool) {
	for _, elem := range arr {
		if m, ok := elem.(map[string]any); ok {
			if m["id"] == id {
				return elem, true
			}
		}
	}
	return nil, false
}

// TreeView returns a depth-limited, names-only structural summary of root.
//
//   - map[string]any at depth > 0 → map[string]any{key: TreeView(child, depth-1)}.
//   - map[string]any at depth <= 0 → sorted []string of keys.
//   - []any of id-objects → []string of ids.
//   - other []any → int length.
//   - scalar → nil.
func TreeView(root any, depth int) any {
	switch v := root.(type) {
	case map[string]any:
		if depth <= 0 {
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return keys
		}
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = TreeView(child, depth-1)
		}
		return out
	case []any:
		if isIDArray(v) {
			ids := make([]string, 0, len(v))
			for _, elem := range v {
				m := elem.(map[string]any)
				ids = append(ids, m["id"].(string))
			}
			return ids
		}
		return len(v)
	default:
		return nil
	}
}

// GetDefNode resolves a path against a Definition by marshaling it to a
// generic map first, then delegating to GetNode.
func GetDefNode(def *Definition, path string) (any, error) {
	b, err := json.Marshal(def)
	if err != nil {
		return nil, err
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return GetNode(m, path)
}

// GetStateNode resolves a path against a State, with access to computed runtime views.
//
// If the first segment of path is one of the computed views ("derived", "beats",
// "actions", "endingReached"), the corresponding view is built and the remaining
// path (if any) is resolved against it.
//
// Special case: if the path begins with "relationships/<type>/<from>/<to>"
// (i.e., at least 4 segments total), the function locates that specific
// relationship element and then resolves any remaining sub-path against it.
// All other paths are resolved against the full marshaled state map.
func GetStateNode(def *Definition, st *State, path string) (any, error) {
	path = strings.Trim(path, "/")

	// Check for computed runtime view segments first.
	firstSeg, rest, _ := strings.Cut(path, "/")
	switch firstSeg {
	case "derived":
		view := BuildDerivedView(def, st)
		return marshalAndNavigate(view, rest)
	case "beats":
		view := ActiveBeats(def, st)
		return marshalAndNavigate(view, rest)
	case "actions":
		view, err := AvailableActions(def, st)
		if err != nil {
			return nil, err
		}
		return marshalAndNavigate(view, rest)
	case "endingReached":
		view := EndingsReached(def, st)
		return marshalAndNavigate(view, rest)
	}

	segments := strings.SplitN(path, "/", 5) // up to 5 parts: [relationships, type, from, to, rest]

	if len(segments) >= 4 && segments[0] == "relationships" {
		relType, from, to := segments[1], segments[2], segments[3]
		rel := findRelationship(st, relType, from, to)
		if rel == nil {
			return nil, fmt.Errorf("%w: no relationship %s %s->%s", errNoSuchPath, relType, from, to)
		}
		// Marshal the relationship element to a generic map.
		b, err := json.Marshal(rel)
		if err != nil {
			return nil, err
		}
		var elem any
		if err := json.Unmarshal(b, &elem); err != nil {
			return nil, err
		}
		if len(segments) == 4 {
			return elem, nil
		}
		// Remaining sub-path after the triple.
		return GetNode(elem, segments[4])
	}

	b, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return GetNode(m, path)
}

// marshalAndNavigate round-trips v through JSON to produce a generic any, then
// navigates into it via GetNode(v, path). An empty path returns the whole view.
func marshalAndNavigate(v any, path string) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return GetNode(m, path)
}
