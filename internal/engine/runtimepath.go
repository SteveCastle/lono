package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// errPathNotWritable is returned when a path is computed/read-only or
// otherwise not writable through the runtime path compiler.
var errPathNotWritable = errors.New("path not writable")

// IsPathNotWritable reports whether err is (or wraps) errPathNotWritable.
func IsPathNotWritable(err error) bool { return errors.Is(err, errPathNotWritable) }

// readOnlyRoots is the set of top-level path segments that are computed views
// and must never be written through CompileRuntimeWrite.
var readOnlyRoots = map[string]bool{
	"derived":       true,
	"beats":         true,
	"actions":       true,
	"endingReached": true,
}

// CompileRuntimeWrite maps a "/" -delimited runtime path + value to a slice
// of validated effect ops, or returns errPathNotWritable for computed/read-only
// or unknown paths. If remove is true the op is a deletion variant.
//
// Supported paths (non-remove):
//
//	world/<v>                                → set{Target:"world.<v>", Value:value}
//	entities/<id>                            → create_entity (value is map with "type"/"attrs")
//	entities/<id>/attrs/<a>                  → set{Target:"entity.<id>.<a>", Value:value}
//	entities/<id>/inventory/<item>           → add_item or remove_item (delta to target count)
//	entities/<id>/equipped/<slot>            → equip{Entity:id, Slot:slot, Item:value-as-string}
//	entities/<id>/machines/<m>              → set_attached_state{Machine:m, Entity:id, State:value-as-string}
//	relationships/<type>/<from>/<to>/attrs/<a> → set_relationship{RelType:type,From:from,To:to,Attrs:{a:value}}
//	relationships/<type>/<from>/<to>/machines/<m> → set_attached_state{Machine:m,From:from,To:to,State:value-as-string}
//	machines/<m>                             → set_machine_state{Machine:m, State:value-as-string}
//
// Supported paths (remove=true):
//
//	entities/<id>                            → destroy_entity{ID:id}
//	entities/<id>/equipped/<slot>            → unequip{Entity:id, Slot:slot}
//	entities/<id>/inventory/<item>           → remove_item of full current count (no-op if zero)
//	relationships/<type>/<from>/<to>         → remove_relationship{RelType:type,From:from,To:to}
func CompileRuntimeWrite(def *Definition, st *State, path string, value any, remove bool) ([]Effect, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", errPathNotWritable)
	}
	parts := strings.Split(path, "/")

	// Reject known computed/read-only roots.
	if readOnlyRoots[parts[0]] {
		return nil, fmt.Errorf("%w: %q is a computed/read-only path", errPathNotWritable, parts[0])
	}

	switch parts[0] {
	case "world":
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: world path must be world/<var>", errPathNotWritable)
		}
		if remove {
			return nil, fmt.Errorf("%w: cannot remove a world variable", errPathNotWritable)
		}
		return []Effect{{Op: "set", Target: "world." + parts[1], Value: value}}, nil

	case "entities":
		return compileEntityWrite(def, st, parts, value, remove)

	case "relationships":
		return compileRelationshipWrite(def, st, parts, value, remove)

	case "machines":
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: machines path must be machines/<machine>", errPathNotWritable)
		}
		if remove {
			return nil, fmt.Errorf("%w: cannot remove a machine state", errPathNotWritable)
		}
		state, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("machines/<m> value must be a string (machine state)")
		}
		return []Effect{{Op: "set_machine_state", Machine: parts[1], State: state}}, nil

	default:
		return nil, fmt.Errorf("%w: unknown path root %q", errPathNotWritable, parts[0])
	}
}

func compileEntityWrite(def *Definition, st *State, parts []string, value any, remove bool) ([]Effect, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("%w: entities path requires at least entities/<id>", errPathNotWritable)
	}
	id := parts[1]

	// entities/<id> only
	if len(parts) == 2 {
		if remove {
			return []Effect{{Op: "destroy_entity", ID: id}}, nil
		}
		// create_entity: value must be a map with at least "type"
		m, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entities/<id> value must be a JSON object with a \"type\" field")
		}
		typ, _ := m["type"].(string)
		if typ == "" {
			return nil, fmt.Errorf("entities/<id> object must have a non-empty \"type\" field")
		}
		var attrs map[string]any
		if a, ok := m["attrs"].(map[string]any); ok {
			attrs = a
		}
		return []Effect{{Op: "create_entity", EntityType: typ, ID: id, Attrs: attrs}}, nil
	}

	if len(parts) < 4 {
		return nil, fmt.Errorf("%w: entities path too short", errPathNotWritable)
	}
	sub := parts[2]

	switch sub {
	case "attrs":
		attr := parts[3]
		if remove {
			return nil, fmt.Errorf("%w: cannot remove an entity attribute (use set to zero it)", errPathNotWritable)
		}
		return []Effect{{Op: "set", Target: "entity." + id + "." + attr, Value: value}}, nil

	case "inventory":
		item := parts[3]
		return compileInventoryWrite(st, id, item, value, remove)

	case "equipped":
		slot := parts[3]
		if remove {
			return []Effect{{Op: "unequip", Entity: id, Slot: slot}}, nil
		}
		itemStr, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("entities/<id>/equipped/<slot> value must be a string (item type)")
		}
		return []Effect{{Op: "equip", Entity: id, Slot: slot, Item: itemStr}}, nil

	case "machines":
		machine := parts[3]
		if remove {
			return nil, fmt.Errorf("%w: cannot remove an entity machine state", errPathNotWritable)
		}
		stateStr, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("entities/<id>/machines/<m> value must be a string (machine state)")
		}
		return []Effect{{Op: "set_attached_state", Machine: machine, Entity: id, State: stateStr}}, nil

	default:
		return nil, fmt.Errorf("%w: unknown entity sub-path %q", errPathNotWritable, sub)
	}
}

func compileInventoryWrite(st *State, id, item string, value any, remove bool) ([]Effect, error) {
	var cur int
	if en, ok := st.Entities[id]; ok {
		cur = en.Inventory[item]
	}
	if remove {
		if cur == 0 {
			// Already absent — no-op (return empty slice).
			return []Effect{}, nil
		}
		return []Effect{{Op: "remove_item", Entity: id, Item: item, Count: cur}}, nil
	}
	// Non-remove: compute delta to reach integer target.
	f, ok := toFloat(value)
	if !ok {
		return nil, fmt.Errorf("entities/<id>/inventory/<item> value must be a number")
	}
	target := int(f)
	if float64(target) != f {
		return nil, fmt.Errorf("entities/<id>/inventory/<item> value must be a whole number")
	}
	if target < 0 {
		return nil, fmt.Errorf("entities/<id>/inventory/<item> target count must be non-negative")
	}
	delta := target - cur
	switch {
	case delta == 0:
		return []Effect{}, nil
	case delta > 0:
		return []Effect{{Op: "add_item", Entity: id, Item: item, Count: delta}}, nil
	default: // delta < 0
		return []Effect{{Op: "remove_item", Entity: id, Item: item, Count: -delta}}, nil
	}
}

func compileRelationshipWrite(def *Definition, st *State, parts []string, value any, remove bool) ([]Effect, error) {
	// Minimum for relationships/<type>/<from>/<to>
	if len(parts) < 4 {
		return nil, fmt.Errorf("%w: relationships path requires relationships/<type>/<from>/<to>", errPathNotWritable)
	}
	relType, from, to := parts[1], parts[2], parts[3]

	// relationships/<type>/<from>/<to> only
	if len(parts) == 4 {
		if remove {
			return []Effect{{Op: "remove_relationship", RelType: relType, From: from, To: to}}, nil
		}
		// Bare relationship create/update is not supported through set (use apply)
		return nil, fmt.Errorf("%w: to create a relationship use apply --ops set_relationship; path must include a sub-key (attrs/<a>)", errPathNotWritable)
	}

	if len(parts) < 6 {
		return nil, fmt.Errorf("%w: relationships sub-path too short", errPathNotWritable)
	}
	sub := parts[4]

	switch sub {
	case "attrs":
		attr := parts[5]
		if remove {
			return nil, fmt.Errorf("%w: cannot remove a relationship attribute", errPathNotWritable)
		}
		return []Effect{{Op: "set_relationship", RelType: relType, From: from, To: to, Attrs: map[string]any{attr: value}}}, nil

	case "machines":
		machine := parts[5]
		if remove {
			return nil, fmt.Errorf("%w: cannot remove a relationship machine state", errPathNotWritable)
		}
		stateStr, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("relationships/.../machines/<m> value must be a string (machine state)")
		}
		return []Effect{{Op: "set_attached_state", Machine: machine, From: from, To: to, State: stateStr}}, nil

	default:
		return nil, fmt.Errorf("%w: unknown relationship sub-path %q", errPathNotWritable, sub)
	}
}

// ForceWrite performs a raw, unvalidated mutation of st by marshaling it to a
// generic map, navigating to the parent of the final path segment, and
// setting or deleting the leaf key. The updated map is then unmarshaled back
// into *st.
//
// Path segments are "/" -delimited. Intermediate nodes must already exist as
// map[string]any; ForceWrite will NOT create missing intermediate maps.
//
// Array-valued nodes (e.g. relationships) cannot be targeted by index through
// this function — return errPathNotWritable in that case. ForceWrite is an
// escape hatch for out-of-band corrections; prefer CompileRuntimeWrite for
// normal use.
func ForceWrite(st *State, path string, value any, remove bool) error {
	path = strings.Trim(path, "/")
	if path == "" {
		return fmt.Errorf("%w: empty path", errPathNotWritable)
	}
	segments := strings.Split(path, "/")

	// Marshal state to a generic map.
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("force write marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("force write unmarshal: %w", err)
	}

	// Navigate to the parent of the final segment.
	parent, leafKey, err := navigateToParent(m, segments)
	if err != nil {
		return fmt.Errorf("force write: %w", err)
	}

	if remove {
		delete(parent, leafKey)
	} else {
		parent[leafKey] = value
	}

	// Marshal back into *st.
	updated, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("force write re-marshal: %w", err)
	}
	return json.Unmarshal(updated, st)
}

// navigateToParent descends into node along segments[0..n-2] and returns the
// parent map and the final segment (leaf key). All intermediate nodes must be
// map[string]any; arrays are not supported.
func navigateToParent(node map[string]any, segments []string) (map[string]any, string, error) {
	cur := node
	for _, seg := range segments[:len(segments)-1] {
		child, ok := cur[seg]
		if !ok {
			return nil, "", fmt.Errorf("%w: path segment %q not found", errPathNotWritable, seg)
		}
		next, ok := child.(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("%w: path segment %q is not a map (got %T); array paths are not supported by ForceWrite", errPathNotWritable, seg, child)
		}
		cur = next
	}
	return cur, segments[len(segments)-1], nil
}
