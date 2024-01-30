package game

// Design System for Magic effects
// We have

// UntargetedActions (e.g. blessing)
// - Cannot be targeted
// - Can be delayed
// - Will be executed in the context of the actor that uses it
// - Example Usage: "action: blessing"

// TargetedActions (e.g. fireball)
// - Can be targeted
// - Cannot be delayed (currently)
// - Must define a HitZone for targeting
// - Must define TargetingRules (e.g. can only target enemies, range, needs LoS, etc.)
// - Will receive the attacker, the target position, and the hit locations
// - Example Usage: "action_targeted: fireball(5, 10)" // radius, damage
// - Example Usage: "action_targeted: apply_status(confused, 3)" // status, turns

// StatusEffects (e.g. confused)
// - Can be applied to an actor
// - Can be temporary or permanent
// - Can be parameterized (e.g. poison damage) - NOT YET IMPLEMENTED
// - Will have a direct impact on game mechanics

// Modifiers (e.g. +1 to strength)
// - Are only applied by equipment, and are active only during wearing
// - Will provide bonus/malus to character stats
// - Example Usage: "modifier: strength(1)"
// - Example Usage: "modifier: dexterity(2-4)"

// ItemHooks (e.g. on hit, on kill)
// - Become active once the item is equipped
// - Can change game mechanics in dramatic ways
// - Used for OnHit, OnKill, etc.
// - They can usually accept both TargetedActions and UntargetedActions
// - Example Usage: "hook: on_hit(apply_status(confused, 3))"

