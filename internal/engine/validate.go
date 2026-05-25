package engine

import "fmt"

type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string { return e.Path + ": " + e.Message }

// ValidateDefinition checks a definition for internal consistency. It returns
// all problems found (empty slice means valid).
func ValidateDefinition(def *Definition) []ValidationError {
	var errs []ValidationError
	add := func(path, msg string) { errs = append(errs, ValidationError{Path: path, Message: msg}) }

	if def.ID == "" {
		add("id", "game id is required")
	}
	for name, spec := range def.World {
		errs = append(errs, validateSpec(fmt.Sprintf("world.%s", name), spec)...)
	}
	for tName, et := range def.EntityTypes {
		for aName, spec := range et.Attributes {
			errs = append(errs, validateSpec(fmt.Sprintf("entityTypes.%s.%s", tName, aName), spec)...)
			if spec.Type == "ref" && spec.RefType != "" {
				if _, ok := def.EntityTypes[spec.RefType]; !ok {
					add(fmt.Sprintf("entityTypes.%s.%s", tName, aName), "refType "+spec.RefType+" is not a defined entity type")
				}
			}
		}
	}
	for rName, rt := range def.RelationshipTypes {
		if _, ok := def.EntityTypes[rt.From]; !ok {
			add("relationshipTypes."+rName+".from", rt.From+" is not a defined entity type")
		}
		if _, ok := def.EntityTypes[rt.To]; !ok {
			add("relationshipTypes."+rName+".to", rt.To+" is not a defined entity type")
		}
		for aName, spec := range rt.Attributes {
			errs = append(errs, validateSpec(fmt.Sprintf("relationshipTypes.%s.%s", rName, aName), spec)...)
		}
	}
	for mName, m := range def.Machines {
		if m.Attach != nil {
			kind, name := m.Attach.AttachKind()
			switch kind {
			case "relationshipType":
				if _, ok := def.RelationshipTypes[name]; !ok {
					add("machines."+mName+".attach", "unknown relationship type "+name)
				}
			case "entityType":
				if _, ok := def.EntityTypes[name]; !ok {
					add("machines."+mName+".attach", "unknown entity type "+name)
				}
			default:
				add("machines."+mName+".attach", `to must be "relationshipType:<name>" or "entityType:<name>"`)
			}
		}
		if !contains(m.States, m.Initial) {
			add("machines."+mName+".initial", "initial state "+m.Initial+" not in states")
		}
		for _, tr := range m.Transitions {
			if !contains(m.States, tr.To) {
				add(fmt.Sprintf("machines.%s.%s.to", mName, tr.ID), "to-state "+tr.To+" not in states")
			}
			for _, fs := range tr.From {
				if fs != "*" && !contains(m.States, fs) {
					add(fmt.Sprintf("machines.%s.%s.from", mName, tr.ID), "from-state "+fs+" not in states")
				}
			}
		}
		for sName := range m.StateMeta {
			if !contains(m.States, sName) {
				add("machines."+mName+".stateMeta", "state "+sName+" not in states")
			}
		}
	}
	for name, d := range def.Derived {
		path := "derived." + name
		if d.Over != "relationships" && d.Over != "entities" {
			add(path+".over", `must be "relationships" or "entities"`)
		}
		if d.Over == "relationships" && d.Where.Type != "" {
			if _, ok := def.RelationshipTypes[d.Where.Type]; !ok {
				add(path+".where.type", d.Where.Type+" is not a defined relationship type")
			}
		}
		if d.Over == "entities" && d.Where.Type != "" {
			if _, ok := def.EntityTypes[d.Where.Type]; !ok {
				add(path+".where.type", d.Where.Type+" is not a defined entity type")
			}
		}
		if !validReduce(d.Reduce) {
			add(path+".reduce", "invalid reduce "+d.Reduce)
		}
		// Referenced attributes must exist on the filtered type. Only checkable
		// when where.type names a known type.
		if d.Where.Type != "" {
			var attrs map[string]VarSpec
			if d.Over == "relationships" {
				if rt, ok := def.RelationshipTypes[d.Where.Type]; ok {
					attrs = rt.Attributes
				}
			} else if d.Over == "entities" {
				if et, ok := def.EntityTypes[d.Where.Type]; ok {
					attrs = et.Attributes
				}
			}
			if attrs != nil {
				if _, rattr := splitReduce(d.Reduce); rattr != "" {
					if _, ok := attrs[rattr]; !ok {
						add(path+".reduce", "attribute "+rattr+" is not defined on "+d.Where.Type)
					}
				}
				for _, p := range d.Where.Attrs {
					if _, ok := attrs[p.Attr]; !ok {
						add(path+".where.attrs", "attribute "+p.Attr+" is not defined on "+d.Where.Type)
					}
				}
			}
		}
	}
	// Validate the first-class cast.
	for id, e := range def.Entities {
		path := fmt.Sprintf("entities.%s", id)
		et, ok := def.EntityTypes[e.Type]
		if !ok {
			add(path+".type", fmt.Sprintf("unknown entity type %q", e.Type))
		} else {
			// Validate attrs.
			for k, v := range e.Attrs {
				spec, ok := et.Attributes[k]
				if !ok {
					add(fmt.Sprintf("%s.attrs.%s", path, k), fmt.Sprintf("unknown attribute %q on entity type %q", k, e.Type))
					continue
				}
				if err := ValidateValue(spec, v); err != nil {
					add(fmt.Sprintf("%s.attrs.%s", path, k), err.Error())
				}
			}
			// Validate inventory items.
			for item := range e.Inventory {
				if _, ok := def.ItemTypes[item]; !ok {
					add(fmt.Sprintf("%s.inventory.%s", path, item), fmt.Sprintf("unknown item type %q", item))
				}
			}
			// Validate equipped slots.
			for slot, item := range e.Equipped {
				slotSpec, ok := et.Slots[slot]
				if !ok {
					add(fmt.Sprintf("%s.equipped.%s", path, slot), fmt.Sprintf("entity type %q has no slot %q", e.Type, slot))
					continue
				}
				it, ok := def.ItemTypes[item]
				if !ok {
					add(fmt.Sprintf("%s.equipped.%s", path, slot), fmt.Sprintf("unknown item type %q", item))
					continue
				}
				if !it.Equippable {
					add(fmt.Sprintf("%s.equipped.%s", path, slot), fmt.Sprintf("item %q is not equippable", item))
				}
				if !contains(slotSpec.Accepts, it.Category) {
					add(fmt.Sprintf("%s.equipped.%s", path, slot), fmt.Sprintf("slot %q does not accept category %q (item %q)", slot, it.Category, item))
				}
			}
		}
	}
	for i, r := range def.Relationships {
		path := fmt.Sprintf("relationships[%d]", i)
		if _, ok := def.RelationshipTypes[r.Type]; !ok {
			add(path+".type", fmt.Sprintf("unknown relationship type %q", r.Type))
		}
		if _, ok := def.Entities[r.From]; !ok {
			add(path+".from", fmt.Sprintf("entity %q is not in def.Entities", r.From))
		}
		if _, ok := def.Entities[r.To]; !ok {
			add(path+".to", fmt.Sprintf("entity %q is not in def.Entities", r.To))
		}
		if rt, ok := def.RelationshipTypes[r.Type]; ok {
			for k, v := range r.Attrs {
				spec, ok := rt.Attributes[k]
				if !ok {
					add(fmt.Sprintf("%s.attrs.%s", path, k), fmt.Sprintf("unknown attribute %q on relationship type %q", k, r.Type))
					continue
				}
				if err := ValidateValue(spec, v); err != nil {
					add(fmt.Sprintf("%s.attrs.%s", path, k), err.Error())
				}
			}
		}
	}

	for id, entry := range def.Lore {
		path := fmt.Sprintf("lore.%s", id)
		if entry.Title == "" {
			add(path+".title", "lore title is required")
		}
		if entry.Text == "" {
			add(path+".text", "lore text is required")
		}
	}

	for id, b := range def.Beats {
		path := "beats." + id
		if b.Text == "" {
			add(path+".text", "beat text is required")
		}
		if b.MachineState != nil {
			m, ok := def.Machines[b.MachineState.Machine]
			if !ok {
				add(path+".machineState.machine", "unknown machine "+b.MachineState.Machine)
			} else if m.Attach != nil {
				add(path+".machineState.machine", b.MachineState.Machine+" is an attached machine; beats bind to global machines only")
			} else if !contains(m.States, b.MachineState.State) {
				add(path+".machineState.state", "state "+b.MachineState.State+" not in machine "+b.MachineState.Machine)
			}
		}
	}

	// Validate effects in setup and machine transitions (new ops: compute, if).
	for i, e := range def.Setup {
		errs = append(errs, validateEffect(fmt.Sprintf("setup[%d]", i), e)...)
	}
	for mName, m := range def.Machines {
		for _, tr := range m.Transitions {
			for i, e := range tr.Effects {
				errs = append(errs, validateEffect(fmt.Sprintf("machines.%s.%s.effects[%d]", mName, tr.ID, i), e)...)
			}
		}
	}

	// Validate triggers.
	for tName, trig := range def.Triggers {
		path := "triggers." + tName
		if trig.When == nil && trig.Every == 0 {
			add(path, "trigger needs when or every")
		}
		for i, e := range trig.Effects {
			errs = append(errs, validateEffect(fmt.Sprintf("%s.effects[%d]", path, i), e)...)
		}
	}

	// Movement lint (def-aware): every move destination must resolve to an entity
	// known at runtime, and the attr it moves along must be a ref. Catches typo'd
	// destinations and moves down a non-ref attribute at import, before play.
	// "Known" = the authored cast plus anything create_entity introduces, so
	// dynamically created destinations are not false-flagged.
	known := map[string]bool{}
	for id := range def.Entities {
		known[id] = true
	}
	collectCreatedEntityIDs(def.Setup, known)
	for _, m := range def.Machines {
		for _, tr := range m.Transitions {
			collectCreatedEntityIDs(tr.Effects, known)
		}
	}
	for _, trig := range def.Triggers {
		collectCreatedEntityIDs(trig.Effects, known)
	}
	errs = append(errs, validateMoveEffects(def, "setup", def.Setup, known)...)
	for mName, m := range def.Machines {
		for _, tr := range m.Transitions {
			errs = append(errs, validateMoveEffects(def, fmt.Sprintf("machines.%s.%s.effects", mName, tr.ID), tr.Effects, known)...)
		}
	}
	for tName, trig := range def.Triggers {
		errs = append(errs, validateMoveEffects(def, "triggers."+tName+".effects", trig.Effects, known)...)
	}

	return errs
}

// collectCreatedEntityIDs records every entity id a create_entity op would
// introduce, recursing into if/schedule branches.
func collectCreatedEntityIDs(effs []Effect, into map[string]bool) {
	for _, e := range effs {
		switch e.Op {
		case "create_entity":
			if e.ID != "" {
				into[e.ID] = true
			}
		case "if":
			collectCreatedEntityIDs(e.Then, into)
			collectCreatedEntityIDs(e.Else, into)
		case "schedule":
			collectCreatedEntityIDs(e.Do, into)
		}
	}
}

// validateMoveEffects checks each move op's destination is a known entity and
// its move attribute is a ref on the mover's type. Recurses into if/schedule.
func validateMoveEffects(def *Definition, path string, effs []Effect, known map[string]bool) []ValidationError {
	var errs []ValidationError
	for i, e := range effs {
		p := fmt.Sprintf("%s[%d]", path, i)
		switch e.Op {
		case "move":
			if e.To != "" && !known[e.To] {
				errs = append(errs, ValidationError{p + ".to", fmt.Sprintf("move destination %q is not a defined entity", e.To)})
			}
			attr := e.Attr
			if attr == "" {
				attr = "location"
			}
			if ei, ok := def.Entities[e.Entity]; ok {
				if et, ok := def.EntityTypes[ei.Type]; ok {
					spec, ok := et.Attributes[attr]
					if !ok {
						errs = append(errs, ValidationError{p + ".attr", fmt.Sprintf("mover %q (type %q) has no attribute %q to move along", e.Entity, ei.Type, attr)})
					} else if spec.Type != "ref" {
						errs = append(errs, ValidationError{p + ".attr", fmt.Sprintf("move attribute %q on type %q is not a ref (got %q)", attr, ei.Type, spec.Type)})
					}
				}
			}
		case "if":
			errs = append(errs, validateMoveEffects(def, p+".then", e.Then, known)...)
			errs = append(errs, validateMoveEffects(def, p+".else", e.Else, known)...)
		case "schedule":
			errs = append(errs, validateMoveEffects(def, p+".do", e.Do, known)...)
		}
	}
	return errs
}

// validateEffect checks the new compute and if ops for structural correctness.
// It recurses into if.Then and if.Else so nested effects are also checked.
func validateEffect(path string, e Effect) []ValidationError {
	var errs []ValidationError
	add := func(p, msg string) { errs = append(errs, ValidationError{Path: p, Message: msg}) }

	switch e.Op {
	case "compute":
		validFns := map[string]bool{"add": true, "sub": true, "mul": true, "div": true, "min": true, "max": true, "mod": true}
		if !validFns[e.Fn] {
			add(path+".fn", fmt.Sprintf("compute fn %q is not valid (want add|sub|mul|div|min|max|mod)", e.Fn))
		}
		if e.A == nil {
			add(path+".a", "compute requires operand a")
		}
		if e.B == nil {
			add(path+".b", "compute requires operand b")
		}
	case "if":
		if e.When == nil {
			add(path+".when", "if effect requires a when guard")
		}
		for i, sub := range e.Then {
			errs = append(errs, validateEffect(fmt.Sprintf("%s.then[%d]", path, i), sub)...)
		}
		for i, sub := range e.Else {
			errs = append(errs, validateEffect(fmt.Sprintf("%s.else[%d]", path, i), sub)...)
		}
	case "schedule":
		if e.In <= 0 {
			add(path+".in", fmt.Sprintf("schedule in must be > 0 (got %d)", e.In))
		}
		for i, sub := range e.Do {
			errs = append(errs, validateEffect(fmt.Sprintf("%s.do[%d]", path, i), sub)...)
		}
	case "cooldown":
		if e.Key == "" {
			add(path+".key", "cooldown key must not be empty")
		}
		if e.Ticks <= 0 {
			add(path+".ticks", fmt.Sprintf("cooldown ticks must be > 0 (got %d)", e.Ticks))
		}
	case "record":
		if e.Text == "" {
			add(path+".text", "record text must not be empty")
		}
	case "move":
		if e.Entity == "" {
			add(path+".entity", "move requires entity")
		}
		if e.To == "" {
			add(path+".to", "move requires to")
		}
	case "discover":
		if e.Lore == "" {
			add(path+".lore", "discover requires lore id")
		}
	case "check":
		if e.Dice == "" {
			add(path+".dice", "check requires dice (e.g. \"1d20\")")
		}
		if e.Store == "" {
			add(path+".store", "check requires a store key (read the result as check.<store>.*)")
		}
		if e.DC == nil {
			add(path+".dc", "check requires a dc (a number, or {\"$path\":\"…\"} for an opposed check)")
		}
	}
	return errs
}

// validateSpec checks a single VarSpec for self-consistency.
func validateSpec(path string, spec VarSpec) []ValidationError {
	var errs []ValidationError
	switch spec.Type {
	case "int", "float", "bool", "string", "ref":
	case "enum":
		if len(spec.Values) == 0 {
			errs = append(errs, ValidationError{path, "enum requires values"})
		}
	case "set":
		if spec.Elem != "string" && spec.Elem != "ref" {
			errs = append(errs, ValidationError{path + ".elem", "set elem must be \"string\" or \"ref\", got \"" + spec.Elem + "\""})
		}
	default:
		errs = append(errs, ValidationError{path, "unknown type " + spec.Type})
		return errs
	}
	if spec.Default != nil {
		if err := ValidateValue(spec, spec.Default); err != nil {
			errs = append(errs, ValidationError{path + ".default", err.Error()})
		}
	}
	return errs
}

// validReduce reports whether a derived reduce verb is recognized.
func validReduce(reduce string) bool {
	verb, attr := splitReduce(reduce)
	switch verb {
	case "count", "any", "list":
		return attr == ""
	case "sum", "min", "max", "argmax", "argmin":
		return attr != ""
	}
	return false
}
