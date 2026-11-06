package game

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// spawns.json (server/internal/mapdata, written by the map editor, see
// docs/map-editor.md) replaces the spawn list of every map it lists: monster
// spawns and town NPCs alike. Other maps keep DefaultSpawnConfig.

// SpawnOverrides is the contents of spawns.json.
type SpawnOverrides struct {
	Version int                    `json:"version"`
	Maps    map[string][]SpawnSpot `json:"maps"`
}

// SpawnSpot is one spawn: Count NPCs of one type, either at X,Y (spread over
// ±Radius) or anywhere in Area (x1, y1, x2, y2), where they also wander.
type SpawnSpot struct {
	Npc     int     `json:"npc"`
	Count   int     `json:"count,omitempty"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Radius  int     `json:"radius,omitempty"`
	Area    *[4]int `json:"area,omitempty"`
	Dir     int     `json:"dir,omitempty"`     // facing on spawn, 1-8 (0 = default)
	Respawn int     `json:"respawn,omitempty"` // seconds (0 = the NPC type's)
	Note    string  `json:"note,omitempty"`
}

// ParseSpawnOverrides reads and checks a spawns.json file.
func ParseSpawnOverrides(data []byte) (SpawnOverrides, error) {
	var o SpawnOverrides
	if err := json.Unmarshal(data, &o); err != nil {
		return o, fmt.Errorf("spawns.json: %w", err)
	}
	if o.Version != 1 {
		return o, fmt.Errorf("spawns.json: unknown version %d", o.Version)
	}
	seen := map[string]string{}
	for m, spots := range o.Maps {
		lower := strings.ToLower(m)
		if other, dup := seen[lower]; dup {
			return o, fmt.Errorf("spawns.json: map %q is listed twice (%q)", m, other)
		}
		seen[lower] = m
		for i, s := range spots {
			where := fmt.Sprintf("spawns.json: %s spawn %d", m, i+1)
			if _, ok := npc.NpcTypes[s.Npc]; !ok {
				return o, fmt.Errorf("%s: unknown NPC type %d", where, s.Npc)
			}
			if s.Count < 0 || s.Count > 500 {
				return o, fmt.Errorf("%s: count %d (must be 0-500)", where, s.Count)
			}
			if s.Dir < 0 || s.Dir > 8 {
				return o, fmt.Errorf("%s: direction %d (must be 0-8)", where, s.Dir)
			}
			if s.Radius < 0 || s.Respawn < 0 {
				return o, fmt.Errorf("%s: radius and respawn can't be negative", where)
			}
			if a := s.Area; a != nil && (a[2] < a[0] || a[3] < a[1]) {
				return o, fmt.Errorf("%s: area %v must be x1 y1 x2 y2 with x1<=x2, y1<=y2", where, *a)
			}
		}
	}
	return o, nil
}

// LoadSpawnOverridesFrom parses dir/spawns.json, or the built-in copy.
func LoadSpawnOverridesFrom(dir string) (SpawnOverrides, error) {
	data, err := mapdata.ReadSpawnsFile(dir)
	if err != nil {
		return SpawnOverrides{}, err
	}
	return ParseSpawnOverrides(data)
}

// entry is the spot as a SpawnEntry on map m.
func (s SpawnSpot) entry(m string) SpawnEntry {
	e := SpawnEntry{
		MapName: m, NpcTypeID: s.Npc, Count: s.Count,
		SpawnX: s.X, SpawnY: s.Y, SpawnRadius: s.Radius,
		RespawnDelay: s.Respawn, Dir: s.Dir,
	}
	if a := s.Area; a != nil {
		e.AreaX1, e.AreaY1, e.AreaX2, e.AreaY2 = a[0], a[1], a[2], a[3]
	}
	return e
}

// Apply returns base with the spawns of every listed map replaced.
func (o SpawnOverrides) Apply(base *SpawnConfig) *SpawnConfig {
	listed := map[string]bool{}
	for m := range o.Maps {
		listed[strings.ToLower(m)] = true
	}
	out := &SpawnConfig{}
	for _, e := range base.Spawns {
		if !listed[strings.ToLower(e.MapName)] {
			out.Spawns = append(out.Spawns, e)
		}
	}
	for m, spots := range o.Maps {
		for _, s := range spots {
			out.Spawns = append(out.Spawns, s.entry(m))
		}
	}
	return out
}

// spawnsOn is cfg's entries for one map (name ignoring case).
func spawnsOn(cfg *SpawnConfig, mapName string) []SpawnEntry {
	var out []SpawnEntry
	for _, e := range cfg.Spawns {
		if strings.EqualFold(e.MapName, mapName) {
			out = append(out, e)
		}
	}
	return out
}

// SpotOf is a spawn entry in spawns.json form (for the map editor's export).
func SpotOf(e SpawnEntry) SpawnSpot {
	s := SpawnSpot{Npc: e.NpcTypeID, Count: e.Count, X: e.SpawnX, Y: e.SpawnY, Radius: e.SpawnRadius, Dir: e.Dir, Respawn: e.RespawnDelay}
	if e.HasArea() {
		s.Area = &[4]int{e.AreaX1, e.AreaY1, e.AreaX2, e.AreaY2}
	}
	return s
}
