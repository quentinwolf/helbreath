package game

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// The map configs (maps/<map>.txt; docs/map-config.md: doors, spawns, town NPCs)
// and the map editor's walkable.json (docs/map-editor.md) are read from a folder
// on disk and reloaded within a second of being saved, so edits take effect
// without restarting the server.

// mapEditsWatch remembers which version of each file is loaded.
type mapEditsWatch struct {
	dir       string
	lastCheck time.Time
	// per file: the stamp now loaded, and the last stamp that failed to load
	loaded, failed map[string]string
	// baseSpawns: the spawn list (nil until SpawnNPCs); baseFromConfigs: it came
	// from the map configs, so it's rebuilt when they change.
	// spawnSig: per map (lower case), its spawns as running ("" = none).
	baseSpawns      *SpawnConfig
	baseFromConfigs bool
	spawnSig        map[string]string
}

// mapConfigsKey is the loaded/failed key for the maps/ folder.
const mapConfigsKey = mapdata.MapConfigDir + "/"

// SetMapEditsDir sets the folder holding maps/*.txt and walkable.json. With
// "" (or a folder that doesn't exist) the copies built into the server are used
// and nothing is reloaded. Call before LoadMaps.
func (e *Engine) SetMapEditsDir(dir string) {
	if dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			log.Printf("Map edits: folder %q not found, using the built-in map configs and walkable.json", dir)
			dir = ""
		}
	}
	e.mapEdits.dir = dir
}

// loadMapEdits reads the map configs and walkable.json once the maps are loaded.
func (e *Engine) loadMapEdits() {
	e.initMapEdits()
	if e.mapEdits.dir != "" {
		log.Printf("Map edits: reading %s*.txt and %s from %s (reloaded when they change)",
			mapConfigsKey, mapdata.WalkableFile, e.mapEdits.dir)
	}
	e.reloadMapConfigs(false)
	e.reloadMapEdit(mapdata.WalkableFile, false)
}

func (e *Engine) initMapEdits() {
	if e.mapEdits.loaded == nil {
		e.mapEdits.loaded = map[string]string{}
		e.mapEdits.failed = map[string]string{}
		e.mapEdits.spawnSig = map[string]string{}
	}
}

// trackSpawns remembers the spawn list the server starts with (SpawnNPCs), so
// a reload can tell which maps changed.
func (e *Engine) trackSpawns(base *SpawnConfig) *SpawnConfig {
	e.initMapEdits()
	e.mapEdits.baseSpawns = base
	e.mapEdits.spawnSig = spawnSignatures(base)
	return base
}

// spawnSignatures is each map's spawn entries as JSON, to see which maps changed.
func spawnSignatures(cfg *SpawnConfig) map[string]string {
	byMap := map[string][]SpawnEntry{}
	for _, e := range cfg.Spawns {
		m := strings.ToLower(e.MapName)
		byMap[m] = append(byMap[m], e)
	}
	sig := map[string]string{}
	for m, entries := range byMap {
		b, _ := json.Marshal(entries)
		sig[m] = string(b)
	}
	return sig
}

// checkMapEdits reloads a file whose stamp changed since it was loaded.
func (e *Engine) checkMapEdits() {
	if e.mapEdits.dir == "" || e.mapEdits.loaded == nil {
		return
	}
	files := []string{mapdata.WalkableFile}
	if stamp := mapdata.ConfigStamp(e.mapEdits.dir); stamp != e.mapEdits.loaded[mapConfigsKey] {
		e.reloadMapConfigs(true)
	}
	for _, name := range files {
		stamp := mapdata.EditFileStamp(e.mapEdits.dir, name)
		if stamp != e.mapEdits.loaded[name] && stamp != e.mapEdits.failed[name] {
			e.reloadMapEdit(name, true)
		}
	}
}

// reloadMapEdit loads one file. live: the server is running, so tell players
// whose map changed and the admins online.
func (e *Engine) reloadMapEdit(name string, live bool) {
	dir := e.mapEdits.dir
	stamp := mapdata.EditFileStamp(dir, name)
	var summary string
	var err error
	switch name {
	case mapdata.WalkableFile:
		var o mapdata.WalkOverrides
		o, err = mapdata.LoadWalkOverridesFrom(dir)
		if err == nil {
			var changed []string
			for _, gm := range e.maps {
				if n := o.Reapply(gm); n > 0 {
					changed = append(changed, gm.Name+" ("+plural(n, "tile")+")")
					if live {
						e.sendCollisionUpdate(gm)
					}
				}
			}
			summary = "walkability changed on " + strings.Join(changed, ", ")
			if len(changed) == 0 {
				summary = "no walkability changes"
			}
		}
	}
	if err != nil {
		e.mapEdits.failed[name] = stamp
		log.Printf("WARNING: map edits: %v (keeping what was loaded before)", err)
		if live {
			e.tellAdmins("Map edits: "+name+" has an error, not loaded: "+err.Error(), 2)
		}
		return
	}
	e.mapEdits.loaded[name] = stamp
	e.mapEdits.failed[name] = ""
	if live {
		log.Printf("Map edits: reloaded %s: %s", name, summary)
		e.tellAdmins("Map edits: reloaded "+name+": "+summary, 1)
	} else if summary != "no walkability changes" && summary != "" {
		log.Printf("Map edits: %s: %s", name, summary)
	}
}

// doorAt is the door a player stepping onto (x, y) uses (see TeleportTable.DoorAt).
func (e *Engine) doorAt(gm *mapdata.GameMap, mapName string, x, y int) (mapdata.TeleportDest, bool) {
	t := e.teleports.Load()
	if t == nil {
		return mapdata.TeleportDest{}, false
	}
	return t.DoorAt(gm, mapName, x, y)
}

// reloadMapConfigs reads maps/*.txt. A file with an error keeps the version
// loaded before (the built-in one at startup) and the error is reported. live:
// the server is running, so the doors and spawns are rebuilt and the admins told.
func (e *Engine) reloadMapConfigs(live bool) {
	dir := e.mapEdits.dir
	stamp := mapdata.ConfigStamp(dir)
	var prev mapdata.MapConfigs
	if live {
		prev = mapdata.Configs()
	}
	cs, errs := mapdata.LoadMapConfigs(dir, prev)
	mapdata.SetConfigs(cs)
	e.mapEdits.loaded[mapConfigsKey] = stamp
	for _, err := range errs {
		log.Printf("WARNING: map configs: %v (keeping that map's previous config)", err)
		if live {
			e.tellAdmins("Map configs: error, not loaded: "+err.Error(), 2)
		}
	}
	if !live {
		table := mapdata.TeleportTableFrom(cs)
		e.teleports.Store(&table)
		if dir != "" && stamp != "" {
			log.Printf("Map configs: %s from %s", plural(len(cs), "map"), filepath.Join(dir, mapdata.MapConfigDir))
		}
		return
	}
	table := mapdata.TeleportTableFrom(cs)
	e.teleports.Store(&table)
	summary := "doors rebuilt"
	if e.mapEdits.baseSpawns != nil && e.mapEdits.baseFromConfigs {
		e.mapEdits.baseSpawns = DefaultSpawnConfig()
		summary += "; " + e.applySpawns()
	}
	log.Printf("Map configs: reloaded: %s", summary)
	e.tellAdmins("Map configs: reloaded: "+summary, 1)
}

// applySpawns respawns every map whose spawns changed.
func (e *Engine) applySpawns() string {
	cfg := e.mapEdits.baseSpawns
	sig := spawnSignatures(cfg)
	var changed []string
	for _, gm := range e.maps {
		lower := strings.ToLower(gm.Name)
		if sig[lower] == e.mapEdits.spawnSig[lower] {
			continue
		}
		removed, created := e.respawnMap(gm, spawnsOn(cfg, gm.Name))
		changed = append(changed, fmt.Sprintf("%s (%d NPCs out, %d in)", gm.Name, removed, created))
	}
	e.mapEdits.spawnSig = sig
	if len(changed) == 0 {
		return "no spawn changes"
	}
	return "respawned " + strings.Join(changed, ", ")
}

// respawnMap removes a map's spawn-list NPCs (not summons or pets) and spawns
// entries in their place, telling the players nearby.
func (e *Engine) respawnMap(gm *mapdata.GameMap, entries []SpawnEntry) (removed, created int) {
	e.npcs.Range(func(key, v any) bool {
		n, ok := v.(*npc.NPC)
		if !ok || !n.FromSpawnList || !strings.EqualFold(n.MapName, gm.Name) {
			return true
		}
		if n.State != npc.StateDead {
			gm.ClearOwner(n.X, n.Y)
			e.broadcastAroundNPC(gm, n.X, n.Y, -1, network.MsgNpcDisappear, &pb.NpcDisappear{ObjectId: n.ObjectID})
		}
		e.npcs.Delete(key)
		removed++
		return true
	})
	for i := range entries {
		created += e.spawnEntry(&entries[i], true)
	}
	return removed, created
}

// sendCollisionUpdate sends a map's new collision grid to the players on it.
func (e *Engine) sendCollisionUpdate(gm *mapdata.GameMap) {
	data, err := network.Encode(network.MsgMapCollisionUpdate, &pb.MapInfo{
		Name:          gm.Name,
		Width:         int32(gm.Width),
		Height:        int32(gm.Height),
		CollisionGrid: gm.PackCollisionGrid(),
	})
	if err != nil {
		return
	}
	e.players.Range(func(_, v any) bool {
		if p, ok := v.(*player.Player); ok && strings.EqualFold(p.MapName, gm.Name) {
			p.Send(data)
		}
		return true
	})
}

// tellAdmins sends a notice to every admin online (msgType as sendNotification).
func (e *Engine) tellAdmins(msg string, msgType int32) {
	e.players.Range(func(_, v any) bool {
		if p, ok := v.(*player.Player); ok && p.AdminLevel > 0 {
			e.sendNotification(p, msg, msgType)
		}
		return true
	})
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
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
