package engine

import "fmt"

// AddCharacter adds or replaces an entity in def.Entities. It returns an error
// if id or typ is empty. Nil maps on the definition are initialized. The caller
// is responsible for re-validating and saving the definition after the call.
func AddCharacter(def *Definition, id, typ string, attrs map[string]any, description string) error {
	if id == "" {
		return fmt.Errorf("character id is required")
	}
	if typ == "" {
		return fmt.Errorf("character type is required")
	}
	if def.Entities == nil {
		def.Entities = map[string]EntityInit{}
	}
	def.Entities[id] = EntityInit{Type: typ, Attrs: attrs, Description: description}
	return nil
}

// RemoveCharacter removes the entity with the given id from def.Entities and
// drops any Relationships that reference it (either as From or To). It returns
// an error if the entity is not present. The caller is responsible for
// re-validating and saving the definition after the call.
func RemoveCharacter(def *Definition, id string) error {
	if _, ok := def.Entities[id]; !ok {
		return fmt.Errorf("character %q not found", id)
	}
	delete(def.Entities, id)
	// Remove any relationships that reference this entity.
	kept := def.Relationships[:0]
	for _, r := range def.Relationships {
		if r.From != id && r.To != id {
			kept = append(kept, r)
		}
	}
	def.Relationships = kept
	return nil
}

// AddRelationship upserts a relationship into def.Relationships. If a
// relationship with the same type, from, and to already exists it is replaced;
// otherwise a new entry is appended. The caller is responsible for
// re-validating and saving the definition after the call.
func AddRelationship(def *Definition, typ, from, to string, attrs map[string]any) error {
	if typ == "" {
		return fmt.Errorf("relationship type is required")
	}
	if from == "" {
		return fmt.Errorf("relationship from is required")
	}
	if to == "" {
		return fmt.Errorf("relationship to is required")
	}
	for i, r := range def.Relationships {
		if r.Type == typ && r.From == from && r.To == to {
			def.Relationships[i] = RelInit{Type: typ, From: from, To: to, Attrs: attrs}
			return nil
		}
	}
	def.Relationships = append(def.Relationships, RelInit{Type: typ, From: from, To: to, Attrs: attrs})
	return nil
}

// RemoveRelationship removes the first relationship matching type, from, and
// to from def.Relationships. It returns an error if no match is found. The
// caller is responsible for re-validating and saving the definition after the
// call.
func RemoveRelationship(def *Definition, typ, from, to string) error {
	for i, r := range def.Relationships {
		if r.Type == typ && r.From == from && r.To == to {
			def.Relationships = append(def.Relationships[:i], def.Relationships[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no %s relationship %s->%s found", typ, from, to)
}

// GiveItem adds inventory and/or equips an item for a cast entity. If count >
// 0 the item is added to the entity's Inventory; if equipSlot is non-empty the
// entity's Equipped map is set for that slot. It returns an error if charID is
// not present in def.Entities. The caller is responsible for re-validating and
// saving the definition after the call.
func GiveItem(def *Definition, charID, item string, count int, equipSlot string) error {
	e, ok := def.Entities[charID]
	if !ok {
		return fmt.Errorf("character %q not found", charID)
	}
	if count > 0 {
		if e.Inventory == nil {
			e.Inventory = map[string]int{}
		}
		e.Inventory[item] += count
	}
	if equipSlot != "" {
		if e.Equipped == nil {
			e.Equipped = map[string]string{}
		}
		e.Equipped[equipSlot] = item
	}
	def.Entities[charID] = e
	return nil
}
