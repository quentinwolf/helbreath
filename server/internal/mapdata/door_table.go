package mapdata

import "strings"

// TeleportTable is the server's doors, built from the map configs.
type TeleportTable struct {
	Dests TeleportConfig
	// Exact holds (in lower case) the maps with exact-doors = 1 (saved by the
	// map editor): on those a door works on exactly the tiles listed.
	Exact map[string]bool
}

// BuildTeleportTable is the doors of the map configs in use.
func BuildTeleportTable() TeleportTable {
	return TeleportTableFrom(Configs())
}

// TeleportTableFrom is the doors of the maps in cs.
func TeleportTableFrom(cs MapConfigs) TeleportTable {
	t := TeleportTable{Dests: DoorsFrom(cs), Exact: map[string]bool{}}
	for lower, c := range cs {
		if c.ExactDoors {
			t.Exact[lower] = true
		}
	}
	return t
}

// Nearest is the door for a tile: its own entry, or else the nearest entry
// within 2 tiles. Our maps mark some doorways a tile or two wider than the
// original's teleport-loc list, so marked tiles without an entry borrow one.
func (tc TeleportConfig) Nearest(mapName string, x, y int) (TeleportDest, bool) {
	if dest, ok := tc[TeleportKey(mapName, x, y)]; ok {
		return dest, true
	}
	for r := 1; r <= 2; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				if dest, ok := tc[TeleportKey(mapName, x+dx, y+dy)]; ok {
					return dest, true
				}
			}
		}
	}
	return TeleportDest{}, false
}

// DoorAt is the door a player stepping onto (x, y) uses, if any. Maps with
// exact-doors use exactly the tiles listed; other maps need the .amd teleport
// flag and fall back to the nearest door (Nearest).
func (t TeleportTable) DoorAt(gm *GameMap, mapName string, x, y int) (TeleportDest, bool) {
	if t.Exact[strings.ToLower(mapName)] {
		dest, ok := t.Dests[TeleportKey(mapName, x, y)]
		return dest, ok
	}
	if gm == nil || !gm.IsTeleport(x, y) {
		return TeleportDest{}, false
	}
	return t.Dests.Nearest(mapName, x, y)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
