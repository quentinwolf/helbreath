package mapdata

// TeleportDest defines a destination for a teleport tile.
type TeleportDest struct {
	DestMap  string
	DestX    int
	DestY    int
	Dir      int // direction player faces after teleport (0 = keep current)
}

// TeleportConfig maps "mapName:x:y" keys to destinations.
// Multiple source tiles can point to the same destination.
type TeleportConfig map[string]TeleportDest

// TeleportKey creates a lookup key for a source tile.
func TeleportKey(mapName string, x, y int) string {
	// Use a simple format: "map:x:y"
	return mapName + ":" + itoa(x) + ":" + itoa(y)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// NoAttackArea defines a rectangular no-attack zone on a map.
type NoAttackArea struct {
	X1, Y1, X2, Y2 int
}

// BuildNoAttackAreas is every map's no-attack-area rectangles from its config
// file, by lower-case map name. The original files mark "none" with a
// rectangle of -10s; those are left out. (Not enforced yet: docs/STATUS.md.)
func BuildNoAttackAreas() map[string][]NoAttackArea {
	areas := map[string][]NoAttackArea{}
	for lower, c := range Configs() {
		for _, r := range c.NoAttack {
			if r.X2 < 0 || r.Y2 < 0 {
				continue
			}
			areas[lower] = append(areas[lower], NoAttackArea{r.X1, r.Y1, r.X2, r.Y2})
		}
	}
	return areas
}

// BuildTeleportConfig is the doors (teleport-loc) of every map config in use.
func BuildTeleportConfig() TeleportConfig {
	return DoorsFrom(Configs())
}

// DoorsFrom is the doors of the maps in cs. Doors to a map that has no config
// (the original has some to maps the port doesn't have yet) are left out, and
// names are spelled as the maps' files.
func DoorsFrom(cs MapConfigs) TeleportConfig {
	tc := make(TeleportConfig)
	for _, c := range cs {
		for _, t := range c.Teleports {
			dest := cs.Get(t.DestMap)
			if dest == nil {
				continue
			}
			tc[TeleportKey(c.Name, t.X, t.Y)] = TeleportDest{DestMap: dest.Name, DestX: t.DestX, DestY: t.DestY, Dir: t.Dir}
		}
	}
	return tc
}
