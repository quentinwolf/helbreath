package game

// Difficulty multipliers — adjust these to make the game easier or harder.
// Set via environment variable DIFFICULTY_MULTIPLIER (default "3" for 3x easier).
// A value of 1.0 = original C++ difficulty. Higher = easier for players.
var (
	// HitBonus is added to hit chance percentage (0 = no bonus).
	HitBonus = 50

	// DamageMultiplier scales player damage output.
	DamageMultiplier = 4

	// DropRateMultiplier scales loot drop chances.
	// Medium drop rate: 2x base.
	DropRateMultiplier = 2.0

	// XPMultiplier scales experience gained from kills, on top of the original's
	// own low-level boost (killXP). 1 = the original rate (it was 5).
	XPMultiplier = 1.0

	// RandomMobShare is the default cap on a map's random monsters, as a share
	// of its maximum-object (a map's random-mob-max overrides it); 0 = off.
	// The original filled maps up to maximum-object - 30 (570 on Elvine), far
	// more than the occasional wandering monster Zac remembers.
	RandomMobShare = 0.20

	// NPCDamageReduction reduces damage NPCs deal to players.
	NPCDamageReduction = 3

	// EKMultiplier scales Enemy Kill point rewards.
	// Tiered: x10 base, x20 in crusade, x30 in special events.
	EKMultiplierBase    = 10
	EKMultiplierCrusade = 20
	EKMultiplierEvent   = 30
)
