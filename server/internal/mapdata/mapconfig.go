package mapdata

import (
	"fmt"
	"strconv"
	"strings"
)

// Map config files (maps/<map>.txt) use the original server's MAPDATA/*.txt
// syntax, one setting per line:
//
//	key = value value ...      // comment
//
// Values are separated by spaces or tabs; "//" starts a comment anywhere on a
// line; the [MAP-INFO] / [END-MAP-INFO] markers are allowed and ignored. The
// one change from the original is that spot-mob-generator names its monster
// ("Slime") where the original had a mob code from a table inside its server.
// See docs/map-config.md for every key.

// MapConfig is one map's config file.
type MapConfig struct {
	Name string // the map, spelled as its .amd file (= the config file's name)

	Location        string // map-location: the map's town side ("aresden", "elvine") or name
	InitialPoints   []IndexedPoint
	LevelLimit      int
	UpperLevelLimit int
	PKMode          int
	FixedDay        bool // fixed-dayornight-mode: always day, never weather (indoors)
	MaxObjects      int  // maximum-object: cap on monsters on the map (0 = none)
	RandomMobs      RandomMobGenerator

	Teleports   []TeleportLoc
	Waypoints   map[int]Point
	NoAttack    []NumberedRect
	SpotMobs    []SpotMobGenerator
	NPCs        []NpcSetting
	FishPoints  []IndexedPoint
	MaxFish     int
	Minerals    MineralGenerator
	MineralPts  []IndexedPoint
	MaxMinerals int

	// Lines is the file as read, line by line (comments and blank lines too),
	// so an editor can change some settings and write the rest back untouched.
	Lines []ConfigLine
}

// ConfigLine is one line of a config file. Key is "" for blank and comment-only lines.
type ConfigLine struct {
	No      int      // 1-based line number
	Text    string   // the whole line as written
	Key     string   // lower case
	Values  []string // values after the key (comment removed)
	Comment string   // text after "//", if any
}

// Point is a tile.
type Point struct{ X, Y int }

// IndexedPoint is a numbered tile (initial-point, fish-point, mineral-point).
type IndexedPoint struct {
	Num  int
	X, Y int
}

// Rect is a tile rectangle, corners included.
type Rect struct{ X1, Y1, X2, Y2 int }

// Contains reports whether (x, y) is inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X1 && x <= r.X2 && y >= r.Y1 && y <= r.Y2
}

// NumberedRect is a numbered rectangle (no-attack-area).
type NumberedRect struct {
	Num int
	Rect
}

// TeleportLoc is a door tile: stepping on (X, Y) takes you to DestMap.
type TeleportLoc struct {
	X, Y         int
	DestMap      string
	DestX, DestY int
	Dir          int
}

// RandomMobGenerator is random-mob-generator = <on 0/1> <level>.
type RandomMobGenerator struct {
	On    bool
	Level int
}

// SpotMobGenerator is spot-mob-generator = <num> <type> <left> <top> <right> <bottom> <monster> <max>.
// Type 1 keeps up to Max of the monster alive at random spots inside the
// rectangle, wandering there (the original's DEF_MOVETYPE_RANDOMAREA).
type SpotMobGenerator struct {
	Num     int
	Type    int
	Area    Rect
	Monster string
	Max     int
	Line    int
}

// NpcSetting is npc = <name> <move type> <waypoint x10> <name prefix>: a town
// NPC standing at (or walking between) the given waypoints.
type NpcSetting struct {
	Name      string
	MoveType  int
	Waypoints []int
	Line      int
}

// MineralGenerator is mineral-generator = <on 0/1> <level>.
type MineralGenerator struct {
	On    bool
	Level int
}

// knownKeys are every key the original server read (Game.cpp
// _bDecodeMapDataFileContents) plus "not-permanent-map". Keys not listed are
// errors (usually a typo).
var knownKeys = map[string]bool{
	"map-location": true, "initial-point": true, "teleport-loc": true, "waypoint": true,
	"npc": true, "random-mob-generator": true, "maximum-object": true, "npc-avoidrect": true,
	"spot-mob-generator": true, "no-attack-area": true, "fixed-dayornight-mode": true,
	"fish-point": true, "max-fish": true, "type": true, "level-limit": true,
	"mineral-generator": true, "mineral-point": true, "max-mineral": true,
	"upper-level-limit": true, "strategic-point": true, "energy-sphere-creation-point": true,
	"energy-sphere-goal-point": true, "strike-point": true, "item-event": true,
	"energy-sphere-auto-creation": true, "mobevent-amount": true, "apocalypsemobgentype": true,
	"apocalypsebossmob": true, "dynamicgatetype": true, "dynamicgatecoord": true,
	"recallimpossible": true, "apocalypsemap": true, "citizenlimit": true, "heldenianmap": true,
	"heldeniantower": true, "heldenianmodemap": true, "heldenianwinningzone": true,
	"heldeniangatedoor": true, "pk-mode": true, "limited-cast": true, "drop-type": true,
	"not-permanent-map": true, "boss-rate": true, "extra-drop": true,
}

// ParseMapConfig reads one config file. name is the map (the file's name
// without .txt). Every problem found is returned, with line numbers; the
// config is still filled in from the lines that were fine.
func ParseMapConfig(name string, data []byte) (*MapConfig, []error) {
	c := &MapConfig{Name: name, Waypoints: map[int]Point{}}
	var errs []error
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(strings.TrimPrefix(text, string(rune(0xFEFF))), "\n")
	for i, raw := range strings.Split(text, "\n") {
		line := SplitConfigLine(i+1, raw)
		c.Lines = append(c.Lines, line)
		if line.Key == "" {
			continue
		}
		if err := c.apply(line); err != nil {
			errs = append(errs, fmt.Errorf("%s.txt line %d: %w", name, line.No, err))
		}
	}
	return c, errs
}

// IsKnownConfigKey reports whether key (any case) is a map config setting.
func IsKnownConfigKey(key string) bool { return knownKeys[strings.ToLower(key)] }

// SplitConfigLine splits a line into key, values and comment.
func SplitConfigLine(no int, raw string) ConfigLine {
	line := ConfigLine{No: no, Text: raw}
	body := raw
	if i := strings.Index(body, "//"); i >= 0 {
		line.Comment = strings.TrimSpace(body[i+2:])
		body = body[:i]
	}
	body = strings.TrimSpace(body)
	if body == "" || strings.HasPrefix(body, "[") || strings.HasPrefix(body, "/") {
		return line // blank, comment, section marker, or a "/----" rule line
	}
	var key, rest string
	if k, r, ok := strings.Cut(body, "="); ok {
		key, rest = k, r
	} else {
		fields := strings.Fields(body)
		key, rest = fields[0], strings.Join(fields[1:], " ")
	}
	line.Key = strings.ToLower(strings.TrimSpace(key))
	line.Values = strings.Fields(rest)
	return line
}

// apply reads one setting into c.
func (c *MapConfig) apply(l ConfigLine) error {
	if !knownKeys[l.Key] {
		return fmt.Errorf("unknown setting %q", l.Key)
	}
	v := l.Values
	ints := func(n int) ([]int, error) {
		if len(v) < n {
			return nil, fmt.Errorf("%s needs %d numbers, has %d values", l.Key, n, len(v))
		}
		out := make([]int, n)
		for i := 0; i < n; i++ {
			x, err := strconv.Atoi(v[i])
			if err != nil {
				return nil, fmt.Errorf("%s: value %d (%q) isn't a number", l.Key, i+1, v[i])
			}
			out[i] = x
		}
		return out, nil
	}
	one := func(dst *int) error {
		n, err := ints(1)
		if err == nil {
			*dst = n[0]
		}
		return err
	}
	switch l.Key {
	case "map-location":
		if len(v) < 1 {
			return fmt.Errorf("map-location needs a name")
		}
		c.Location = v[0]
	case "initial-point":
		n, err := ints(3)
		if err != nil {
			return err
		}
		c.InitialPoints = append(c.InitialPoints, IndexedPoint{n[0], n[1], n[2]})
	case "level-limit":
		return one(&c.LevelLimit)
	case "upper-level-limit":
		return one(&c.UpperLevelLimit)
	case "pk-mode":
		return one(&c.PKMode)
	case "fixed-dayornight-mode":
		var on int
		err := one(&on)
		c.FixedDay = on != 0
		return err
	case "maximum-object":
		return one(&c.MaxObjects)
	case "random-mob-generator":
		n, err := ints(2)
		if err != nil {
			return err
		}
		c.RandomMobs = RandomMobGenerator{On: n[0] != 0, Level: n[1]}
	case "teleport-loc":
		if len(v) < 6 {
			return fmt.Errorf("teleport-loc needs x y destination-map dest-x dest-y direction")
		}
		n, err := numbers(l.Key, v[0], v[1], v[3], v[4], v[5])
		if err != nil {
			return err
		}
		c.Teleports = append(c.Teleports, TeleportLoc{X: n[0], Y: n[1], DestMap: v[2], DestX: n[2], DestY: n[3], Dir: n[4]})
	case "waypoint":
		n, err := ints(3)
		if err != nil {
			return err
		}
		if _, dup := c.Waypoints[n[0]]; dup {
			return fmt.Errorf("waypoint %d is defined twice", n[0])
		}
		c.Waypoints[n[0]] = Point{n[1], n[2]}
	case "no-attack-area":
		n, err := ints(5)
		if err != nil {
			return err
		}
		c.NoAttack = append(c.NoAttack, NumberedRect{n[0], Rect{n[1], n[2], n[3], n[4]}})
	case "spot-mob-generator":
		if len(v) < 8 {
			return fmt.Errorf("spot-mob-generator needs num type left top right bottom monster max")
		}
		n, err := numbers(l.Key, v[0], v[1], v[2], v[3], v[4], v[5], v[7])
		if err != nil {
			return err
		}
		if _, err := strconv.Atoi(v[6]); err == nil {
			return fmt.Errorf("spot-mob-generator %d: write the monster's name (e.g. Slime), not a number", n[0])
		}
		for _, s := range c.SpotMobs {
			if s.Num == n[0] {
				return fmt.Errorf("spot-mob-generator %d is defined twice", n[0])
			}
		}
		c.SpotMobs = append(c.SpotMobs, SpotMobGenerator{Num: n[0], Type: n[1],
			Area: Rect{n[2], n[3], n[4], n[5]}, Monster: v[6], Max: n[6], Line: l.No})
	case "npc":
		if len(v) < 3 {
			return fmt.Errorf("npc needs name move-type waypoints... prefix")
		}
		mt, err := strconv.Atoi(v[1])
		if err != nil {
			return fmt.Errorf("npc %s: move type %q isn't a number", v[0], v[1])
		}
		// up to 10 waypoint numbers, then the name prefix (ignored here)
		wps := v[2:]
		if len(wps) > 10 {
			wps = wps[:10]
		} else if len(wps) > 1 {
			wps = wps[:len(wps)-1]
		}
		ns, err := numbers(l.Key, wps...)
		if err != nil {
			return err
		}
		c.NPCs = append(c.NPCs, NpcSetting{Name: v[0], MoveType: mt, Waypoints: ns, Line: l.No})
	case "fish-point":
		n, err := ints(3)
		if err != nil {
			return err
		}
		c.FishPoints = append(c.FishPoints, IndexedPoint{n[0], n[1], n[2]})
	case "max-fish":
		return one(&c.MaxFish)
	case "mineral-generator":
		n, err := ints(2)
		if err != nil {
			return err
		}
		c.Minerals = MineralGenerator{On: n[0] != 0, Level: n[1]}
	case "mineral-point":
		n, err := ints(3)
		if err != nil {
			return err
		}
		c.MineralPts = append(c.MineralPts, IndexedPoint{n[0], n[1], n[2]})
	case "max-mineral":
		return one(&c.MaxMinerals)
	}
	// Other known keys are kept in Lines only until their system is ported.
	return nil
}

func numbers(key string, vals ...string) ([]int, error) {
	out := make([]int, len(vals))
	for i, s := range vals {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("%s: %q isn't a number", key, s)
		}
		out[i] = n
	}
	return out, nil
}

// NpcPosition is where an npc line puts its NPC: its first waypoint.
func (c *MapConfig) NpcPosition(n NpcSetting) (Point, error) {
	if len(n.Waypoints) == 0 {
		return Point{}, fmt.Errorf("npc %s has no waypoints", n.Name)
	}
	p, ok := c.Waypoints[n.Waypoints[0]]
	if !ok {
		return Point{}, fmt.Errorf("npc %s: waypoint %d isn't defined", n.Name, n.Waypoints[0])
	}
	return p, nil
}
