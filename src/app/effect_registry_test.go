package app

import (
	"math/rand"
	"rpg/src/common"
	"rpg/src/util"
	"strings"
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
	declares := 0
	for spell, sp := range factory.Generator.D.Spells {
		declared, zoneName := sp.Effect, sp.HitZone
		if len(declared) == 0 {
			continue
		}
		declares++
		zone := common.HitZoneFromString(factory.State.GetMap, zoneName)
		if zone == nil || zone.GetHitPositions == nil {
			t.Errorf("spell %d: hit zone %q did not resolve", spell, zoneName)
		}
		// a bare name resolving to the fallback zone is how a typo hides: a misspelt zone silently becomes
		// target_only. Anything that looks parametrized must name a zone the registry actually knows.
		if base := zoneName; base != "" {
			if util.LooksLikeAFunction(base) {
				base, _ = util.GetNameAndArgs(base)
			}
			if _, known := common.GetAllHitZones()[base]; !known {
				t.Errorf("spell %d: hit zone %q is not a registered zone", spell, zoneName)
			}
		}
		for _, effectName := range declared {
			name, args := effectName, util.Arguments{}
			if util.LooksLikeAFunction(effectName) {
				name, args = util.GetNameAndArgs(effectName)
			}
			if _, exists := factory.GetAllHitEffects()[name]; !exists {
				t.Errorf("spell %d: effect %q missing from GetAllHitEffects", spell, name)
				continue
			}
			// the cast appends the caster's spell level, so resolve the way castFromData does
			got := factory.NewOnHitEffectFromNameAndArgs(name, append(args, "1"))
			if got == nil || got.Activate == nil {
				t.Errorf("spell %d: effect %q resolved to a nil or inert HitEffect", spell, effectName)
				continue
			}
			// on_self hands its remaining arguments to the untargeted registry, so a name that is wrong there
			// resolves to nil in a way only a cast would notice.
			if name == "on_self" {
				if _, known := factory.GetAllUntargetedEffects()[args.Get(0)]; !known {
					t.Errorf("spell %d: on_self names %q, which is not a registered untargeted effect", spell, args.Get(0))
				}
			}
			// arguments split on '|', not ','. A mapping written with commas resolves its first argument to
			// the whole list and its later ones to nothing, which every effect below survives silently. So
			// insist that an effect declaring arguments really got the count it wrote.
			if util.LooksLikeAFunction(effectName) {
				if want := strings.Count(effectName[strings.Index(effectName, "(")+1:len(effectName)-1], "|") + 1; len(args) != want {
					t.Errorf("spell %d: effect %q split into %d arguments, want %d", spell, effectName, len(args), want)
				}
				if strings.Contains(effectName, ",") && len(args) == 1 {
					t.Errorf("spell %d: effect %q separates its arguments with ',' but they split on '|'", spell, effectName)
				}
			}
		}
	}
	if declares == 0 {
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
