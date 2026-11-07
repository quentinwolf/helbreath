// Command mapconfigsave writes the map editor's door and spawn edits into the
// map configs (server/internal/mapdata/maps/<map>.txt). The editor's dev-server
// plugin runs it with the edits as JSON on stdin:
//
//	go run ./server/tools/mapconfigsave [-edits server/internal/mapdata] < edits.json
//
//	{"doors":  {"<map>": [{"tiles": [[x, y], ...], "to": {"map": "...", "x": 0, "y": 0, "dir": 0}, "name": ""}]},
//	 "spawns": {"<map>": [{"npc": 10, "count": 5, "x": 0, "y": 0, "radius": 0, "area": [x1, y1, x2, y2],
//	                       "dir": 0, "respawn": 0, "note": "", "src": "spot:3"}]},
//	 "settings": {"<map>": {"maximum-object": "600", "random-mob-max": ""}}}
//
// settings: values as written in the file, "" removes the line
// (mapdata.EditableSettings lists the keys).
//
// Each listed map's doors / spawns become exactly those given (doors also set
// exact-doors = 1). Only the lines that change are rewritten. Every changed
// file is checked the way the server reads it first; if any has a problem,
// nothing is written. Prints {"written": [...], "errors": [...]}.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/juanrossi/hbonline/server/internal/game" // sets the map config name check
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

type door struct {
	Name  string   `json:"name"`
	Tiles [][2]int `json:"tiles"`
	To    struct {
		Map string `json:"map"`
		X   int    `json:"x"`
		Y   int    `json:"y"`
		Dir int    `json:"dir"`
	} `json:"to"`
}

type spawn struct {
	Npc     int     `json:"npc"`
	Count   int     `json:"count"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Radius  int     `json:"radius"`
	Area    *[4]int `json:"area"`
	Dir     int     `json:"dir"`
	Respawn int     `json:"respawn"`
	Note    string  `json:"note"`
	Src     string  `json:"src"`
}

type input struct {
	Doors    map[string][]door            `json:"doors"`
	Spawns   map[string][]spawn           `json:"spawns"`
	Settings map[string]map[string]string `json:"settings"`
}

type result struct {
	Written []string `json:"written"`
	Errors  []string `json:"errors"`
}

func main() {
	editsDir := flag.String("edits", "server/internal/mapdata", "folder with maps/*.txt")
	flag.Parse()
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		log.Fatal(err)
	}
	res := save(filepath.Join(*editsDir, mapdata.MapConfigDir), in)
	if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
		log.Fatal(err)
	}
}

func save(dir string, in input) result {
	res := result{Written: []string{}, Errors: []string{}}
	fail := func(format string, a ...any) { res.Errors = append(res.Errors, fmt.Sprintf(format, a...)) }

	// The configs as on disk: which maps exist, and each map's file
	files := map[string]string{} // lower name -> file name
	entries, err := os.ReadDir(dir)
	if err != nil {
		fail("%v", err)
		return res
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			files[strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))] = e.Name()
		}
	}
	exists := func(m string) bool { _, ok := files[strings.ToLower(m)]; return ok }
	spelled := func(m string) string { // a map as its config file spells it
		if f, ok := files[strings.ToLower(m)]; ok {
			return strings.TrimSuffix(f, filepath.Ext(f))
		}
		return m
	}

	edits := map[string]*mapdata.MapEdit{}
	editFor := func(m string) *mapdata.MapEdit {
		lower := strings.ToLower(m)
		if edits[lower] == nil {
			edits[lower] = &mapdata.MapEdit{MapExists: exists}
		}
		return edits[lower]
	}
	for m, doors := range in.Doors {
		var tiles []mapdata.DoorTile
		for _, d := range doors {
			if !exists(d.To.Map) {
				fail("%s: a door leads to %q, which has no map config", m, d.To.Map)
			}
			for _, t := range d.Tiles {
				tiles = append(tiles, mapdata.DoorTile{X: t[0], Y: t[1], DestMap: spelled(d.To.Map),
					DestX: d.To.X, DestY: d.To.Y, Dir: d.To.Dir, Note: strings.TrimSpace(d.Name)})
			}
		}
		editFor(m).Doors = &tiles
	}
	for m, spawns := range in.Spawns {
		var list []mapdata.SpawnEdit
		for i, s := range spawns {
			e, err := spawnEdit(s)
			if err != nil {
				fail("%s spawn %d: %v", m, i+1, err)
				continue
			}
			list = append(list, e)
		}
		editFor(m).Spawns = &list
	}
	for m, set := range in.Settings {
		for k := range set {
			if _, ok := mapdata.EditableSettings[k]; !ok {
				fail("%s: %s can't be set from the editor", m, k)
			}
		}
		editFor(m).Settings = set
	}
	if len(res.Errors) > 0 {
		return res
	}

	// Apply and check everything before writing anything
	out := map[string]string{}
	var names []string
	for lower, edit := range edits {
		file, ok := files[lower]
		if !ok {
			fail("%s: no map config (maps/%s.txt)", lower, lower)
			continue
		}
		name := strings.TrimSuffix(file, filepath.Ext(file))
		old, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			fail("%v", err)
			continue
		}
		text, err := mapdata.EditMapConfig(name, old, *edit)
		if err != nil {
			fail("%v", err)
			continue
		}
		if text == string(old) {
			continue
		}
		if c, errs := mapdata.ParseMapConfig(name, []byte(text)); len(errs) > 0 {
			fail("%v", errs[0])
		} else if errs := mapdata.CheckConfig(c); len(errs) > 0 {
			fail("%v", errs[0])
		}
		out[file] = text
		names = append(names, name)
	}
	if len(res.Errors) > 0 {
		return res
	}
	sort.Strings(names)
	for file, text := range out {
		if err := writeWhole(filepath.Join(dir, file), text); err != nil {
			fail("%v", err)
		}
	}
	res.Written = names
	return res
}

// spawnEdit is an editor spawn in config terms: an area (or a spread, or a
// group on one tile) becomes a spot-mob-generator, a single NPC an npc line.
func spawnEdit(s spawn) (mapdata.SpawnEdit, error) {
	name := npc.ConfigName(s.Npc)
	if name == "" {
		return mapdata.SpawnEdit{}, fmt.Errorf("NPC type %d can't be named in a map config", s.Npc)
	}
	e := mapdata.SpawnEdit{Src: s.Src, Name: name, X: s.X, Y: s.Y, Note: strings.TrimSpace(s.Note),
		SpawnOptions: mapdata.SpawnOptions{Dir: s.Dir, Respawn: s.Respawn}}
	count := max(1, s.Count)
	if a := s.Area; a != nil && a[0] == a[2] && a[1] == a[3] && count == 1 {
		// one NPC on one tile: an npc line
		e.X, e.Y = a[0], a[1]
		s.Area = nil
	}
	switch {
	case s.Area != nil:
		e.Spot, e.Area = true, mapdata.Rect{X1: s.Area[0], Y1: s.Area[1], X2: s.Area[2], Y2: s.Area[3]}
	case s.Radius > 0:
		e.Spot = true
		e.Area = mapdata.Rect{X1: max(0, s.X-s.Radius), Y1: max(0, s.Y-s.Radius), X2: s.X + s.Radius, Y2: s.Y + s.Radius}
	case count > 1:
		e.Spot, e.Area = true, mapdata.Rect{X1: s.X, Y1: s.Y, X2: s.X, Y2: s.Y}
	}
	if e.Spot {
		e.Max = count
	}
	return e, nil
}

// writeWhole writes a temp file and renames it over the target, so the running
// server never reads half a file.
func writeWhole(file, text string) error {
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		os.Remove(tmp)
		return os.WriteFile(file, []byte(text), 0o644)
	}
	return nil
}
