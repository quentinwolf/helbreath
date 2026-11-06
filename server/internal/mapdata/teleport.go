package mapdata

import "strings"

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

// TeleportKey creates a lookup key for a source tile: "map:x:y", the map in
// lower case (the original spells some maps two ways, e.g. Cmdhall_1 / CmdHall_1).
func TeleportKey(mapName string, x, y int) string {
	return strings.ToLower(mapName) + ":" + itoa(x) + ":" + itoa(y)
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
