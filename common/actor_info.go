package common

import (
	"fmt"
	"rpg/stats"
)

// ActorInfoSections is the right-click description of an actor: who they are, their numbers, what is on them, and
// - against a monster - how the fight would go. Both UIs read it, the tile UI as a laid out window and the
// terminal one through SectionLines, so the two can't drift apart.
func ActorInfoSections(state *State, actor *Actor) (string, []D1SheetSection) {
	var sections []D1SheetSection
	switch {
	case actor.hero != nil:
		sections = append(actor.D1SheetSections(), actor.d1AttributeSection())
	case actor.d1Monster != nil:
		sections = actor.D1MonsterSections()
	default:
		sections = actor.plainActorSections()
	}
	if effects := actor.statusEffects.AsStrings(); len(effects) > 0 {
		sections = append(sections, D1SheetSection{Title: "Afflictions", Notes: effects})
	}
	if player := state.GetPlayer(); actor != player {
		if forecast, ok := D1ForecastSection(state, player, actor); ok {
			sections = append(sections, forecast)
		}
	}
	return actor.name, sections
}

// SectionLines flattens an info window into plain lines for the terminal UI.
func SectionLines(title string, sections []D1SheetSection) []string {
	lines := []string{title}
	for _, sec := range sections {
		if len(sec.Rows) == 0 && len(sec.Notes) == 0 {
			continue
		}
		lines = append(lines, "", sec.Title)
		for _, row := range sec.Rows {
			lines = append(lines, fmt.Sprintf(" %s: %s", row[0], row[1]))
		}
		for _, note := range sec.Notes {
			lines = append(lines, " "+note)
		}
	}
	return lines
}

// d1AttributeSection is the four attributes as a read-only block, for the info window (the character sheet draws
// them itself, with a "+" per attribute a point can still go into).
func (a *Actor) d1AttributeSection() D1SheetSection {
	sec := D1SheetSection{Title: "Attributes"}
	for _, at := range a.D1SheetAttributes() {
		value := fmt.Sprintf("%d / %d", at.Base, at.Max)
		if at.Total != at.Base {
			value = fmt.Sprintf("%d (%d) / %d", at.Base, at.Total, at.Max)
		}
		sec.Rows = append(sec.Rows, [2]string{at.Label, value})
	}
	return sec
}

// plainActorSections describes an actor that is neither a hero nor a D1 monster: a townsperson, who only has
// numbers worth reading once they have been provoked into fighting back.
func (a *Actor) plainActorSections() []D1SheetSection {
	st := a.GetStats()
	kind := "Townsperson"
	if a.IsVendor() {
		kind = "Vendor"
	}
	rows := [][2]string{
		{"Kind", kind},
		{"Life", st.GetResource(stats.Health).ToString()},
		{"Armor class", fmt.Sprintf("%d", a.GetArmorClass())},
		{"Damage reduction", fmt.Sprintf("%d%%", int(a.GetArmorDamageReduction()*100))},
		{"Melee damage", a.GetDamageDice(MeleeAttack).String()},
	}
	if a.GetEquipment().HasRangedWeaponEquipped() {
		rows = append(rows, [2]string{"Ranged damage", a.GetDamageDice(RangedAttack).String()})
	}
	sections := []D1SheetSection{{Title: "Creature", Rows: rows}}
	var carried []string
	for _, it := range a.GetEquipment().AllItems() {
		carried = append(carried, it.Name())
	}
	if len(carried) > 0 {
		sections = append(sections, D1SheetSection{Title: "Equipment", Notes: carried})
	}
	return sections
}
