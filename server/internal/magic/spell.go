package magic

// SpellType categorizes spells.
type SpellType int

const (
	SpellTypeDamage   SpellType = 1 // Damage on the clicked tile only
	SpellTypeAOE      SpellType = 2 // Damage over an area or line around the clicked tile
	SpellTypeHeal     SpellType = 3 // Healing
	SpellTypeBuff     SpellType = 4 // Positive buff
	SpellTypeDebuff   SpellType = 5 // Negative debuff
	SpellTypeSummon   SpellType = 6 // Summon creature
	SpellTypeTeleport SpellType = 7 // Teleport
)

// SpellElement defines the elemental affinity.
type SpellElement int

const (
	ElementNone  SpellElement = 0
	ElementFire  SpellElement = 1
	ElementIce   SpellElement = 2
	ElementLight SpellElement = 3
	ElementEarth SpellElement = 4
)

// MaxCastRange is how far away (in tiles, either axis) a targeted spell can be
// cast. The original game let you cast at anything on screen; its 640x480 view
// was 20 tiles wide, so 10 tiles each side.
const MaxCastRange = 10

// Dice is an original-style damage roll: Count dice of Sides sides, plus Bonus.
type Dice struct {
	Count, Sides, Bonus int
}

// IsZero reports whether the roll deals no damage.
func (d Dice) IsZero() bool { return d.Count <= 0 && d.Bonus <= 0 }

// Min and Max are the lowest and highest possible rolls.
func (d Dice) Min() int {
	if d.Sides <= 0 {
		return d.Bonus
	}
	return d.Count + d.Bonus
}

func (d Dice) Max() int {
	if d.Sides <= 0 {
		return d.Bonus
	}
	return d.Count*d.Sides + d.Bonus
}

// SpellDef is a static spell definition, generated from the original Magic.cfg.
type SpellDef struct {
	ID         int
	Name       string
	Type       SpellType
	Element    SpellElement
	ManaCost   int
	CastTime   int // ms
	Cooldown   int // ms
	Range      int // cast range in tiles (0 = self)
	Radius     int // largest of AreaX/AreaY, kept for older callers (0 = single tile)
	MinDamage  int // what the main target can take (centre + area dice)
	MaxDamage  int
	HealAmount int
	Duration   int // buff/debuff duration in seconds
	BuffStat   int // which stat to buff (1=STR, 2=VIT, etc.)
	BuffAmount int
	ReqLevel   int
	ReqMAG     int
	ReqINT     int
	GoldCost   int // gold to learn from Gandalf (-1 = not sold, manual only)
	SpriteID   int // client spell effect sprite
	SoundID    int // client sound effect

	// Original Magic.cfg behaviour. CfgType is the DEF_MAGICTYPE_* number the
	// original server switched on; the fields below are derived from it.
	CfgType    int
	AreaX      int  // hit box half-width around the clicked tile
	AreaY      int  // hit box half-height
	Line       bool // also hits a line from the caster to the clicked tile
	CentreDice Dice // extra damage for whoever is on the clicked tile
	AreaDice   Dice // damage for everyone inside the hit box
	LineDice   Dice // damage for everyone on the line

	// Heals (and stamina restores, CfgType 7): HealDice for whoever is on the
	// clicked tile, AreaHealDice for everyone in the AreaX/AreaY box around it.
	HealDice     Dice
	AreaHealDice Dice

	// Staminar Drain (CfgType 5): SPDice off the clicked player's stamina,
	// AreaSPDice off everyone in the AreaX/AreaY box (them included).
	SPDice     Dice
	AreaSPDice Dice

	// Status effect fields (for spells that apply effects)
	ApplyEffect    EffectType // which status effect to apply (0 = none)
	EffectLevel    int        // intensity of the effect (1-3)
	TickDamage     int        // for poison: damage per tick
	TickIntervalMs int        // for poison: ms between ticks

	// Lasting fields (CfgType 14, the original's DEF_MAGICTYPE_CREATE_DYNAMIC:
	// Fire Wall, Fire Field, Poison Cloud, Cloud Kill, Spike Field, Ice Storm).
	// The cast itself does no damage; it leaves field tiles that hurt whoever
	// is in or next to them for Duration seconds.
	Field FieldSpec
}

// Original DEF_MAGICTYPE_* numbers (SpellDef.CfgType) for the debuffs handled
// in game/debuff_spells.go.
const (
	CfgTypeSPDownArea = 5  // Staminar Drain
	CfgTypeHold       = 12 // Hold Person, Paralyze
	CfgTypePossession = 15 // Possession: pick up an item from afar
	CfgTypeConfuse    = 16 // Confuse Language, Confusion, Illusion, Illusion Movement (+ Mass)
)

// FieldKind is the original's DEF_DYNAMICOBJECT_* number (Magic.cfg sValue10).
type FieldKind int

const (
	FieldNone        FieldKind = 0
	FieldFire        FieldKind = 1  // 1d6 a second to the tile and its neighbours
	FieldIceStorm    FieldKind = 8  // 3d3+5 a second over 5x5, freezes players
	FieldSpike       FieldKind = 9  // 2d4 to a player stepping onto it
	FieldPoisonCloud FieldKind = 10 // 1d6 (1d8 if Power >= 20) a second to 3x3, poisons players
)

// FieldLayout is how the tiles are laid out (Magic.cfg sValue11).
type FieldLayout int

const (
	FieldSingle FieldLayout = 0 // one tile at the target (Ice Storm)
	FieldWall   FieldLayout = 1 // a line across the cast direction, Size tiles each side
	FieldSquare FieldLayout = 2 // a square, Size tiles each side of the target
)

// FieldSpec describes a lasting field. Kind 0 = not a field spell.
type FieldSpec struct {
	Kind   FieldKind
	Layout FieldLayout
	Size   int // Magic.cfg sValue12
	Power  int // Magic.cfg sValue5 (poison strength)
}

// IsField reports whether the spell leaves a lasting field.
func (s *SpellDef) IsField() bool { return s.Field.Kind != FieldNone }

// FieldTiles is where a field spell cast from (cx,cy) at (tx,ty) puts its
// tiles, as the original server laid them out. The wall runs across the
// direction from caster to target (matches the client's FireWall).
func (s *SpellDef) FieldTiles(cx, cy, tx, ty int) [][2]int {
	tiles := [][2]int{{tx, ty}}
	switch s.Field.Layout {
	case FieldWall:
		dx, dy := sign(tx-cx), sign(ty-cy)
		rx, ry := -dy, dx
		if rx == 0 && ry == 0 {
			rx = 1
		}
		for i := 1; i <= s.Field.Size; i++ {
			tiles = append(tiles, [2]int{tx + i*rx, ty + i*ry}, [2]int{tx - i*rx, ty - i*ry})
		}
	case FieldSquare:
		tiles = tiles[:0]
		for y := ty - s.Field.Size; y <= ty+s.Field.Size; y++ {
			for x := tx - s.Field.Size; x <= tx+s.Field.Size; x++ {
				tiles = append(tiles, [2]int{x, y})
			}
		}
	}
	return tiles
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// DealsDamage reports whether the spell damages whatever it lands on.
func (s *SpellDef) DealsDamage() bool {
	return !s.CentreDice.IsZero() || !s.AreaDice.IsZero() || !s.LineDice.IsZero()
}

// NeedsTarget reports whether the spell only makes sense with someone on the
// clicked tile (single-tile damage and debuffs). Area spells can be cast on
// empty ground.
func (s *SpellDef) NeedsTarget() bool {
	if s.Type == SpellTypeDebuff {
		// Area debuffs (Confusion, Staminar Drain...) can land on empty ground,
		// and Possession is cast on an item lying on an empty tile.
		return s.AreaX == 0 && s.AreaY == 0 && s.CfgType != CfgTypePossession
	}
	return s.DealsDamage() && !s.Line && s.AreaX == 0 && s.AreaY == 0
}

// SpellDB is the global spell definition registry, filled from spelldefs_gen.go.
var SpellDB = map[int]*SpellDef{}

// GetSpellDef looks up a spell definition by ID.
func GetSpellDef(id int) *SpellDef {
	return SpellDB[id]
}
