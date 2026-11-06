package mapdata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// walkable.json is written by the map editor (docs/map-editor.md): per map, the
// tiles whose walkability differs from the .amd file. It is applied to every map
// as it loads, so movement, NPCs, spells and the collision grid sent to the
// client all see it. The .amd files themselves are never changed.
//
//go:embed walkable.json
var walkOverridesJSON []byte

// WalkOverrides is the contents of walkable.json.
type WalkOverrides struct {
	Version int                 `json:"version"`
	Maps    map[string]WalkEdit `json:"maps"`
}

// WalkEdit lists one map's changed tiles as [x, y] pairs.
type WalkEdit struct {
	Block [][2]int `json:"block,omitempty"` // walkable in the .amd, blocked here
	Open  [][2]int `json:"open,omitempty"`  // blocked in the .amd, walkable here
}

// ParseWalkOverrides reads and checks a walkable.json file.
func ParseWalkOverrides(data []byte) (WalkOverrides, error) {
	var o WalkOverrides
	if err := json.Unmarshal(data, &o); err != nil {
		return o, fmt.Errorf("walkable.json: %w", err)
	}
	if o.Version != 1 {
		return o, fmt.Errorf("walkable.json: unknown version %d", o.Version)
	}
	seen := map[string]string{}
	for m, e := range o.Maps {
		lower := strings.ToLower(m)
		if other, dup := seen[lower]; dup {
			return o, fmt.Errorf("walkable.json: map %q is listed twice (%q)", m, other)
		}
		seen[lower] = m
		tiles := map[[2]int]bool{}
		for _, t := range append(append([][2]int{}, e.Block...), e.Open...) {
			if tiles[t] {
				return o, fmt.Errorf("walkable.json: %s tile (%d,%d) is listed twice", m, t[0], t[1])
			}
			tiles[t] = true
		}
	}
	return o, nil
}

// LoadWalkOverrides parses the walkable.json built into the server.
func LoadWalkOverrides() (WalkOverrides, error) {
	return ParseWalkOverrides(walkOverridesJSON)
}

// Reapply makes gm's walkability the .amd's (BaseWalkable) plus the edits listed
// for its map (name ignoring case), and returns how many tiles changed. Only
// changed tiles are written, so it is safe to run on a live map. Tiles outside
// the map are skipped; a map not listed goes back to its .amd walkability.
func (o WalkOverrides) Reapply(gm *GameMap) int {
	want := make(map[[2]int]bool)
	for m, e := range o.Maps {
		if !strings.EqualFold(m, gm.Name) {
			continue
		}
		for _, t := range e.Block {
			want[t] = false
		}
		for _, t := range e.Open {
			want[t] = true
		}
	}
	changed := 0
	for y := 0; y < gm.Height; y++ {
		row := gm.Tiles[y]
		for x := 0; x < gm.Width; x++ {
			w, edited := want[[2]int{x, y}]
			if !edited {
				w = row[x].BaseWalkable
			}
			if row[x].Walkable != w {
				row[x].Walkable = w
				changed++
			}
		}
	}
	return changed
}
