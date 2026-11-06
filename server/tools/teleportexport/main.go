// Command teleportexport prints the server's maps, built-in doors and built-in
// spawns as JSON for the map editor (client/map-editor.html; its dev-server
// plugin runs this).
//
//	go run ./server/tools/teleportexport -maps assets/MAPDATA
//
// For every map it lists the tiles that work as doors today, the way the server
// resolves them for maps not in teleports.json: tiles with the .amd teleport flag,
// using their own door entry or the nearest one within 2 tiles. Entries on tiles
// without the flag never fire, so they are listed separately. Also reports
// whether teleports.json, walkable.json and spawns.json (as on disk) parse.
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
	Borrowed bool `json:"borrowed,omitempty"`
}

type mapInfo struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Doors: tiles that work as doors on this map today (built-in doors).
	Doors []tile `json:"doors"`
	// Unflagged: built-in entries on tiles without the teleport flag (never fire).
	Unflagged []tile `json:"unflagged"`
	// Spawns: the built-in spawn list (DefaultSpawnConfig) for this map.
	Spawns []game.SpawnSpot `json:"spawns"`
}

type npcTypeInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Stationary bool   `json:"stationary,omitempty"`
	Town       bool   `json:"town,omitempty"`
}

type output struct {
	Maps          []mapInfo     `json:"maps"`
	OverrideError string        `json:"overrideError,omitempty"`
	WalkError     string        `json:"walkError,omitempty"`
	SpawnError    string        `json:"spawnError,omitempty"`
	NpcTypes      []npcTypeInfo `json:"npcTypes"`
}

func main() {
	mapsDir := flag.String("maps", "assets/MAPDATA", "the server's .amd folder")
	editsDir := flag.String("edits", "server/internal/mapdata", "folder with teleports.json and walkable.json")
	flag.Parse()

	entries, err := os.ReadDir(*mapsDir)
	if err != nil {
		log.Fatal(err)
	}
	builtin := mapdata.BuildTeleportConfig()

	// Built-in entries per map (lower-case name), to find the unflagged ones.
	byMap := map[string][][3]int{}
	for key := range builtin {
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			continue
		}
		x, _ := strconv.Atoi(parts[1])
		y, _ := strconv.Atoi(parts[2])
		byMap[strings.ToLower(parts[0])] = append(byMap[strings.ToLower(parts[0])], [3]int{x, y})
	}
	// Built-in keys keep the spelling they were typed with; find it per map.
	spelling := map[string]string{}
	for key := range builtin {
		m, _, _ := strings.Cut(key, ":")
		spelling[strings.ToLower(m)] = m
	}

	var out output
	if _, err := mapdata.LoadTeleportOverridesFrom(*editsDir); err != nil {
		out.OverrideError = err.Error()
	}
	if _, err := mapdata.LoadWalkOverridesFrom(*editsDir); err != nil {
		out.WalkError = err.Error()
	}
	if _, err := game.LoadSpawnOverridesFrom(*editsDir); err != nil {
		out.SpawnError = err.Error()
	}
	spawns := game.DefaultSpawnConfig()
	for _, t := range npc.NpcTypes {
		out.NpcTypes = append(out.NpcTypes, npcTypeInfo{ID: t.ID, Name: t.Name, Stationary: t.Stationary, Town: npc.IsShopNPC(t.ID)})
	}
	sort.Slice(out.NpcTypes, func(i, j int) bool { return out.NpcTypes[i].ID < out.NpcTypes[j].ID })
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".amd") {
			continue
		}
		gm, err := mapdata.LoadAMD(filepath.Join(*mapsDir, e.Name()))
		if err != nil {
			log.Printf("skipping %s: %v", e.Name(), err)
			continue
		}
		lower := strings.ToLower(gm.Name)
		keyName := gm.Name
		if s, ok := spelling[lower]; ok {
			keyName = s
		}
		info := mapInfo{Name: gm.Name, File: e.Name(), Width: gm.Width, Height: gm.Height, Doors: []tile{}, Unflagged: []tile{}, Spawns: []game.SpawnSpot{}}
		for _, entry := range spawns.Spawns {
			if strings.EqualFold(entry.MapName, gm.Name) {
				info.Spawns = append(info.Spawns, game.SpotOf(entry))
			}
		}
		for y := 0; y < gm.Height; y++ {
			for x := 0; x < gm.Width; x++ {
				if !gm.Tiles[y][x].Teleport {
					continue
				}
				d, ok := builtin.Nearest(keyName, x, y)
				if !ok {
					continue
				}
				_, own := builtin[mapdata.TeleportKey(keyName, x, y)]
				info.Doors = append(info.Doors, tile{X: x, Y: y, To: toDest(d), Borrowed: !own})
			}
		}
		for _, xy := range byMap[lower] {
			x, y := xy[0], xy[1]
			if x >= 0 && y >= 0 && x < gm.Width && y < gm.Height && gm.Tiles[y][x].Teleport {
				continue
			}
			info.Unflagged = append(info.Unflagged, tile{X: x, Y: y, To: toDest(builtin[mapdata.TeleportKey(keyName, x, y)])})
		}
		sort.Slice(info.Unflagged, func(i, j int) bool {
			a, b := info.Unflagged[i], info.Unflagged[j]
			return a.Y < b.Y || (a.Y == b.Y && a.X < b.X)
		})
		out.Maps = append(out.Maps, info)
	}
	sort.Slice(out.Maps, func(i, j int) bool { return strings.ToLower(out.Maps[i].Name) < strings.ToLower(out.Maps[j].Name) })

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		log.Fatal(err)
	}
}

func toDest(d mapdata.TeleportDest) dest {
	return dest{Map: d.DestMap, X: d.DestX, Y: d.DestY, Dir: d.Dir}
}
