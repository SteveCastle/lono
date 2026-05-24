package engine

import (
	"fmt"
	"time"
)

// applyEffect mutates st in place per a single op. Callers run effects against a
// clone and discard it on error to preserve atomicity.
func applyEffect(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	switch e.Op {
	case "set":
		return setScalar(def, st, ctx, e.Target, resolveValue(st, ctx, e.Value))
	case "inc", "dec", "mul":
		return arithScalar(def, st, ctx, e.Op, e.Target, e.Value)
	case "add_item", "remove_item":
		return inventoryOp(def, st, e)
	case "create_entity":
		return createEntity(def, st, e)
	case "destroy_entity":
		if _, ok := st.Entities[e.ID]; !ok {
			return fmt.Errorf("unknown entity %q", e.ID)
		}
		delete(st.Entities, e.ID)
		return nil
	case "set_relationship":
		return setRelationship(def, st, e)
	case "adjust_relationship":
		return adjustRelationship(def, st, e)
	case "remove_relationship":
		return removeRelationship(st, e)
	case "set_machine_state":
		m, ok := def.Machines[e.Machine]
		if !ok {
			return fmt.Errorf("unknown machine %q", e.Machine)
		}
		if !contains(m.States, e.State) {
			return fmt.Errorf("state %q not in machine %q", e.State, e.Machine)
		}
		st.Machines[e.Machine] = e.State
		return nil
	case "roll":
		return rollOp(ctx, e)
	case "set_attached_state":
		return setAttachedState(def, st, e)
	case "mark_beat":
		return markBeat(def, st, e)
	case "equip":
		return equipOp(def, st, e)
	case "unequip":
		en, ok := st.Entities[e.Entity]
		if !ok {
			return fmt.Errorf("unknown entity %q", e.Entity)
		}
		delete(en.Equipped, e.Slot)
		return nil
	case "compute":
		return computeOp(def, st, ctx, e)
	case "if":
		return ifOp(def, st, ctx, e)
	case "add_to":
		return setAddOp(def, st, ctx, e)
	case "remove_from":
		return setRemoveOp(def, st, ctx, e)
	case "clear":
		return setClearOp(def, st, ctx, e)
	case "schedule":
		return scheduleOp(st, e)
	case "cooldown":
		return cooldownOp(st, e)
	case "record":
		return recordOp(st, e)
	case "move":
		return moveOp(def, st, e)
	default:
		return fmt.Errorf("unknown effect op %q", e.Op)
	}
}

// scheduleOp enqueues e.Do to fire when st.Clock reaches st.Clock+e.In.
func scheduleOp(st *State, e Effect) error {
	if e.In <= 0 {
		return fmt.Errorf("schedule: in must be > 0 (got %d)", e.In)
	}
	st.Scheduled = append(st.Scheduled, ScheduledItem{
		Due:     st.Clock + e.In,
		Effects: e.Do,
	})
	return nil
}

// cooldownOp sets a named cooldown expiry: Cooldowns[Key] = Clock + Ticks.
func cooldownOp(st *State, e Effect) error {
	if e.Key == "" {
		return fmt.Errorf("cooldown: key must not be empty")
	}
	if e.Ticks <= 0 {
		return fmt.Errorf("cooldown: ticks must be > 0 (got %d)", e.Ticks)
	}
	if st.Cooldowns == nil {
		st.Cooldowns = map[string]int{}
	}
	st.Cooldowns[e.Key] = st.Clock + e.Ticks
	return nil
}

// recordOp appends a LogEntry to st.Log. Text must be non-empty.
func recordOp(st *State, e Effect) error {
	if e.Text == "" {
		return fmt.Errorf("record: text must not be empty")
	}
	st.Log = append(st.Log, LogEntry{
		Seq:   len(st.Log) + 1,
		Clock: st.Clock,
		TS:    time.Now().UTC(),
		Tags:  e.Tags,
		Text:  e.Text,
	})
	return nil
}

// moveOp moves an entity to a new location by setting a ref attribute.
// attr defaults to "location". If e.Via is set, a relationship of that type
// from the entity's current attr value to e.To must exist.
func moveOp(def *Definition, st *State, e Effect) error {
	if e.Entity == "" {
		return fmt.Errorf("move: entity is required")
	}
	if e.To == "" {
		return fmt.Errorf("move: to is required")
	}
	en, ok := st.Entities[e.Entity]
	if !ok {
		return fmt.Errorf("move: unknown entity %q", e.Entity)
	}
	attr := e.Attr
	if attr == "" {
		attr = "location"
	}
	et, ok := def.EntityTypes[en.Type]
	if !ok {
		return fmt.Errorf("move: unknown entity type %q", en.Type)
	}
	spec, ok := et.Attributes[attr]
	if !ok {
		return fmt.Errorf("move: entity type %q has no attribute %q", en.Type, attr)
	}
	if spec.Type != "ref" {
		return fmt.Errorf("move: attribute %q on %q is not a ref (got %q)", attr, en.Type, spec.Type)
	}
	if _, ok := st.Entities[e.To]; !ok {
		return fmt.Errorf("move: destination %q does not exist", e.To)
	}
	if e.Via != "" {
		cur, _ := en.Attrs[attr].(string)
		if findRelationship(st, e.Via, cur, e.To) == nil {
			return fmt.Errorf("no %s from %q to %q", e.Via, cur, e.To)
		}
	}
	en.Attrs[attr] = e.To
	return nil
}

// ifOp evaluates e.When; applies e.Then on true else e.Else. Nestable.
func ifOp(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	ok, err := evalGuard(st, ctx, e.When)
	if err != nil {
		return fmt.Errorf("if guard: %w", err)
	}
	branch := e.Else
	if ok {
		branch = e.Then
	}
	for _, sub := range branch {
		if err := applyEffect(def, st, ctx, sub); err != nil {
			return err
		}
	}
	return nil
}

// computeOp evaluates target = A <fn> B where fn ∈ add|sub|mul|div|min|max|mod.
func computeOp(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	spec, err := specForTarget(def, st, ctx, e.Target)
	if err != nil {
		return err
	}
	aVal := resolveValue(st, ctx, e.A)
	bVal := resolveValue(st, ctx, e.B)
	aF, aok := toFloat(aVal)
	bF, bok := toFloat(bVal)
	if !aok {
		return fmt.Errorf("compute: operand a is not numeric (got %T)", aVal)
	}
	if !bok {
		return fmt.Errorf("compute: operand b is not numeric (got %T)", bVal)
	}
	var result float64
	switch e.Fn {
	case "add":
		result = aF + bF
	case "sub":
		result = aF - bF
	case "mul":
		result = aF * bF
	case "div":
		if bF == 0 {
			return fmt.Errorf("compute: div by zero")
		}
		result = aF / bF
	case "min":
		if aF < bF {
			result = aF
		} else {
			result = bF
		}
	case "max":
		if aF > bF {
			result = aF
		} else {
			result = bF
		}
	case "mod":
		if bF == 0 {
			return fmt.Errorf("compute: mod by zero")
		}
		result = float64(int64(aF) % int64(bF))
	default:
		return fmt.Errorf("compute: unknown fn %q (want add|sub|mul|div|min|max|mod)", e.Fn)
	}
	if err := ValidateValue(spec, result); err != nil {
		return fmt.Errorf("compute %s: %w", e.Target, err)
	}
	writeTarget(st, ctx, e.Target, result)
	return nil
}

// setAddOp appends a string value to a set attribute if not already present.
func setAddOp(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	spec, err := specForTarget(def, st, ctx, e.Target)
	if err != nil {
		return err
	}
	if spec.Type != "set" {
		return fmt.Errorf("target %q is not a set", e.Target)
	}
	cur, err := resolvePath(st, ctx, e.Target)
	if err != nil {
		return err
	}
	arr, ok := cur.([]any)
	if !ok {
		return fmt.Errorf("target %q is not a set", e.Target)
	}
	val, ok := e.Value.(string)
	if !ok {
		return fmt.Errorf("add_to value must be a string, got %T", e.Value)
	}
	// For ref-elem sets, validate the entity exists.
	if spec.Elem == "ref" {
		if _, exists := st.Entities[val]; !exists {
			return fmt.Errorf("add_to: entity %q does not exist", val)
		}
	}
	// No-op if already present.
	for _, item := range arr {
		if item == val {
			return nil
		}
	}
	next := append(arr, val)
	if err := ValidateValue(spec, next); err != nil {
		return fmt.Errorf("add_to %s: %w", e.Target, err)
	}
	writeTarget(st, ctx, e.Target, next)
	return nil
}

// setRemoveOp removes a string value from a set attribute if present.
func setRemoveOp(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	spec, err := specForTarget(def, st, ctx, e.Target)
	if err != nil {
		return err
	}
	if spec.Type != "set" {
		return fmt.Errorf("target %q is not a set", e.Target)
	}
	cur, err := resolvePath(st, ctx, e.Target)
	if err != nil {
		return err
	}
	arr, ok := cur.([]any)
	if !ok {
		return fmt.Errorf("target %q is not a set", e.Target)
	}
	val, ok := e.Value.(string)
	if !ok {
		return fmt.Errorf("remove_from value must be a string, got %T", e.Value)
	}
	next := make([]any, 0, len(arr))
	for _, item := range arr {
		if item != val {
			next = append(next, item)
		}
	}
	writeTarget(st, ctx, e.Target, next)
	return nil
}

// setClearOp empties a set attribute.
func setClearOp(def *Definition, st *State, ctx *evalCtx, e Effect) error {
	spec, err := specForTarget(def, st, ctx, e.Target)
	if err != nil {
		return err
	}
	if spec.Type != "set" {
		return fmt.Errorf("target %q is not a set", e.Target)
	}
	writeTarget(st, ctx, e.Target, []any{})
	return nil
}

func equipOp(def *Definition, st *State, e Effect) error {
	en, ok := st.Entities[e.Entity]
	if !ok {
		return fmt.Errorf("unknown entity %q", e.Entity)
	}
	et, ok := def.EntityTypes[en.Type]
	if !ok {
		return fmt.Errorf("unknown entity type %q", en.Type)
	}
	slot, ok := et.Slots[e.Slot]
	if !ok {
		return fmt.Errorf("entity type %q has no slot %q", en.Type, e.Slot)
	}
	it, ok := def.ItemTypes[e.Item]
	if !ok {
		return fmt.Errorf("unknown item type %q", e.Item)
	}
	if !it.Equippable {
		return fmt.Errorf("item %q is not equippable", e.Item)
	}
	if !contains(slot.Accepts, it.Category) {
		return fmt.Errorf("slot %q does not accept category %q", e.Slot, it.Category)
	}
	if en.Equipped == nil {
		en.Equipped = map[string]string{}
	}
	if cur := en.Equipped[e.Slot]; cur != "" {
		return fmt.Errorf("slot %q already holds %q", e.Slot, cur)
	}
	en.Equipped[e.Slot] = e.Item
	return nil
}

func markBeat(def *Definition, st *State, e Effect) error {
	b, ok := def.Beats[e.Beat]
	if !ok {
		return fmt.Errorf("unknown beat %q", e.Beat)
	}
	if !b.once() {
		return nil // repeatable beats are never recorded as delivered
	}
	for _, id := range st.DeliveredBeats {
		if id == e.Beat {
			return nil // idempotent
		}
	}
	st.DeliveredBeats = append(st.DeliveredBeats, e.Beat)
	return nil
}

// resolveValue expands special value forms to concrete values:
//   - {"$roll":"name"} → ctx.rolls[name] (stored roll result)
//   - {"$path":"<path>"} → resolvePath(st, ctx, path) (returns nil on error)
//   - anything else → v unchanged
func resolveValue(st *State, ctx *evalCtx, v any) any {
	if m, ok := v.(map[string]any); ok {
		if name, ok := m["$roll"].(string); ok && ctx != nil {
			return ctx.rolls[name]
		}
		if p, ok := m["$path"].(string); ok {
			val, err := resolvePath(st, ctx, p)
			if err != nil {
				return nil
			}
			return val
		}
	}
	return v
}

// specForTarget returns the VarSpec governing a settable target path
// (this.<attr>, world.<var>, or entity.<id>.<attr>).
func specForTarget(def *Definition, st *State, ctx *evalCtx, path string) (VarSpec, error) {
	parts := splitPath(path)
	switch {
	case len(parts) == 2 && parts[0] == "this":
		if ctx == nil || ctx.host == nil {
			return VarSpec{}, fmt.Errorf("this.* used outside an attached-machine context")
		}
		if ctx.host.kind == "entity" {
			et, ok := def.EntityTypes[ctx.host.ent.Type]
			if !ok {
				return VarSpec{}, fmt.Errorf("unknown entity type %q", ctx.host.ent.Type)
			}
			spec, ok := et.Attributes[parts[1]]
			if !ok {
				return VarSpec{}, fmt.Errorf("unknown attr %q on this", parts[1])
			}
			return spec, nil
		}
		rt, ok := def.RelationshipTypes[ctx.host.rel.Type]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown relationship type %q", ctx.host.rel.Type)
		}
		spec, ok := rt.Attributes[parts[1]]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown rel attr %q on this", parts[1])
		}
		return spec, nil
	case len(parts) == 2 && parts[0] == "world":
		spec, ok := def.World[parts[1]]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown world var %q", parts[1])
		}
		return spec, nil
	case len(parts) == 3 && parts[0] == "entity":
		e, ok := st.Entities[parts[1]]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown entity %q", parts[1])
		}
		et, ok := def.EntityTypes[e.Type]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown entity type %q", e.Type)
		}
		spec, ok := et.Attributes[parts[2]]
		if !ok {
			return VarSpec{}, fmt.Errorf("unknown attr %q", parts[2])
		}
		return spec, nil
	default:
		return VarSpec{}, fmt.Errorf("target %q is not settable", path)
	}
}

func writeTarget(st *State, ctx *evalCtx, path string, v any) {
	parts := splitPath(path)
	switch parts[0] {
	case "world":
		st.World[parts[1]] = v
	case "entity":
		st.Entities[parts[1]].Attrs[parts[2]] = v
	case "this":
		if ctx.host.kind == "entity" {
			ctx.host.ent.Attrs[parts[1]] = v
		} else {
			ctx.host.rel.Attrs[parts[1]] = v
		}
	}
}

func setScalar(def *Definition, st *State, ctx *evalCtx, path string, v any) error {
	spec, err := specForTarget(def, st, ctx, path)
	if err != nil {
		return err
	}
	if spec.Type == "ref" {
		if s, ok := v.(string); ok {
			if _, ok := st.Entities[s]; !ok && s != "" {
				return fmt.Errorf("ref target %q does not exist", s)
			}
		}
	}
	if err := ValidateValue(spec, v); err != nil {
		return fmt.Errorf("set %s: %w", path, err)
	}
	writeTarget(st, ctx, path, v)
	return nil
}

func arithScalar(def *Definition, st *State, ctx *evalCtx, op, path string, rawAmt any) error {
	spec, err := specForTarget(def, st, ctx, path)
	if err != nil {
		return err
	}
	cur, err := resolvePath(st, ctx, path)
	if err != nil {
		return err
	}
	curF, ok := toFloat(cur)
	if !ok {
		return fmt.Errorf("%s is not numeric", path)
	}
	amt, ok := toFloat(resolveValue(st, ctx, rawAmt))
	if !ok {
		return fmt.Errorf("%s amount not numeric", op)
	}
	var next float64
	switch op {
	case "inc":
		next = curF + amt
	case "dec":
		next = curF - amt
	case "mul":
		next = curF * amt
	}
	if err := ValidateValue(spec, next); err != nil {
		return fmt.Errorf("%s %s: %w", op, path, err)
	}
	writeTarget(st, ctx, path, next)
	return nil
}

func inventoryOp(def *Definition, st *State, e Effect) error {
	if _, ok := def.ItemTypes[e.Item]; !ok {
		return fmt.Errorf("unknown item type %q", e.Item)
	}
	en, ok := st.Entities[e.Entity]
	if !ok {
		return fmt.Errorf("unknown entity %q", e.Entity)
	}
	if e.Count < 0 {
		return fmt.Errorf("count must be non-negative")
	}
	if en.Inventory == nil {
		en.Inventory = map[string]int{}
	}
	cur := en.Inventory[e.Item]
	var next int
	if e.Op == "add_item" {
		next = cur + e.Count
		if ms := def.ItemTypes[e.Item].MaxStack; ms != nil && next > *ms {
			return fmt.Errorf("%s would exceed maxStack %d", e.Item, *ms)
		}
	} else {
		next = cur - e.Count
		if next < 0 {
			return fmt.Errorf("not enough %s (have %d, remove %d)", e.Item, cur, e.Count)
		}
	}
	if next == 0 {
		delete(en.Inventory, e.Item)
	} else {
		en.Inventory[e.Item] = next
	}
	return nil
}

// initEntityMachines sets attached entity-machines on a freshly created entity.
func initEntityMachines(def *Definition, e *Entity) {
	for name, m := range def.Machines {
		kind, typ := m.Attach.AttachKind()
		if kind == "entityType" && typ == e.Type {
			if e.Machines == nil {
				e.Machines = map[string]string{}
			}
			e.Machines[name] = m.Initial
		}
	}
}

// initRelMachines sets attached relationship-machines on a freshly created link.
func initRelMachines(def *Definition, r *Relationship) {
	for name, m := range def.Machines {
		kind, typ := m.Attach.AttachKind()
		if kind == "relationshipType" && typ == r.Type {
			if r.Machines == nil {
				r.Machines = map[string]string{}
			}
			r.Machines[name] = m.Initial
		}
	}
}

func createEntity(def *Definition, st *State, e Effect) error {
	et, ok := def.EntityTypes[e.EntityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", e.EntityType)
	}
	if e.ID == "" {
		return fmt.Errorf("create_entity requires id")
	}
	if _, exists := st.Entities[e.ID]; exists {
		return fmt.Errorf("entity %q already exists", e.ID)
	}
	attrs := map[string]any{}
	for name, spec := range et.Attributes {
		attrs[name] = DefaultValue(spec)
	}
	for name, v := range e.Attrs {
		spec, ok := et.Attributes[name]
		if !ok {
			return fmt.Errorf("unknown attr %q for type %q", name, e.EntityType)
		}
		if err := ValidateValue(spec, v); err != nil {
			return fmt.Errorf("attr %q: %w", name, err)
		}
		attrs[name] = v
	}
	st.Entities[e.ID] = &Entity{Type: e.EntityType, Attrs: attrs, Inventory: map[string]int{}}
	initEntityMachines(def, st.Entities[e.ID])
	return nil
}

func relSpec(def *Definition, st *State, e Effect) (RelType, error) {
	rt, ok := def.RelationshipTypes[e.RelType]
	if !ok {
		return RelType{}, fmt.Errorf("unknown relationship type %q", e.RelType)
	}
	for _, id := range []string{e.From, e.To} {
		en, ok := st.Entities[id]
		if !ok {
			return RelType{}, fmt.Errorf("unknown entity %q", id)
		}
		want := rt.From
		if id == e.To {
			want = rt.To
		}
		if en.Type != want {
			return RelType{}, fmt.Errorf("entity %q is %q, relationship needs %q", id, en.Type, want)
		}
	}
	return rt, nil
}

func ensureRelationship(def *Definition, st *State, rt RelType, e Effect) *Relationship {
	if r := findRelationship(st, e.RelType, e.From, e.To); r != nil {
		return r
	}
	attrs := map[string]any{}
	for name, spec := range rt.Attributes {
		attrs[name] = DefaultValue(spec)
	}
	r := &Relationship{Type: e.RelType, From: e.From, To: e.To, Attrs: attrs}
	initRelMachines(def, r)
	st.Relationships = append(st.Relationships, r)
	return r
}

func setRelationship(def *Definition, st *State, e Effect) error {
	rt, err := relSpec(def, st, e)
	if err != nil {
		return err
	}
	r := ensureRelationship(def, st, rt, e)
	for name, v := range e.Attrs {
		spec, ok := rt.Attributes[name]
		if !ok {
			return fmt.Errorf("unknown rel attr %q", name)
		}
		if err := ValidateValue(spec, v); err != nil {
			return fmt.Errorf("rel attr %q: %w", name, err)
		}
		r.Attrs[name] = v
	}
	return nil
}

func adjustRelationship(def *Definition, st *State, e Effect) error {
	rt, err := relSpec(def, st, e)
	if err != nil {
		return err
	}
	spec, ok := rt.Attributes[e.Attr]
	if !ok {
		return fmt.Errorf("unknown rel attr %q", e.Attr)
	}
	if e.By == nil {
		return fmt.Errorf("adjust_relationship requires 'by'")
	}
	r := ensureRelationship(def, st, rt, e)
	cur, _ := toFloat(r.Attrs[e.Attr])
	next := cur + *e.By
	if err := ValidateValue(spec, next); err != nil {
		return fmt.Errorf("rel attr %q: %w", e.Attr, err)
	}
	r.Attrs[e.Attr] = next
	return nil
}

func removeRelationship(st *State, e Effect) error {
	for i, r := range st.Relationships {
		if r.Type == e.RelType && r.From == e.From && r.To == e.To {
			st.Relationships = append(st.Relationships[:i], st.Relationships[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no relationship %s %s->%s", e.RelType, e.From, e.To)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func rollOp(ctx *evalCtx, e Effect) error {
	if ctx.rng == nil {
		return fmt.Errorf("roll requires an RNG")
	}
	v, err := ctx.rng.RollDice(e.Dice)
	if err != nil {
		return err
	}
	if e.Store != "" {
		ctx.rolls[e.Store] = float64(v)
	}
	ctx.record = append(ctx.record, RollResult{Store: e.Store, Dice: e.Dice, Result: float64(v)})
	return nil
}

func setAttachedState(def *Definition, st *State, e Effect) error {
	m, ok := def.Machines[e.Machine]
	if !ok || m.Attach == nil {
		return fmt.Errorf("%q is not an attached machine", e.Machine)
	}
	if !contains(m.States, e.State) {
		return fmt.Errorf("state %q not in machine %q", e.State, e.Machine)
	}
	kind, _ := m.Attach.AttachKind()
	switch kind {
	case "entityType":
		en, ok := st.Entities[e.Entity]
		if !ok {
			return fmt.Errorf("unknown entity %q", e.Entity)
		}
		if en.Machines == nil {
			en.Machines = map[string]string{}
		}
		en.Machines[e.Machine] = e.State
	case "relationshipType":
		r := findRelationship(st, machineRelType(m), e.From, e.To)
		if r == nil {
			return fmt.Errorf("no %s relationship %s->%s", machineRelType(m), e.From, e.To)
		}
		if r.Machines == nil {
			r.Machines = map[string]string{}
		}
		r.Machines[e.Machine] = e.State
	default:
		return fmt.Errorf("machine %q has malformed attach", e.Machine)
	}
	return nil
}

// machineRelType returns the relationship type an attached machine targets.
func machineRelType(m Machine) string {
	_, name := m.Attach.AttachKind()
	return name
}
