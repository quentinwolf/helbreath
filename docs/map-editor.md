# Map editor (doors and teleports)

A browser tool for seeing and fixing the doors on every map (building entrances
and exits, town gates and dungeon stairs), which tiles you can walk on, and where
monsters and town NPCs spawn. It draws
each map the way the game does, marks every door tile and arrival spot, and lists
problems such as doors you can't step on or doors that never fire.

## Running it

You need Node (for the client) and Go (the editor uses a Go tool to read the
server's built-in doors). The game server does **not** need to be running.

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
| Start a map over | **Revert doors** goes back to the built-in doors, **Reset this map's walkability** to the map file, and **Revert spawns** to the built-in spawns. Each takes effect when you save. |

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

Moving a building entrance usually means three steps. Open the tiles in front of the
door (**Walkable**), put the door's tiles there (**Edit tiles**), then block anything
that should stay solid. The Problems list updates as you paint. For example, it warns
when a door tile or an arrival spot ends up blocked.

The status bar shows the tile under the mouse: coordinates, walkable or blocked,
flag, door, and arrivals.

## How doors work on the server

Built-in doors come from the original server's map files (`teleports_gen.go`, from
`teleport-loc` lines) plus hand-typed ones (`teleport.go`). On those maps, a tile works
as a door only if **both** of these are true:

1. the server's `.amd` file has the teleport flag on that tile, and
2. there is a door entry on that tile or within 2 tiles of it (the nearest one wins).

Entries on tiles without the flag never fire. When the editor first ran, this was why
the Cathedral and Command Hall entrances in Aresden and Elvine didn't work at all, and
why the warehouses worked on only some of their tiles. The **Problems** list reports it,
and **Make them work** turns those entries into working doors.

Once you change a map in the editor, that map's doors are saved to
`server/internal/mapdata/teleports.json`. From then on the server uses **only that
file** for that map: a door works on exactly the tiles listed, and the `.amd` teleport
flags and the 2-tile fallback are ignored. Maps not in the file keep their built-in
doors, so the Go tables stay as the fallback.

### Live reload

The game server reads `teleports.json`, `walkable.json` and `spawns.json` from
`server/internal/mapdata/` (relative to where it runs, normally the repo root). It
checks them once a second and reloads a file as soon as it changes, so a save in the
editor takes effect in the running game:

- The log shows `Map edits: reloaded walkable.json: walkability changed on elvine (12 tiles)`,
  and admins in game get the same notice.
- Doors are swapped in at once. For walkability, the players on a changed map get its
  new collision grid, so their client's click-to-walk matches straight away. Nobody is
  moved and the scene doesn't reload.
- A broken file is reported in the log and to admins in game. The server keeps what it
  had loaded and tries again on the next save.

Use `-mapedits <folder>` (or `MAP_EDITS_DIR`) to read the files from somewhere else. Both
files are also built into the server binary (`go:embed`). If the folder doesn't exist,
as on a deployed server without the source tree, the server uses those built-in copies
and doesn't reload. `go test ./server/internal/mapdata/` checks the built-in copies parse.

### File format

```json
{
  "version": 1,
  "maps": {
    "elvine": [
      {"name": "Warehouse", "to": {"map": "wrhus_2", "x": 56, "y": 36, "dir": 4}, "tiles": [[197,127],[198,128],[199,129]]}
    ]
  }
}
```

- Map names use the server's spelling (the `.amd` file name, e.g. `Cmdhall_2`).
- `dir` is the way you face on arrival: 1 = north, then clockwise to 8 = north-west.
  0 (or leaving it out) keeps your current facing.
- A tile can belong to only one door. `name` is optional and only used by the editor.
- `x`/`y` of -1 means the middle of the destination map. If the arrival tile is
  blocked, the server moves you to the nearest open tile.

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

## Spawns (spawns.json)

The **Spawns & NPCs** layer shows every spawn on the map with the NPC drawn as the game
draws it, facing its set direction (or south-east by default):

- **Spawn areas** (filled rectangles): the NPCs spawn, respawn and wander anywhere inside.
  Most monster spawns from the original map files are areas.
- **Spots with a spread** (outlined square): the NPCs spawn within ±N tiles of the spot and
  wander around it.
- **Single spots**: town NPCs (shopkeepers, officers) and other fixed NPCs.

Colours: blue for town NPCs, green for guards, purple for other standing NPCs, and orange
for monsters. The Problems list warns about blocked spots, areas with too few open tiles,
and NPCs standing on a door tile.

Like doors, editing a map's spawns saves **all** of that map's spawns to
`server/internal/mapdata/spawns.json`. That list then replaces the map's built-in spawns
(the original spawn rectangles in `spawns_gen.go` plus `handWrittenSpawns` in
`spawn_config.go`). Other maps keep the built-in list.

```json
{
  "version": 1,
  "maps": {
    "gshop_2": [
      {"npc": 15, "x": 62, "y": 42, "dir": 4}
    ],
    "elvine": [
      {"npc": 10, "count": 20, "x": 156, "y": 36, "area": [147, 30, 166, 42], "note": "Slimes"},
      {"npc": 1014, "count": 6, "x": 150, "y": 150, "radius": 30, "respawn": 60}
    ]
  }
}
```

`npc` is the NPC type (as in NPC.cfg). `count` defaults to 1. `area` is x1, y1, x2, y2.
`dir` is the facing on spawn (1 = north, clockwise to 8). `respawn` is in seconds (0 or
missing uses the NPC type's delay). `note` is only for you.

When the file changes, the running server **replaces every spawn-list NPC on each changed
map**: they vanish and the new list spawns at once, so monsters there start fresh at full
health. Maps whose spawns didn't change are left alone. Summoned creatures are never
touched. The log and admins in game see `respawned gshop_2 (1 NPCs out, 1 in)`.

## Good to know

- **Half-tile drawing offset.** The original client draws each tile's ground centred on the
  tile's point, and buildings and characters stand on that point (`DrawBackground`,
  `ix - 16, iy - 16`). Our client draws the ground from the point instead, so in game,
  buildings and characters sit on the top-left corner of their tile rather than in the
  middle. The editor draws buildings and NPCs half a tile further on, in the middle of their
  tile, so a highlighted tile is where a character standing on it appears.
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
  and handles input; `doors.ts` has the door grouping, file format and problem checks).
- `client/vite-plugins/mapEditorApi.ts`: the dev-server API (reads the maps, runs the Go
  export, writes `teleports.json` and `walkable.json`). After changing this file, restart
  `npm run dev` (or save `vite.config.ts`) so Vite reloads it.
- `server/tools/teleportexport/`: prints the built-in doors as the server resolves them.
- `server/internal/mapdata/teleport_overrides.go`: loads `teleports.json`, and `DoorAt`,
  which the engine uses when a player steps on a tile.
- `server/internal/mapdata/walk_overrides.go`: loads `walkable.json` (`Reapply`).
  `edit_files.go` reads both files from disk or the built-in copies.
- `client/src/dev/mapeditor/spawns.ts`: spawn file format and spawn checks.
  `client/src/constants/NpcSprites.ts`: NPC type → sprite file, shared by the game and the editor.
- `server/internal/game/spawn_overrides.go`: loads `spawns.json` and applies it to the spawn
  list. The engine spawns from it in `SpawnNPCs`, and `respawnMap` (in `map_edits.go`)
  swaps a map's NPCs when it changes.
- `server/internal/game/map_edits.go`: the once-a-second reload, and the collision-grid
  update (message `0xAE`, a `MapInfo`) that the client handles in `GameScene.onCollisionUpdate`.
