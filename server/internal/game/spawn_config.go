package game

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// SpawnEntry defines a single NPC spawn point in the configuration.
type SpawnEntry struct {
	MapName      string `json:"map_name"`
	NpcTypeID    int    `json:"npc_type_id"`
	Count        int    `json:"count"`         // number of NPCs to spawn (0 or 1 = single spawn)
	SpawnX       int    `json:"spawn_x"`
	SpawnY       int    `json:"spawn_y"`
	SpawnRadius  int    `json:"spawn_radius"`  // random offset radius from SpawnX/SpawnY (0 = exact position)
	RespawnDelay int    `json:"respawn_delay"` // override respawn delay in seconds (0 = use NpcType default)
	Dir          int    `json:"dir,omitempty"` // facing on (re)spawn, 1-8 (0 = the NPC type's default)

	// An original spot-mob-generator rectangle (all four 0 = none): NPCs spawn,
	// respawn and wander anywhere inside it, like DEF_MOVETYPE_RANDOMAREA.
	AreaX1 int `json:"area_x1,omitempty"`
	AreaY1 int `json:"area_y1,omitempty"`
	AreaX2 int `json:"area_x2,omitempty"`
	AreaY2 int `json:"area_y2,omitempty"`

	// Src is the map config line it came from ("spot:<num>" or "npc:<line>"),
	// Note that line's comment: for the map editor, not part of what spawns.
	Src  string `json:"-"`
	Note string `json:"-"`
}

// HasArea reports whether the entry spawns into a rectangle.
func (entry *SpawnEntry) HasArea() bool {
	return entry.AreaX2 > entry.AreaX1 || entry.AreaY2 > entry.AreaY1
}

// SpawnConfig holds the full spawn configuration for the server.
type SpawnConfig struct {
	Spawns []SpawnEntry `json:"spawns"`
}

// LoadSpawnConfig attempts to load spawn configuration from a JSON file.
// Returns nil if the file does not exist (caller should use defaults).
func LoadSpawnConfig(path string) (*SpawnConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no config file, use defaults
		}
		return nil, err
	}

	var cfg SpawnConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ToSpawnPoints converts a SpawnConfig into a flat list of npc.SpawnPoint entries,
// expanding Count and SpawnRadius into individual spawn points.
func (cfg *SpawnConfig) ToSpawnPoints() []npc.SpawnPoint {
	var points []npc.SpawnPoint
	for _, entry := range cfg.Spawns {
		count := entry.Count
		if count <= 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			points = append(points, npc.SpawnPoint{
				NpcTypeID: entry.NpcTypeID,
				MapName:   entry.MapName,
				X:         entry.SpawnX,
				Y:         entry.SpawnY,
			})
		}
	}
	return points
}

// RespawnDelayFor returns the respawn delay override for a given spawn entry,
// or zero (meaning use the NpcType default).
func (entry *SpawnEntry) RespawnDelayDuration() time.Duration {
	if entry.RespawnDelay > 0 {
		return time.Duration(entry.RespawnDelay) * time.Second
	}
	return 0
}

// DefaultSpawnConfig is every map's spawns from its config file
// (mapdata/maps/<map>.txt): the spot-mob-generator rectangles, each map held to
// its maximum-object cap, and the town NPCs of its npc lines.
func DefaultSpawnConfig() *SpawnConfig {
	return &SpawnConfig{Spawns: spawnsFromConfigs(mapdata.Configs())}
}

// spawnsFromConfigs is the spawns of every map in cs.
func spawnsFromConfigs(cs mapdata.MapConfigs) []SpawnEntry {
	var out []SpawnEntry
	for _, name := range cs.Names() {
		out = append(out, mapSpawns(cs.Get(name))...)
	}
	return out
}

// mapSpawns is one map's spawns. A name that isn't an NPC type is skipped
// here; checkConfigNames refuses such a file when it is loaded.
func mapSpawns(c *mapdata.MapConfig) []SpawnEntry {
	var out []SpawnEntry
	total := 0
	for _, s := range c.SpotMobs {
		if s.Type == 1 && s.Max > 0 {
			total += s.Max
		}
	}
	for _, s := range c.SpotMobs {
		if s.Type != 1 || s.Max <= 0 {
			continue // type 2 (walk between waypoints) isn't ported
		}
		id, err := npc.TypeForConfigName(s.Monster)
		if err != nil {
			continue
		}
		count := s.Max
		// The original filled the rectangles a monster at a time, at random,
		// until the map held maximum-object: when they add up to more than
		// that (the starter map: 122 against 50), each one gets its share.
		if c.MaxObjects > 0 && total > c.MaxObjects {
			count = max(1, s.Max*c.MaxObjects/total)
		}
		out = append(out, SpawnEntry{MapName: c.Name, NpcTypeID: id, Count: count,
			AreaX1: s.Area.X1, AreaY1: s.Area.Y1, AreaX2: s.Area.X2, AreaY2: s.Area.Y2,
			Dir: s.Dir, RespawnDelay: s.Respawn, Src: fmt.Sprintf("spot:%d", s.Num), Note: s.Comment})
	}
	for _, n := range c.NPCs {
		id, err := npc.TypeForConfigName(n.Name)
		if err != nil {
			continue
		}
		at, err := c.NpcPosition(n)
		if err != nil {
			continue
		}
		out = append(out, SpawnEntry{MapName: c.Name, NpcTypeID: id, SpawnX: at.X, SpawnY: at.Y,
			Dir: n.Dir, RespawnDelay: n.Respawn, Src: fmt.Sprintf("npc:%d", n.Line), Note: n.Comment})
	}
	return out
}

func init() {
	mapdata.SetConfigChecker(checkConfigNames)
}

// checkConfigNames finds monster and NPC names in a map config that aren't NPC types.
func checkConfigNames(c *mapdata.MapConfig) []error {
	var errs []error
	for _, s := range c.SpotMobs {
		if _, err := npc.TypeForConfigName(s.Monster); err != nil {
			errs = append(errs, fmt.Errorf("%s.txt line %d: spot-mob-generator %d: %w", c.Name, s.Line, s.Num, err))
		}
	}
	for _, n := range c.NPCs {
		if _, err := npc.TypeForConfigName(n.Name); err != nil {
			errs = append(errs, fmt.Errorf("%s.txt line %d: npc: %w", c.Name, n.Line, err))
		}
	}
	return errs
}

// MapSpawnsOf is one map config's spawns as the server makes them (for the
// map editor's export).
func MapSpawnsOf(c *mapdata.MapConfig) []SpawnEntry { return mapSpawns(c) }
