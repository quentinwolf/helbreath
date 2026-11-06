package mapdata

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Writing map config files (for the map editor). EditMapConfig changes only the
// lines that need it, so comments, blank lines and everything the editor doesn't
// know about stay as they were.

// DoorTile is one tile of a door as the editor has it.
type DoorTile struct {
	X, Y         int
	DestMap      string
	DestX, DestY int
	Dir          int
	Note         string // written as the line's comment ("" keeps an existing one)
}

// SpawnEdit is one spawn as the editor has it.
type SpawnEdit struct {
	Src  string // the line it came from: "spot:<num>" or "npc:<line>" ("" = new)
	Name string // monster or NPC name as the config writes it
	// Spot: a spot-mob-generator keeping up to Max alive in Area. Otherwise an
	// npc line: one NPC standing at X, Y.
	Spot bool
	Area Rect
	Max  int
	X, Y int
	SpawnOptions
	Note string // the line's comment
}

// MapEdit is what to change in one map's config. nil leaves that part alone.
type MapEdit struct {
	Doors  *[]DoorTile // the map's door tiles; also sets exact-doors = 1
	Spawns *[]SpawnEdit
	// MapExists says whether a door's destination exists. Doors to other maps
	// (the original's doors to maps the port lacks) are left in the file untouched.
	MapExists func(string) bool
}

// EditMapConfig returns the config text with the edit applied.
func EditMapConfig(name string, text []byte, edit MapEdit) (string, error) {
	c, errs := ParseMapConfig(name, text)
	if len(errs) > 0 {
		return "", fmt.Errorf("%s.txt has errors, fix them first: %v", name, errs[0])
	}
	w := newConfigWriter(string(text), c)
	if edit.Doors != nil {
		exists := edit.MapExists
		if exists == nil {
			exists = func(string) bool { return true }
		}
		w.setDoors(*edit.Doors, exists)
		w.setSetting("exact-doors", "1", "doors work on exactly the tiles listed (set by the map editor)")
	}
	if edit.Spawns != nil {
		if err := w.setSpawns(*edit.Spawns); err != nil {
			return "", err
		}
	}
	return w.text(), nil
}

// configWriter edits a file's lines: each original line can be kept, replaced
// or removed, and new lines go after a given original line.
type configWriter struct {
	c      *MapConfig
	lines  []string         // original lines (index = line number - 1)
	repl   map[int][]string // index -> replacement lines (empty = removed)
	after  map[int][]string // index -> lines inserted after it (-1 = at the top)
	crlf   bool
	newWps int // waypoint numbers handed out so far
}

func newConfigWriter(text string, c *MapConfig) *configWriter {
	crlf := strings.Contains(text, "\r\n")
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return &configWriter{c: c, lines: strings.Split(text, "\n"), repl: map[int][]string{},
		after: map[int][]string{}, crlf: crlf}
}

func (w *configWriter) replace(line int, text string) { w.repl[line-1] = []string{text} }
func (w *configWriter) remove(line int)               { w.repl[line-1] = []string{} }
func (w *configWriter) removed(line int) bool {
	r, ok := w.repl[line-1]
	return ok && len(r) == 0
}

// insertAfter adds lines after original line `line` (0 = at the top).
func (w *configWriter) insertAfter(line int, texts ...string) {
	w.after[line-1] = append(w.after[line-1], texts...)
}

func (w *configWriter) text() string {
	var out []string
	out = append(out, w.after[-1]...)
	for i, l := range w.lines {
		if r, ok := w.repl[i]; ok {
			out = append(out, r...)
		} else {
			out = append(out, l)
		}
		out = append(out, w.after[i]...)
	}
	nl := "\n"
	if w.crlf {
		nl = "\r\n"
	}
	return strings.Join(out, nl) + nl
}

// endLine is the line new settings go before when there's nothing similar to
// put them after: the line before [END-MAP-INFO], or the last line.
func (w *configWriter) endLine() int {
	for i := len(w.lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(w.lines[i]), "[END-MAP-INFO]") {
			return i // insert after line i (1-based i = the line before the marker)
		}
	}
	return len(w.lines)
}

// lastLineOf is the last line with key (0 = none).
func (w *configWriter) lastLineOf(key string) int {
	last := 0
	for _, l := range w.c.Lines {
		if l.Key == key {
			last = l.No
		}
	}
	return last
}

func withComment(text, comment string) string {
	if comment = strings.TrimSpace(comment); comment != "" {
		return text + "   // " + comment
	}
	return text
}

// setSetting sets a one-value setting, adding it after map-location if missing.
func (w *configWriter) setSetting(key, value, comment string) {
	for _, l := range w.c.Lines {
		if l.Key == key {
			if len(l.Values) == 1 && l.Values[0] == value {
				return
			}
			w.replace(l.No, withComment(key+" = "+value, l.Comment))
			return
		}
	}
	at := w.lastLineOf("map-location")
	if at == 0 {
		for _, l := range w.c.Lines {
			if strings.HasPrefix(strings.TrimSpace(l.Text), "[MAP-INFO]") {
				at = l.No
			}
		}
	}
	w.insertAfter(at, "// Port setting: "+comment, key+" = "+value)
}

// --- doors ---

func formatDoor(d DoorTile) string {
	return fmt.Sprintf("teleport-loc     =     %-6d %-6d %-19s %-6d %-6d %d", d.X, d.Y, d.DestMap, d.DestX, d.DestY, d.Dir)
}

func sameDest(t TeleportLoc, d DoorTile) bool {
	return strings.EqualFold(t.DestMap, d.DestMap) && t.DestX == d.DestX && t.DestY == d.DestY && t.Dir == d.Dir
}

func (w *configWriter) setDoors(doors []DoorTile, exists func(string) bool) {
	want := map[[2]int]DoorTile{}
	for _, d := range doors {
		want[[2]int{d.X, d.Y}] = d
	}
	done := map[[2]int]bool{}
	kept := map[string]int{} // destination -> last line now holding it
	destKey := func(m string, x, y, dir int) string { return fmt.Sprintf("%s|%d|%d|%d", strings.ToLower(m), x, y, dir) }
	lastDoor := 0
	for _, t := range w.c.Teleports {
		lastDoor = t.Line
		if !exists(t.DestMap) {
			continue // a door to a map the port doesn't have: left as it is
		}
		at := [2]int{t.X, t.Y}
		d, ok := want[at]
		switch {
		case done[at] || !ok:
			w.remove(t.Line)
			continue
		case !sameDest(t, d) || (d.Note != "" && d.Note != t.Comment):
			comment := t.Comment
			if d.Note != "" {
				comment = d.Note
			}
			w.replace(t.Line, withComment(formatDoor(d), comment))
		}
		done[at] = true
		kept[destKey(d.DestMap, d.DestX, d.DestY, d.Dir)] = t.Line
	}
	// New tiles, by destination, after a line with the same destination if any
	groups := map[string][]DoorTile{}
	var order []string
	for _, d := range doors {
		if done[[2]int{d.X, d.Y}] {
			continue
		}
		k := destKey(d.DestMap, d.DestX, d.DestY, d.Dir)
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], d)
		done[[2]int{d.X, d.Y}] = true
	}
	for _, k := range order {
		g := groups[k]
		sort.Slice(g, func(i, j int) bool { return g[i].Y < g[j].Y || (g[i].Y == g[j].Y && g[i].X < g[j].X) })
		at, ok := kept[k]
		if !ok {
			at = lastDoor
		}
		if at == 0 {
			at = w.endLine()
		}
		var texts []string
		for _, d := range g {
			texts = append(texts, withComment(formatDoor(d), d.Note))
		}
		w.insertAfter(at, texts...)
	}
}

// --- spawns ---

func formatOptions(o SpawnOptions) string {
	s := ""
	if o.Dir != 0 {
		s += " dir=" + strconv.Itoa(o.Dir)
	}
	if o.Respawn != 0 {
		s += " respawn=" + strconv.Itoa(o.Respawn)
	}
	return s
}

func formatSpot(num int, e SpawnEdit) string {
	return withComment(fmt.Sprintf("spot-mob-generator = %-3d 1   %d %d %d %d   %s %d%s",
		num, e.Area.X1, e.Area.Y1, e.Area.X2, e.Area.Y2, e.Name, e.Max, formatOptions(e.SpawnOptions)), e.Note)
}

func formatNpc(n NpcSetting, name string, wps []int, o SpawnOptions, note string) string {
	ws := make([]string, len(wps))
	for i, wp := range wps {
		ws[i] = strconv.Itoa(wp)
	}
	mt := n.MoveType
	if mt == 0 {
		mt = 1
	}
	return withComment(fmt.Sprintf("npc               =   %-18s %d   %s   1%s", name, mt, strings.Join(ws, " "), formatOptions(o)), note)
}

func formatWaypoint(num, x, y int) string {
	return fmt.Sprintf("waypoint          =     %-6d %-5d %d", num, x, y)
}

func (w *configWriter) setSpawns(edits []SpawnEdit) error {
	c := w.c
	for i, e := range edits {
		if strings.TrimSpace(e.Name) == "" || strings.ContainsAny(e.Name, " \t=/") {
			return fmt.Errorf("spawn %d: bad name %q", i+1, e.Name)
		}
		if e.Spot && (e.Max < 1 || e.Area.X2 < e.Area.X1 || e.Area.Y2 < e.Area.Y1) {
			return fmt.Errorf("spawn %d (%s): a spot needs a count and an area", i+1, e.Name)
		}
	}
	// Which spot lines and npc lines the editor's spawns came from
	spots := map[int]*SpotMobGenerator{}
	for i := range c.SpotMobs {
		s := &c.SpotMobs[i]
		if s.Type == 1 && s.Max > 0 {
			spots[s.Num] = s
		}
	}
	npcs := map[int]*NpcSetting{}
	for i := range c.NPCs {
		npcs[c.NPCs[i].Line] = &c.NPCs[i]
	}
	spotOf := make([]*SpotMobGenerator, len(edits))
	npcOf := make([]*NpcSetting, len(edits))
	usedSpot := map[int]bool{}
	usedNpc := map[int]bool{}
	// 1. by source line
	for i, e := range edits {
		kind, id, _ := strings.Cut(e.Src, ":")
		n, err := strconv.Atoi(id)
		if err != nil {
			continue
		}
		if kind == "spot" && e.Spot && spots[n] != nil && !usedSpot[n] {
			spotOf[i], usedSpot[n] = spots[n], true
		}
		if kind == "npc" && !e.Spot && npcs[n] != nil && !usedNpc[n] && strings.EqualFold(npcs[n].Name, e.Name) {
			npcOf[i], usedNpc[n] = npcs[n], true
		}
	}
	// 2. same monster and area / same NPC name, nearest first
	for i, e := range edits {
		if !e.Spot || spotOf[i] != nil {
			continue
		}
		for num, s := range spots {
			if !usedSpot[num] && strings.EqualFold(s.Monster, e.Name) && s.Area == e.Area {
				spotOf[i], usedSpot[num] = s, true
				break
			}
		}
	}
	for i, e := range edits {
		if e.Spot || npcOf[i] != nil {
			continue
		}
		best, bestDist := 0, -1
		for line, n := range npcs {
			if usedNpc[line] || !strings.EqualFold(n.Name, e.Name) {
				continue
			}
			p, _ := c.NpcPosition(*n)
			d := abs(p.X-e.X) + abs(p.Y-e.Y)
			if bestDist < 0 || d < bestDist || (d == bestDist && line < best) {
				best, bestDist = line, d
			}
		}
		if bestDist >= 0 {
			npcOf[i], usedNpc[best] = npcs[best], true
		}
	}

	// Waypoints: their lines, and which npc lines use each one
	wpLine := map[int]int{}
	maxWp := 0
	for _, l := range c.Lines {
		if l.Key == "waypoint" && len(l.Values) > 0 {
			if n, err := strconv.Atoi(l.Values[0]); err == nil {
				wpLine[n] = l.No
				maxWp = max(maxWp, n)
			}
		}
	}
	wpUsers := map[int]map[int]bool{} // waypoint -> npc lines using it
	for _, n := range c.NPCs {
		for _, wp := range n.Waypoints {
			if wpUsers[wp] == nil {
				wpUsers[wp] = map[int]bool{}
			}
			wpUsers[wp][n.Line] = true
		}
	}
	ownWaypoints := func(n *NpcSetting) bool {
		for _, wp := range n.Waypoints {
			if len(wpUsers[wp]) != 1 || wpLine[wp] == 0 {
				return false
			}
		}
		return len(n.Waypoints) > 0
	}
	newWp := func() int {
		maxWp++
		return maxWp
	}

	// Lines no spawn came from are removed (an npc's own waypoints with it)
	for num, s := range spots {
		if !usedSpot[num] {
			w.remove(s.Line)
		}
	}
	for line, n := range npcs {
		if usedNpc[line] {
			continue
		}
		if ownWaypoints(n) {
			for _, wp := range uniqueInts(n.Waypoints) {
				w.remove(wpLine[wp])
			}
		}
		w.remove(line)
	}

	// Kept lines: rewritten where they changed
	maxSpot := 0
	for _, s := range c.SpotMobs {
		maxSpot = max(maxSpot, s.Num)
	}
	for _, l := range c.Lines { // commented-out spot lines keep their numbers too
		if n := commentedSpotNum(l.Text); n > maxSpot {
			maxSpot = n
		}
	}
	var newSpots, newNpcs []string
	var newNpcAt []int
	for i, e := range edits {
		if e.Spot {
			if s := spotOf[i]; s != nil {
				same := strings.EqualFold(s.Monster, e.Name) && s.Area == e.Area && s.Max == e.Max &&
					s.SpawnOptions == e.SpawnOptions && strings.TrimSpace(s.Comment) == strings.TrimSpace(e.Note)
				if !same {
					w.replace(s.Line, formatSpot(s.Num, e))
				}
				continue
			}
			maxSpot++
			newSpots = append(newSpots, formatSpot(maxSpot, e))
			continue
		}
		if n := npcOf[i]; n != nil {
			p, _ := c.NpcPosition(*n)
			wps := n.Waypoints
			moved := p.X != e.X || p.Y != e.Y
			if moved && ownWaypoints(n) {
				for _, wp := range uniqueInts(n.Waypoints) {
					w.replace(wpLine[wp], formatWaypoint(wp, e.X, e.Y))
				}
			} else if moved {
				wp := newWp()
				wps = repeatInt(wp, 10)
				w.insertAfter(n.Line-1, formatWaypoint(wp, e.X, e.Y))
			}
			same := !moved || ownWaypoints(n)
			if !same || !strings.EqualFold(n.Name, e.Name) || n.SpawnOptions != e.SpawnOptions ||
				strings.TrimSpace(n.Comment) != strings.TrimSpace(e.Note) {
				w.replace(n.Line, formatNpc(*n, e.Name, wps, e.SpawnOptions, e.Note))
			}
			continue
		}
		wp := newWp()
		newNpcs = append(newNpcs, formatWaypoint(wp, e.X, e.Y),
			formatNpc(NpcSetting{MoveType: 1}, e.Name, repeatInt(wp, 10), e.SpawnOptions, e.Note))
		newNpcAt = append(newNpcAt, i)
	}
	if len(newSpots) > 0 {
		at := w.lastLineOf("spot-mob-generator")
		if at == 0 {
			at = w.endLine()
		}
		w.insertAfter(at, newSpots...)
	}
	if len(newNpcs) > 0 {
		at := w.lastLineOf("npc")
		if at == 0 {
			at = w.lastLineOf("waypoint")
		}
		if at == 0 {
			at = w.endLine()
		}
		w.insertAfter(at, newNpcs...)
	}
	return nil
}

// commentedSpotNum is the number of a commented-out spot-mob-generator line (0 if not one).
func commentedSpotNum(text string) int {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "//") {
		return 0
	}
	t = strings.TrimLeft(t, "/ \t")
	if !strings.HasPrefix(strings.ToLower(t), "spot-mob-generator") {
		return 0
	}
	l := SplitConfigLine(0, t)
	if len(l.Values) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(l.Values[0])
	return n
}

func uniqueInts(v []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range v {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func repeatInt(v, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}
