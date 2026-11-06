package npc

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Map config files (mapdata/maps/*.txt) name their monsters and town NPCs.
// A name is our NpcType name (any case) when only one type has it, or one of
// the original server's names below for types we show under a shared or
// different name.
var configAliases = map[string]int{
	"guard-aresden": 1013, "guard-elvine": 1014, "guard-neutral": 1015,
	"city-dark-elf": 1059,
	// The original's Dark-Elf: NPC.cfg's City-Dark-Elf (type 54 is a hand-made stand-in)
	"dark-elf": 1059,
	// HGServer 3.51's spot-mob-generator names that differ from our NPC.cfg
	"orge": 29, "giant-frog": 57, "cannibal-plant": 60, "giant-plant": 76,
}

var (
	nameIndexOnce sync.Once
	uniqueNames   map[string]int   // lower-case name -> type, names only one type has
	sharedNames   map[string][]int // lower-case name -> types, names several types share
)

func buildNameIndex() {
	uniqueNames, sharedNames = map[string]int{}, map[string][]int{}
	byName := map[string][]int{}
	for id, t := range NpcTypes {
		n := strings.ToLower(t.Name)
		byName[n] = append(byName[n], id)
	}
	for n, ids := range byName {
		sort.Ints(ids)
		if len(ids) == 1 {
			uniqueNames[n] = ids[0]
		} else {
			sharedNames[n] = ids
		}
	}
}

// TypeForConfigName is the NPC type a map config name means.
func TypeForConfigName(name string) (int, error) {
	nameIndexOnce.Do(buildNameIndex)
	n := strings.ToLower(name)
	if id, ok := configAliases[n]; ok {
		return id, nil
	}
	if id, ok := uniqueNames[n]; ok {
		return id, nil
	}
	if ids, ok := sharedNames[n]; ok {
		return 0, fmt.Errorf("%q is the name of %d NPC types (%v); use a more specific name such as Guard-Aresden", name, len(ids), ids)
	}
	return 0, fmt.Errorf("no NPC or monster called %q", name)
}

// ConfigName is the name a map config file uses for an NPC type ("" if none).
func ConfigName(id int) string {
	nameIndexOnce.Do(buildNameIndex)
	if t := NpcTypes[id]; t != nil && uniqueNames[strings.ToLower(t.Name)] == id {
		return t.Name
	}
	best := "" // types shown under a shared name: the original server's (shortest) name
	for alias, aid := range configAliases {
		if aid == id && (best == "" || len(alias) < len(best) || (len(alias) == len(best) && alias < best)) {
			best = alias
		}
	}
	if best != "" {
		return displayAlias(best)
	}
	return ""
}

// displayAlias capitalises an alias the way the original files write it
// ("guard-aresden" -> "Guard-Aresden").
func displayAlias(a string) string {
	parts := strings.Split(a, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}
