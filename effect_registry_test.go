package main

import (
	"math/rand"
	"rpg/common"
	"rpg/d1"
	"rpg/util"
	"testing"
)

// Phase 2 of the engine/content split restored the H&H effect/action engine. Every named effect it
// provides must still resolve through the factory lookups. One test, by name, non-nil.
func TestRestoredEffectsResolveByName(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[1], false, "")
	factory := theGame.GetFactory()

	untargeted := []string{
		"replicate_with",        // clone self
		"heal_health_mana_full", // heal health+mana
		"heal_health",
		"apply_berserk_self", // berserk
		"apply_status_self",
		"heal_mana",
		"random_near_teleport", // phase door
		"buff_damage",
		"buff_speed",
		"recall",
		"buff_armor", // bless
		"enchant_weapon",
		"enchant_armor",
		"multi_shot",
		"telefrag",
		"stoneskin",
		"acquirement",
		"earthquake",
	}
	for _, name := range untargeted {
		if got := factory.NewUntargetedActionFromName(name); got == nil {
			t.Errorf("untargeted effect %q did not resolve through NewUntargetedActionFromName", name)
		}
	}

	targeted := []string{
		"steal_item",
		"steal_item_teleport",
		"mind_push",
		"fireball",
		"project_fire", // fire ray
		"fire_zone",    // fire bresen / fire wall
		"blink",
		"confuse",
		"poison",
		"immobilize",
		"jumpstart",
		"earthshaker",
		"summon_monster",
		"berserk",
	}
	hitEffects := factory.GetAllHitEffects()
	for _, name := range targeted {
		if _, exists := hitEffects[name]; !exists {
			t.Errorf("targeted effect %q missing from GetAllHitEffects", name)
			continue
		}
		if got := factory.NewOnHitEffectFromNameAndArgs(name, util.Arguments{}); got == nil {
			t.Errorf("targeted effect %q resolved to a nil HitEffect", name)
		}
	}

	// Phase 5 folded the D1 spells into this same model: each one names a hit zone and an on-hit effect in its
	// content, and nothing resolves through a spell enum any more. Every mapping the content declares must
	// produce a zone that answers with positions and a non-nil effect.
	for spell, effectName := range d1.SpellEffect {
		zoneName := d1.SpellHitZone[spell]
		zone := common.HitZoneFromString(factory.State.GetMap, zoneName)
		if zone == nil || zone.GetHitPositions == nil {
			t.Errorf("spell %d: hit zone %q did not resolve", spell, zoneName)
		}
		name, args := effectName, util.Arguments{}
		if util.LooksLikeAFunction(effectName) {
			name, args = util.GetNameAndArgs(effectName)
		}
		if _, exists := factory.GetAllHitEffects()[name]; !exists {
			t.Errorf("spell %d: effect %q missing from GetAllHitEffects", spell, name)
			continue
		}
		// the cast appends the caster's spell level, so resolve the way castFromData does
		if got := factory.NewOnHitEffectFromNameAndArgs(name, append(args, "1")); got == nil || got.Activate == nil {
			t.Errorf("spell %d: effect %q resolved to a nil or inert HitEffect", spell, effectName)
		}
	}
	if len(d1.SpellEffect) == 0 {
		t.Error("no D1 spell declares an effect; spells.rec did not load")
	}
	for name, roll := range common.GetAllDamageRolls() {
		if got := roll(common.RollContext{Rng: rand.New(rand.NewSource(1)), Mag: 30, Clvl: 5, Slvl: 2}); got.Ticks < 1 {
			t.Errorf("damage roll %q produced %d ticks, want at least 1", name, got.Ticks)
		}
	}

	// item hooks: fireball on ranged attack, mana on kill, return damage, chance of triple melee damage
	hooks := common.NewHooksForItem(factory.RandomSource, 1, 4)
	if len(hooks) != 4 {
		t.Fatalf("expected all 4 item hooks, got %d", len(hooks))
	}
	for i, h := range hooks {
		if h == nil {
			t.Errorf("item hook %d is nil", i)
		}
	}
}
