// Command teleportexport prints the server's maps, doors, spawns and map
// settings as JSON for the map editor (client/map-editor.html; its dev-server
// plugin runs this).
//
//	go run ./server/tools/teleportexport -maps assets/MAPDATA [-edits server/internal/mapdata]
//
// Everything comes from the map configs on disk (<edits>/maps/*.txt), read the
// way the server reads them. For every map it lists the tiles that work as
// doors today: on maps with exact-doors the tiles listed; on the others the
// tiles with the .amd teleport flag, using their own door entry or the nearest
// one within 2 tiles. Entries on unflagged tiles of those maps never fire, so
// they are listed separately.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/juanrossi/hbonline/server/internal/game"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

type dest struct {
	Map string `json:"map"`
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Dir int    `json:"dir,omitempty"`
}

type tile struct {
	X  int  `json:"x"`
	Y  int  `json:"y"`
	To dest `json:"to"`
	// Borrowed: the tile has no entry of its own and uses one within 2 tiles.
	Borrowed bool   `json:"borrowed,omitempty"`
	Note     string `json:"note,omitempty"` // the teleport-loc line's comment
}

// spot is one spawn, as the editor edits it.
type spot struct {
	Npc     int     `json:"npc"`
	Count   int     `json:"count,omitempty"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Area    *[4]int `json:"area,omitempty"`
	Dir     int     `json:"dir,omitempty"`
	Respawn int     `json:"respawn,omitempty"`
	Note    string  `json:"note,omitempty"`
	Src     string  `json:"src,omitempty"` // "spot:<num>" or "npc:<line>"
}

// settings are the map config values the editor shows.
type settings struct {
	Location        string   `json:"location,omitempty"`
	FixedDay        bool     `json:"fixedDay,omitempty"`
	MaxObjects      int      `json:"maxObjects,omitempty"`
	RandomMobs      int      `json:"randomMobs,omitempty"` // level, 0 = off
	LevelLimit      int      `json:"levelLimit,omitempty"`
	UpperLevelLimit int      `json:"upperLevelLimit,omitempty"`
	InitialPoints   [][3]int `json:"initialPoints"` // num, x, y
	Waypoints       [][3]int `json:"waypoints"`
	FishPoints      [][3]int `json:"fishPoints"`
	MineralPoints   [][3]int `json:"mineralPoints"`
	NoAttackAll     bool     `json:"noAttackAll,omitempty"` // the whole map (a -10 rectangle)
	NoAttack        [][5]int `json:"noAttack"`     // num, x1, y1, x2, y2
	NpcAvoid        [][5]int `json:"npcAvoid"`     // where random monsters don't spawn
	StrikePoints    [][3]int `json:"strikePoints"` // num, x, y
}

type mapInfo struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Config string `json:"config,omitempty"` // the config file, "" if the map has none
	// Exact: exact-doors = 1, doors work on exactly the tiles listed.
	Exact bool `json:"exact,omitempty"`
	// Doors: tiles that work as doors on this map today.
	Doors []tile `json:"doors"`
	// Unflagged: entries on tiles without the teleport flag (never fire; not on exact maps).
	Unflagged []tile   `json:"unflagged"`
	Spawns    []spot   `json:"spawns"`
	Settings  settings `json:"settings"`
}

type npcTypeInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ConfigName string `json:"configName,omitempty"` // as map configs write it ("" = can't be used there)
	Stationary bool   `json:"stationary,omitempty"`
	Town       bool   `json:"town,omitempty"`
}

type output struct {
	Maps         []mapInfo     `json:"maps"`
	WalkError    string        `json:"walkError,omitempty"`
	ConfigErrors []string      `json:"configErrors,omitempty"`
	NpcTypes     []npcTypeInfo `json:"npcTypes"`
}

func main() {
	mapsDir := flag.String("maps", "assets/MAPDATA", "the server's .amd folder")
	editsDir := flag.String("edits", "server/internal/mapdata", "folder with maps/*.txt and walkable.json")
	flag.Parse()

	var out output
	configs, errs := mapdata.LoadMapConfigs(*editsDir, nil)
	for _, err := range errs {
		out.ConfigErrors = append(out.ConfigErrors, err.Error())
	}
	mapdata.SetConfigs(configs)
	table := mapdata.TeleportTableFrom(configs)
	if _, err := mapdata.LoadWalkOverridesFrom(*editsDir); err != nil {
		out.WalkError = err.Error()
	}
	for _, t := range npc.NpcTypes {
		out.NpcTypes = append(out.NpcTypes, npcTypeInfo{ID: t.ID, Name: t.Name, ConfigName: npc.ConfigName(t.ID),
			Stationary: t.Stationary, Town: npc.IsShopNPC(t.ID)})
	}
	sort.Slice(out.NpcTypes, func(i, j int) bool { return out.NpcTypes[i].ID < out.NpcTypes[j].ID })
	spawns := game.DefaultSpawnConfig()

	entries, err := os.ReadDir(*mapsDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".amd") {
			continue
		}
		gm, err := mapdata.LoadAMD(filepath.Join(*mapsDir, e.Name()))
		if err != nil {
			log.Printf("skipping %s: %v", e.Name(), err)
			continue
		}
		info := mapInfo{Name: gm.Name, File: e.Name(), Width: gm.Width, Height: gm.Height,
			Doors: []tile{}, Unflagged: []tile{}, Spawns: []spot{}}
		c := configs.Get(gm.Name)
		notes := map[[2]int]string{}
		if c != nil {
			info.Config = c.Name + ".txt"
			info.Exact = c.ExactDoors
			info.Settings = settingsOf(c)
			for _, t := range c.Teleports {
				if configs.Get(t.DestMap) != nil {
					notes[[2]int{t.X, t.Y}] = t.Comment
				}
			}
		}
		for _, s := range spawns.Spawns {
			if strings.EqualFold(s.MapName, gm.Name) {
				info.Spawns = append(info.Spawns, spotOf(s))
			}
		}
		if info.Exact {
			for key, d := range table.Dests {
				m, x, y := splitKey(key)
				if m == strings.ToLower(gm.Name) {
					info.Doors = append(info.Doors, tile{X: x, Y: y, To: toDest(d), Note: notes[[2]int{x, y}]})
				}
			}
		} else {
			for y := 0; y < gm.Height; y++ {
				for x := 0; x < gm.Width; x++ {
					if !gm.Tiles[y][x].Teleport {
						continue
					}
					d, ok := table.Dests.Nearest(gm.Name, x, y)
					if !ok {
						continue
					}
					_, own := table.Dests[mapdata.TeleportKey(gm.Name, x, y)]
					info.Doors = append(info.Doors, tile{X: x, Y: y, To: toDest(d), Borrowed: !own, Note: notes[[2]int{x, y}]})
				}
			}
			for key, d := range table.Dests {
				m, x, y := splitKey(key)
				if m != strings.ToLower(gm.Name) {
					continue
				}
				if x >= 0 && y >= 0 && x < gm.Width && y < gm.Height && gm.Tiles[y][x].Teleport {
					continue
				}
				info.Unflagged = append(info.Unflagged, tile{X: x, Y: y, To: toDest(d), Note: notes[[2]int{x, y}]})
			}
		}
		sortTiles(info.Doors)
		sortTiles(info.Unflagged)
		out.Maps = append(out.Maps, info)
	}
	sort.Slice(out.Maps, func(i, j int) bool { return strings.ToLower(out.Maps[i].Name) < strings.ToLower(out.Maps[j].Name) })

	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		log.Fatal(err)
	}
}

func splitKey(key string) (string, int, int) {
	parts := strings.Split(key, ":")
	if len(parts) != 3 {
		return "", 0, 0
	}
	x, _ := strconv.Atoi(parts[1])
	y, _ := strconv.Atoi(parts[2])
	return parts[0], x, y
}

func sortTiles(ts []tile) {
	sort.Slice(ts, func(i, j int) bool { return ts[i].Y < ts[j].Y || (ts[i].Y == ts[j].Y && ts[i].X < ts[j].X) })
}

func toDest(d mapdata.TeleportDest) dest {
	return dest{Map: d.DestMap, X: d.DestX, Y: d.DestY, Dir: d.Dir}
}

func spotOf(e game.SpawnEntry) spot {
	s := spot{Npc: e.NpcTypeID, Count: e.Count, X: e.SpawnX, Y: e.SpawnY, Dir: e.Dir, Respawn: e.RespawnDelay, Note: e.Note, Src: e.Src}
	if e.HasArea() {
		s.Area = &[4]int{e.AreaX1, e.AreaY1, e.AreaX2, e.AreaY2}
		s.X, s.Y = (e.AreaX1+e.AreaX2)/2, (e.AreaY1+e.AreaY2)/2
	}
	return s
}

func settingsOf(c *mapdata.MapConfig) settings {
	s := settings{Location: c.Location, FixedDay: c.FixedDay, MaxObjects: c.MaxObjects,
		LevelLimit: c.LevelLimit, UpperLevelLimit: c.UpperLevelLimit,
		InitialPoints: [][3]int{}, Waypoints: [][3]int{}, FishPoints: [][3]int{}, MineralPoints: [][3]int{},
		NoAttack: [][5]int{}, NpcAvoid: [][5]int{}, StrikePoints: [][3]int{}}
	if c.RandomMobs.On {
		s.RandomMobs = max(1, c.RandomMobs.Level)
	}
	for _, p := range c.InitialPoints {
		s.InitialPoints = append(s.InitialPoints, [3]int{p.Num, p.X, p.Y})
	}
	for n, p := range c.Waypoints {
		s.Waypoints = append(s.Waypoints, [3]int{n, p.X, p.Y})
	}
	sort.Slice(s.Waypoints, func(i, j int) bool { return s.Waypoints[i][0] < s.Waypoints[j][0] })
	for _, p := range c.FishPoints {
		s.FishPoints = append(s.FishPoints, [3]int{p.Num, p.X, p.Y})
	}
	for _, p := range c.MineralPts {
		s.MineralPoints = append(s.MineralPoints, [3]int{p.Num, p.X, p.Y})
	}
	s.NoAttackAll = c.WholeMapNoAttack()
	for _, r := range c.NpcAvoid {
		s.NpcAvoid = append(s.NpcAvoid, [5]int{r.Num, r.X1, r.Y1, r.X2, r.Y2})
	}
	for _, r := range c.NoAttack {
		if r.Y1 > 0 { // the original's rule (CMap::_SetupNoAttackArea)
			s.NoAttack = append(s.NoAttack, [5]int{r.Num, r.X1, r.Y1, r.X2, r.Y2})
		}
	}
	// Settings the parser keeps only as lines (not ported yet)
	for _, l := range c.Lines {
		n := numbersOf(l.Values)
		switch {
		case l.Key == "strike-point" && len(n) >= 3:
			s.StrikePoints = append(s.StrikePoints, [3]int{n[0], n[1], n[2]})
		}
	}
	return s
}

// numbersOf is the leading numbers of a line's values.
func numbersOf(v []string) []int {
	var out []int
	for _, s := range v {
		n, err := strconv.Atoi(s)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
