package game

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// The map editor's files (teleports.json, walkable.json, spawns.json; docs/map-editor.md) are
// read from a folder on disk and reloaded within a second of being saved, so door
// and walkability edits take effect without restarting the server.

// mapEditsWatch remembers which version of each file is loaded.
type mapEditsWatch struct {
	dir       string
	lastCheck time.Time
	// per file: the stamp now loaded, and the last stamp that failed to load
	loaded, failed map[string]string
	// baseSpawns: the spawn list before spawns.json (nil until SpawnNPCs).
	// spawnSig: per map (lower case), its spawns.json entries as loaded ("" = none).
	baseSpawns *SpawnConfig
	spawnSig   map[string]string
}

// SetMapEditsDir sets the folder holding teleports.json and walkable.json. With
// "" (or a folder that doesn't exist) the copies built into the server are used
// and nothing is reloaded. Call before LoadMaps.
func (e *Engine) SetMapEditsDir(dir string) {
	if dir != "" {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			log.Printf("Map edits: folder %q not found, using the built-in teleports.json and walkable.json", dir)
			dir = ""
		}
	}
	e.mapEdits.dir = dir
}

// loadMapEdits reads both files once the maps are loaded.
func (e *Engine) loadMapEdits() {
	e.initMapEdits()
	if e.mapEdits.dir != "" {
		log.Printf("Map edits: reading %s, %s and %s from %s (reloaded when they change)",
			mapdata.TeleportsFile, mapdata.WalkableFile, mapdata.SpawnsFile, e.mapEdits.dir)
	}
	e.reloadMapEdit(mapdata.TeleportsFile, false)
	e.reloadMapEdit(mapdata.WalkableFile, false)
}

func (e *Engine) initMapEdits() {
	if e.mapEdits.loaded == nil {
		e.mapEdits.loaded = map[string]string{}
		e.mapEdits.failed = map[string]string{}
		e.mapEdits.spawnSig = map[string]string{}
	}
}

// loadSpawnEdits is base with spawns.json applied (SpawnNPCs, at startup).
func (e *Engine) loadSpawnEdits(base *SpawnConfig) *SpawnConfig {
	e.initMapEdits()
	stamp := mapdata.EditFileStamp(e.mapEdits.dir, mapdata.SpawnsFile)
	o, err := LoadSpawnOverridesFrom(e.mapEdits.dir)
	if err != nil {
		e.mapEdits.failed[mapdata.SpawnsFile] = stamp
		log.Printf("WARNING: map edits: %v (using the built-in spawns)", err)
		return base
	}
	e.mapEdits.loaded[mapdata.SpawnsFile] = stamp
	e.mapEdits.spawnSig = spawnSignatures(o)
	if len(o.Maps) > 0 {
		log.Printf("Map edits: %s: %s with edited spawns", mapdata.SpawnsFile, plural(len(o.Maps), "map"))
	}
	return o.Apply(base)
}

// spawnSignatures is each listed map's entries as JSON, to see which maps changed.
func spawnSignatures(o SpawnOverrides) map[string]string {
	sig := map[string]string{}
	for m, spots := range o.Maps {
		b, _ := json.Marshal(spots)
		sig[strings.ToLower(m)] = string(b)
	}
	return sig
}

// checkMapEdits reloads a file whose stamp changed since it was loaded.
func (e *Engine) checkMapEdits() {
	if e.mapEdits.dir == "" || e.mapEdits.loaded == nil {
		return
	}
	files := []string{mapdata.TeleportsFile, mapdata.WalkableFile}
	if e.mapEdits.baseSpawns != nil {
		files = append(files, mapdata.SpawnsFile)
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
	case mapdata.TeleportsFile:
		var table mapdata.TeleportTable
		table, err = mapdata.BuildTeleportTableFrom(dir)
		if err == nil {
			e.teleports.Store(&table)
			summary = plural(len(table.Exact), "map") + " with edited doors"
		}
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
	case mapdata.SpawnsFile:
		var o SpawnOverrides
		o, err = LoadSpawnOverridesFrom(dir)
		if err == nil && e.mapEdits.baseSpawns != nil {
			summary = e.applySpawnEdits(o)
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
	} else if summary != "no walkability changes" && summary != "0 maps with edited doors" && summary != "" {
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

// applySpawnEdits respawns every map whose spawns.json entries changed.
func (e *Engine) applySpawnEdits(o SpawnOverrides) string {
	sig := spawnSignatures(o)
	cfg := o.Apply(e.mapEdits.baseSpawns)
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
			e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgNpcDisappear, &pb.NpcDisappear{ObjectId: n.ObjectID})
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
