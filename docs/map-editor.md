# Map editor

A browser tool for seeing and fixing the doors on every map (building entrances
and exits, town gates and dungeon stairs), which tiles you can walk on, and the
spot mob generators and town NPCs. It saves doors and spawns into each map's
config file (`server/internal/mapdata/maps/<map>.txt`, see [map-config.md](map-config.md)),
and also shows the config's other settings: no-attack areas, waypoints, fish,
mineral, start and strike points. It draws
each map the way the game does, marks every door tile and arrival spot, and lists
problems such as doors you can't step on or doors that never fire.

## Running it

You need Node (for the client) and Go (the editor uses Go tools to read and write
the map configs exactly as the server does). The game server does **not** need to be running.

```bash
cd client
npm install        # first time only
npm run dev
```

Then open **http://localhost:3000/map-editor.html**. The first load takes a few
seconds: it prepares the map sprites and runs `go run ./server/tools/teleportexport`.

- `?map=elvine&x=200&y=128` in the address opens a map at a tile. The address
  updates as you move between maps, so a refresh keeps your place.
- Go is found on `PATH`, or at `C:\Program Files\Go\bin\go.exe` on Windows.
  Set `GO_BIN` to use another one.

The editor is dev-only: its page and API exist only under `npm run dev`. They are
not part of `npm run build`.

## Using it

| To do this | Do this |
| --- | --- |
| Move around | Drag with any mouse button, or use the arrow keys. Use the wheel to zoom. |
| Select a door | Click it on the map, or in **Doors on this map**. |
| Keep the panel tidy | The map picker, layers and undo stay at the top, and **Save** stays at the bottom. The sections in between (Walkability, Doors, Spawns & NPCs, Problems, Controls) fold away when you click their titles. The editor remembers which are open and unfolds one when you select something in it. |
| Change where it leads | Pick the map and enter x/y in **Selected door**, or click **Pick arrival**. This opens the destination map; click the tile where players should arrive. |
| Change which tiles you step on | **Edit tiles**, then click or drag on the map. Dragging from an empty tile adds tiles, dragging from a door tile removes them. Right-drag still pans. Press Esc when done. |
| Add a door | **+ New door**, paint its tiles, then set where it leads. |
| See the other side | **Go to arrival** follows the door. **Back** returns. Double-click an entry in **Doors that lead here** to jump to that door. |
| Change which tiles are blocked | **Edit walkability**, choose **Block**, **Walkable** or **Restore** (back to the map file), and paint. Shift+drag paints a rectangle. Choose a brush size from 1×1 to 5×5. In this mode, pan with right-drag. Keys: B / O / R pick the paint, `[` `]` change the brush, Esc finishes. |
| Move an NPC or monster spawn | **Edit spawns**, then drag the NPC (or its spawn area) on the map. Click one to edit it in the panel: NPC type, count, respawn time, facing, spot with spread (±tiles) or a spawn area. **Draw area** drags out a new area. **+ New spawn** places one where you click, **Duplicate** copies the selected one, and **Delete** (or the Delete key) removes it. |
| Undo | **Undo** / **Redo**, or Ctrl+Z / Ctrl+Y (Ctrl+Shift+Z). One history covers every map and every kind of edit, so Ctrl+Z after **Pick arrival** on another map undoes the pick. When the next undo is on another map, the note beside the buttons says which. |
| Save | **Save** (or Ctrl+S). A running game server picks the changes up within a second, so no restart is needed (see below). |
| Throw away changes | **Discard door changes** / **Discard spawn changes** go back to what the map's config file says (undo can bring the changes back). **Reset this map's walkability** goes back to the `.amd` and takes effect when you save. Older versions of a config are in git. |

What's drawn on the map:

- **Coloured tiles**: doors, one colour per door. The selected door has a white outline.
- **Red cross**: a door tile that's blocked. Nobody can step on it, so it never fires.
- **Magenta box with a slash**: a built-in door entry on a tile without the teleport flag.
  It never fires (see below).
- **Circles**: arrival spots of doors leading to this map. The arrow shows which way you
  face, and the label says which map the door is on.
- Layers (top of the panel): **Blocked tiles** shades everything the server won't let
  you walk on: red as in the map file, **orange** for tiles you blocked, **green** for
  tiles you opened. **Teleport flags** shows the `.amd` teleport bit.
  **No-attack & no-spawn areas** draws the config's `no-attack-area` rectangles (blue) and
  `npc-avoidrect` (yellow outline: no random monsters). **Waypoints, fish, mineral & start
  points** shows `waypoint`, `fish-point`, `mineral-point`, `strike-point` and `initial-point`
  with their numbers. These two layers are view-only for now; edit them in the `.txt`.
- The line under the map picker names the map's config file and sums up its settings
  (side, monster cap, random mob generator, indoors, level limits, how many points).

Moving a building entrance usually means three steps. Open the tiles in front of the
door (**Walkable**), put the door's tiles there (**Edit tiles**), then block anything
that should stay solid. The Problems list updates as you paint. For example, it warns
when a door tile or an arrival spot ends up blocked.

The status bar shows the tile under the mouse: coordinates, walkable or blocked,
flag, door, and arrivals.

## How doors work on the server

Doors are the `teleport-loc` lines of each map's config. A map works one of two ways:

- **By teleport flag** (most maps, as imported from the original files): a tile works as a
  door only if the server's `.amd` file has the teleport flag on it **and** there is a door
  entry on that tile or within 2 tiles of it (the nearest one wins). Entries on tiles
  without the flag never fire. When the editor first ran, this was why the Cathedral and
  Command Hall entrances in Aresden and Elvine didn't work at all, and why the warehouses
  worked on only some of their tiles. The **Problems** list reports it, and **Make them
  work** turns those entries into working doors.
- **Exact doors** (`exact-doors = 1`): a door works on exactly the tiles listed; the flags
  and the 2-tile fallback are ignored. Saving a map's doors in the editor writes every door
  tile as a `teleport-loc` line and sets `exact-doors = 1`, so what you see is what works.

Lines whose destination map has no config (the original's doors to maps the port doesn't
have yet, such as the farms) are kept in the file untouched and ignored by the server.

### Saving and live reload

**Save** writes only the maps you changed, and in each file only the lines that differ:

- door tiles are matched by tile, so unchanged tiles keep their lines (and comments);
  new tiles go after the other lines of the same door;
- spot mob generators keep their number; a new one gets the next free number;
- town NPCs keep their `npc` line; when one moves, its waypoints move with it (or it gets a
  new waypoint if it shared them with another NPC);
- a door's name and a spawn's note are written as the line's `//` comment.

The Go tool `server/tools/mapconfigsave` does the writing and checks every changed file the
way the server reads it; if one has a problem nothing is written and the editor says why.

The game server checks the map configs and `walkable.json` once a second and reloads what
changed, so a save takes effect in the running game:

- The log shows `Map configs: reloaded: doors rebuilt; respawned gshop_2 (1 NPCs out, 1 in)`
  or `Map edits: reloaded walkable.json: walkability changed on elvine (12 tiles)`, and admins
  in game get the same notice.
- Doors are swapped in at once. For walkability, the players on a changed map get its new
  collision grid, so their client's click-to-walk matches straight away. Nobody is moved
  and the scene doesn't reload.
- A broken file (also one you edited by hand) is reported in the log and to admins in game,
  and the server keeps that map's previous version. The editor shows the same error in a
  banner when it loads.

Use `-mapedits <folder>` (or `MAP_EDITS_DIR`) to read the files from somewhere else. They
are also built into the server binary (`go:embed`). If the folder doesn't exist, as on a
deployed server without the source tree, the server uses those built-in copies and doesn't
reload. `go test ./server/internal/mapdata/` checks the built-in copies load.

## Walkability (walkable.json)

Walkability edits go to `server/internal/mapdata/walkable.json`. Like the doors file, it
reloads live, and it only lists the tiles that differ from the map file:

```json
{
  "version": 1,
  "maps": {
    "elvine": {
      "open": [[203,124],[204,124]],
      "block": [[210,133]]
    }
  }
}
```

The server applies it on top of each map's `.amd` walkability at startup and on every
reload. Taking a tile out of the file puts it back as the `.amd` has it. Everything that
checks walkability sees the change: player movement, NPCs, spells, and the collision
grid sent to the client. The `.amd` files themselves are never modified.

## Spot mob generators and town NPCs

The **Spot mob generators & NPCs** layer shows every spawn on the map with the NPC drawn as
the game draws it, facing its set direction (or south-east by default):

- **Spawn areas** (filled rectangles): a `spot-mob-generator`. Up to its count are alive at
  once, spawning, respawning and wandering anywhere inside.
- **Spots with a spread** (outlined square): saved as an area of ±N tiles around the spot.
- **Single spots**: a town NPC (shopkeeper, officer) or any single NPC, saved as an `npc` line
  standing on a waypoint. A group on one tile is saved as a one-tile area.

Colours: blue for town NPCs, green for guards, purple for other standing NPCs, and orange
for monsters. The Problems list warns about blocked spots, areas with too few open tiles,
NPCs standing on a door tile, and NPC types that can't be saved (types that share their name
with others, such as the soldiers: pick a specific one).

Facing and a respawn time other than the NPC type's are saved as `dir=N` and `respawn=N` at
the end of the line, the note as its comment:

```
spot-mob-generator = 3   1   100 80 109 87   Slime 8   // starter slimes
npc               =   ShopKeeper-W       2   1 2 3 4 5 6 7 8 9 10   1 dir=4
```

When a map's spawns change, the running server **replaces every spawn-list NPC on that map**:
they vanish and the new list spawns at once, so monsters there start fresh at full health.
Maps whose spawns didn't change are left alone. Summoned creatures are never touched.

## Good to know

- **Tiles are centred on their point.** As in the original client (`DrawBackground`,
  `ix - 16, iy - 16`), each tile's ground is drawn centred on the tile's point, the spot
  where characters, buildings and effects stand. The game does this in
  `HBMap.renderMapTiles` and turns mouse positions into tiles with `worldToTile` in
  `GameScene`. The editor shows the same picture shifted by half a tile, so its cells run
  from x*32 to x*32+32.
- **Two copies of each map.** The picture comes from the client's copy
  (`client/public/assets/maps`). Walkability, flags and problems come from the server's
  copy (`assets/MAPDATA`), which is what the game uses for movement. About 20 maps differ
  between the two, mostly in decorations and blocked tiles. On a few (e.g. `elvined1`,
  `fightzone6`) even the size differs, and the editor says when the picture may not line
  up. Maps the client doesn't have show no picture.
- The editor doesn't change `.amd` files. Walkability edits live in `walkable.json`, and
  teleport flags and decorations stay as they are. The client still draws its own copy
  of the map, so a building stays drawn where you made its tiles walkable.

## Where the code is

- `client/map-editor.html`, `client/src/dev/mapeditor/`: the page (`MapEditor.ts` draws
  and handles input; `doors.ts` has the door grouping and problem checks; `spawns.ts` the
  spawn working copies and checks).
- `client/vite-plugins/mapEditorApi.ts`: the dev-server API: runs the Go export, hands
  saves to `mapconfigsave`, writes `walkable.json`. After changing this file, restart
  `npm run dev` (or save `vite.config.ts`) so Vite reloads it.
- `server/tools/teleportexport/`: prints every map's doors (as the server resolves them),
  spawns and config settings for the editor.
- `server/tools/mapconfigsave/` and `server/internal/mapdata/mapconfig_edit.go`: write the
  editor's doors and spawns into the map configs, changing only the lines that differ.
- `server/internal/mapdata/`: `mapconfig.go` (parser), `mapconfigs.go` (loader),
  `door_table.go` (`DoorAt`, which the engine uses when a player steps on a tile),
  `walk_overrides.go` (`walkable.json`, `Reapply`).
- `client/src/constants/NpcSprites.ts`: NPC type → sprite file, shared by the game and the editor.
- `server/internal/game/spawn_config.go`: spawns from the map configs. The engine spawns them
  in `SpawnNPCs`, and `respawnMap` (in `map_edits.go`) swaps a map's NPCs when they change.
- `server/internal/game/map_edits.go`: the once-a-second reload, and the collision-grid
  update (message `0xAE`, a `MapInfo`) that the client handles in `GameScene.onCollisionUpdate`.
