package mapdata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// teleports.json is written by the map editor (client/map-editor.html, see
// docs/map-editor.md). Every map listed in it replaces that map's built-in doors
// (the teleport-loc lines of its maps/<map>.txt). On those maps a door works on exactly the
// tiles listed: the .amd teleport flag and the nearby-tile fallback are ignored.
//
//go:embed teleports.json
var teleportOverridesJSON []byte

// TeleportOverrides is the contents of teleports.json.
type TeleportOverrides struct {
	Version int                       `json:"version"`
	Maps    map[string][]OverrideDoor `json:"maps"`
}

// OverrideDoor is one door: the tiles you step on and where they take you.
type OverrideDoor struct {
	Name  string   `json:"name,omitempty"`
	Tiles [][2]int `json:"tiles"`
	To    DoorDest `json:"to"`
}

// DoorDest is where a door takes you. Dir is the way you face on arrival
// (1 = north, clockwise to 8 = north-west; 0 keeps your current facing).
type DoorDest struct {
	Map string `json:"map"`
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Dir int    `json:"dir,omitempty"`
}

// ParseTeleportOverrides reads and checks a teleports.json file.
func ParseTeleportOverrides(data []byte) (TeleportOverrides, error) {
	var o TeleportOverrides
	if err := json.Unmarshal(data, &o); err != nil {
		return o, fmt.Errorf("teleports.json: %w", err)
	}
	if o.Version != 1 {
		return o, fmt.Errorf("teleports.json: unknown version %d", o.Version)
	}
	seen := map[string]string{}
	for m, doors := range o.Maps {
		lower := strings.ToLower(m)
		if other, dup := seen[lower]; dup {
			return o, fmt.Errorf("teleports.json: map %q is listed twice (%q)", m, other)
		}
		seen[lower] = m
		tiles := map[[2]int]bool{}
		for i, d := range doors {
			where := fmt.Sprintf("teleports.json: %s door %d", m, i+1)
			if d.To.Map == "" {
				return o, fmt.Errorf("%s has no destination map", where)
			}
			if d.To.Dir < 0 || d.To.Dir > 8 {
				return o, fmt.Errorf("%s has direction %d (must be 0-8)", where, d.To.Dir)
			}
			if len(d.Tiles) == 0 {
				return o, fmt.Errorf("%s has no tiles", where)
			}
			for _, t := range d.Tiles {
				if tiles[t] {
					return o, fmt.Errorf("%s: tile (%d,%d) belongs to two doors", where, t[0], t[1])
				}
				tiles[t] = true
			}
		}
	}
	return o, nil
}

// LoadTeleportOverrides parses the teleports.json built into the server.
func LoadTeleportOverrides() (TeleportOverrides, error) {
	return ParseTeleportOverrides(teleportOverridesJSON)
}

// Apply replaces the doors of every listed map in tc, and returns those maps'
// names in lower case (the maps whose doors ignore the .amd flag). Their door
// keys use the lower-case name too, so DoorAt finds them however a map is spelled.
func (o TeleportOverrides) Apply(tc TeleportConfig) map[string]bool {
	exact := map[string]bool{}
	for m := range o.Maps {
		exact[strings.ToLower(m)] = true
	}
	for key := range tc {
		if m, _, ok := strings.Cut(key, ":"); ok && exact[strings.ToLower(m)] {
			delete(tc, key)
		}
	}
	for m, doors := range o.Maps {
		for _, d := range doors {
			dest := TeleportDest{DestMap: d.To.Map, DestX: d.To.X, DestY: d.To.Y, Dir: d.To.Dir}
			for _, t := range d.Tiles {
				tc[TeleportKey(strings.ToLower(m), t[0], t[1])] = dest
			}
		}
	}
	return exact
}

// TeleportTable is the server's doors: the built-in ones with teleports.json applied.
type TeleportTable struct {
	Dests TeleportConfig
	// Exact holds (in lower case) the maps from teleports.json.
	Exact map[string]bool
}

// BuildTeleportTable is BuildTeleportConfig plus the built-in teleports.json.
func BuildTeleportTable() (TeleportTable, error) {
	return BuildTeleportTableFrom("")
}

// BuildTeleportTableFrom is BuildTeleportConfig plus teleports.json from dir (or
// the built-in copy, see LoadTeleportOverridesFrom). A broken teleports.json
// returns the built-in doors with the error.
func BuildTeleportTableFrom(dir string) (TeleportTable, error) {
	tc := BuildTeleportConfig()
	o, err := LoadTeleportOverridesFrom(dir)
	if err != nil {
		return TeleportTable{Dests: tc, Exact: map[string]bool{}}, err
	}
	return TeleportTable{Dests: tc, Exact: o.Apply(tc)}, nil
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

// DoorAt is the door a player stepping onto (x, y) uses, if any. Maps from
// teleports.json use exact tiles only; other maps need the .amd teleport flag
// and fall back to the nearest door (Nearest).
func (t TeleportTable) DoorAt(gm *GameMap, mapName string, x, y int) (TeleportDest, bool) {
	if lower := strings.ToLower(mapName); t.Exact[lower] {
		dest, ok := t.Dests[TeleportKey(lower, x, y)]
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
