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

	// Status effect fields (for spells that apply effects)
	ApplyEffect    EffectType // which status effect to apply (0 = none)
	EffectLevel    int        // intensity of the effect (1-3)
	TickDamage     int        // for poison: damage per tick
	TickIntervalMs int        // for poison: ms between ticks
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
		return true
	}
	return s.DealsDamage() && !s.Line && s.AreaX == 0 && s.AreaY == 0
}

// SpellDB is the global spell definition registry, filled from spelldefs_gen.go.
var SpellDB = map[int]*SpellDef{}

// GetSpellDef looks up a spell definition by ID.
func GetSpellDef(id int) *SpellDef {
	return SpellDB[id]
}
