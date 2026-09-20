package main

import (
	"rpg/common"
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
