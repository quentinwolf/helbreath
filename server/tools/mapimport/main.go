// Command mapimport converts the original server's map files (MAPDATA/*.txt)
// into this server's map configs (server/internal/mapdata/maps/<map>.txt).
//
//	go run ./server/tools/mapimport -src "<HelbreathServer-main>/Files/GameServers" \
//	    -maps assets/MAPDATA -out server/internal/mapdata/maps [-only arefarm,elvfarm] [-force]
//
// The files keep the original syntax and comments. Changes made on the way:
//   - spot-mob-generator's mob code becomes the monster's name (the codes only
//     meant something to a table inside HGServer's Game.cpp);
//   - lines the server can't use (unknown settings, mobs HGServer never
//     spawned) are commented out with the reason;
//   - Korean comments are converted from CP949 to UTF-8;
//   - a header says where the file came from.
//
// Only maps the port has (.amd in -maps) are written, named as their .amd file.
// The map configs are edited by hand and by the map editor once imported, so
// existing files are skipped unless -force is given.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// mobNames is HGServer's spot-mob-generator switch (Game.cpp ~26206): code -> name.
var mobNames = map[int]string{
	10: "Slime", 16: "Giant-Ant", 14: "Orc", 18: "Zombie", 11: "Skeleton", 6: "Orc-Mage",
	17: "Scorpion", 12: "Stone-Golem", 13: "Cyclops", 22: "Amphis", 23: "Clay-Golem",
	24: "Guard-Aresden", 25: "Guard-Elvine", 26: "Guard-Neutral", 27: "Hellbound", 29: "Orge",
	30: "Liche", 31: "Demon", 32: "Unicorn", 33: "WereWolf", 34: "Dummy", 35: "Attack-Dummy",
	48: "Stalker", 49: "Hellclaw", 50: "Tigerworm", 54: "Dark-Elf", 53: "Beholder",
	52: "Gagoyle", 57: "Giant-Frog", 58: "Mountain-Giant", 59: "Ettin", 60: "Cannibal-Plant",
	61: "Rudolph", 62: "DireBoar", 63: "Frost", 65: "Ice-Golem", 66: "Wyvern", 55: "Rabbit",
	67: "McGaffin", 68: "Perry", 69: "Devlin", 73: "Fire-Wyvern", 70: "Barlog", 80: "Tentocle",
	71: "Centaurus", 75: "Giant-Lizard", 78: "Minotaurs", 81: "Abaddon", 72: "Claw-Turtle",
	74: "Giant-Crayfish", 76: "Giant-Plant", 77: "MasterMage-Orc", 79: "Nizie",
}

// oldMobNames: dglv2's spots use the older numbering of the random mob
// generator (4 Zombie, 5 Skeleton...). HGServer's spot switch has no such
// cases, so that server left them empty; we read them the old way so the
// dungeon has its undead and golems.
var oldMobNames = map[int]string{
	4: "Zombie", 5: "Skeleton", 7: "Scorpion", 8: "Stone-Golem", 9: "Cyclops",
}

// notSpawned are codes HGServer's switch has commented out or can't resolve.
var notSpawned = map[int]string{
	56: "Cat: commented out in HGServer (\"NPCs not spawning in pits\")",
	28: "Troll: commented out in HGServer (\"NPCs not spawning in pits\")",
	51: "code 51: ambiguous in HGServer (Catapult or Abaddon)",
}

var tokenRE = regexp.MustCompile(`\S+`)

func main() {
	src := flag.String("src", "", "original GameServers folder (searched for MAPDATA/*.txt)")
	mapsDir := flag.String("maps", "assets/MAPDATA", "the port's .amd folder")
	out := flag.String("out", "server/internal/mapdata/maps", "output folder")
	only := flag.String("only", "", "comma-separated maps to import (default: all)")
	force := flag.Bool("force", false, "overwrite existing files")
	flag.Parse()
	if *src == "" {
		log.Fatal("-src is required")
	}
	want := map[string]bool{}
	for _, m := range strings.Split(*only, ",") {
		if m = strings.TrimSpace(m); m != "" {
			want[strings.ToLower(m)] = true
		}
	}

	portMaps := map[string]string{}
	entries, err := os.ReadDir(*mapsDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".amd") {
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			portMaps[strings.ToLower(name)] = name
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	seen := map[string]string{}
	written := 0
	err = filepath.Walk(*src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".txt") ||
			!strings.EqualFold(filepath.Base(filepath.Dir(path)), "mapdata") {
			return err
		}
		key := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		mapName, ok := portMaps[key]
		if !ok || (len(want) > 0 && !want[key]) {
			return nil
		}
		if first, dup := seen[key]; dup {
			fmt.Fprintf(os.Stderr, "skip duplicate %s (using %s)\n", path, first)
			return nil
		}
		seen[key] = path
		dst := filepath.Join(*out, mapName+".txt")
		if _, err := os.Stat(dst); err == nil && !*force {
			fmt.Fprintf(os.Stderr, "%s exists, skipped (use -force to overwrite)\n", dst)
			return nil
		}
		rel, _ := filepath.Rel(*src, path)
		text, notes, err := convert(path, mapName, filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		for _, n := range notes {
			fmt.Fprintf(os.Stderr, "%s: %s\n", mapName, n)
		}
		if _, errs := mapdata.ParseMapConfig(mapName, []byte(text)); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "CHECK %v\n", e)
			}
		}
		written++
		return os.WriteFile(dst, []byte(text), 0o644)
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "%d map configs written to %s\n", written, *out)
}

// convert reads one original file and returns it in our format.
func convert(path, mapName, rel string) (string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	if !utf8.Valid(data) {
		if dec, err := korean.EUCKR.NewDecoder().Bytes(data); err == nil {
			data = dec
		}
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	var notes []string
	var b strings.Builder
	fmt.Fprintf(&b, "// %s: from the original server's map file (HB Server Files 3.51,\n", mapName)
	fmt.Fprintf(&b, "// GameServers/%s), converted by server/tools/mapimport.\n", rel)
	b.WriteString("// Format and settings: docs/map-config.md.\n\n")
	for i, raw := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		raw = strings.TrimRight(raw, " \t")
		line := mapdata.SplitConfigLine(i+1, raw)
		switch {
		case line.Key == "":
			b.WriteString(raw)
		case !mapdata.IsKnownConfigKey(line.Key):
			fmt.Fprintf(&b, "//%s   // mapimport: unknown setting %q, commented out", raw, line.Key)
			notes = append(notes, fmt.Sprintf("line %d: unknown setting %q commented out", i+1, line.Key))
		case line.Key == "spot-mob-generator":
			conv, note := convertSpot(raw)
			b.WriteString(conv)
			if note != "" {
				notes = append(notes, fmt.Sprintf("line %d: %s", i+1, note))
			}
		default:
			b.WriteString(raw)
		}
		b.WriteString("\n")
	}
	return b.String(), notes, nil
}

// convertSpot puts the monster's name in place of the mob code (the 7th value).
func convertSpot(raw string) (string, string) {
	eq := strings.Index(raw, "=")
	body := raw
	if c := strings.Index(body, "//"); c >= 0 {
		body = body[:c]
	}
	if eq < 0 || eq > len(body) {
		return "//" + raw + "   // mapimport: no '=', commented out", "spot line without '=' commented out"
	}
	locs := tokenRE.FindAllStringIndex(body[eq+1:], -1)
	if len(locs) < 8 {
		return "//" + raw + "   // mapimport: too few values, commented out", "short spot line commented out"
	}
	at := locs[6]
	start, end := eq+1+at[0], eq+1+at[1]
	code, err := strconv.Atoi(raw[start:end])
	if err != nil {
		return raw, "" // already a name
	}
	name, ok := mobNames[code]
	if !ok {
		name, ok = oldMobNames[code]
	}
	reason := notSpawned[code]
	if !ok && reason == "" {
		reason = fmt.Sprintf("mob code %d isn't in HGServer's spot list", code)
	}
	if ok {
		id, err := npc.TypeForConfigName(name)
		if err != nil {
			reason = fmt.Sprintf("%s: %v", name, err)
		} else {
			name = npc.ConfigName(id)
		}
	}
	if reason != "" {
		return "//" + raw + "   // mapimport: " + reason, "spot commented out: " + reason
	}
	return raw[:start] + name + raw[end:], ""
}
