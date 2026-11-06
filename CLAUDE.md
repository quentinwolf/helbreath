# CLAUDE.md: HB Online (Helbreath web rewrite)

Web rewrite of the classic Helbreath MMORPG: Go server (`server/`, WebSocket + protobuf,
PostgreSQL or in-memory) and Phaser 3 + TypeScript client (`client/`, Vite).
Zac's fork of `juanrossi/helbreath`; the aim is to play like the original game.

**Start here:** [docs/STATUS.md](docs/STATUS.md) lists what's ported, what's missing and what
was changed on purpose. Update it in the same commit as any change it covers.
`PLAN.md` is the original author's starting spec: read it for background, don't update it.

## Working rules

- One commit per change. Zac commits and pushes himself; don't commit unless asked. Hand over a
  ready-to-paste commit message (plain sentences, like the history).
- Check the original before guessing how something worked (see Reference). Note deliberate
  differences as **Changed** in STATUS.md.
- Keep tests green; add tests for pure logic (parsers, maths, rules).
- Generated files (`*_gen.go`, `SpellCosts.ts`) come from the tools below: regenerate, don't hand-edit.

## Reference (outside the repo, read only)

- Original server: `R:\Helbreath\HelbreathSserver (800x600)\HelbreathServer-main`:
  `HGServer/Game.cpp` and the 3.51 map files `Files/GameServers/*/MAPDATA/*.txt`. Closest to the original.
- Equilibrium v3.82 (customised): `R:\Helbreath\play-helbreath\helbreath-equilibrium-master`
  (`server/Core/*.cfg`: Item, NPC and Magic data came from here).
- Original client source, ErkoKnoll's browser client (MIT; source of most spell effects) and helper
  scripts: `D:\My Documents\VSCode Git Repositories\Claude\helbreath\` (`NOTES.md`, `reference\`, `tools\`).

## Run and check (Windows, Git Bash)

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
go run ./server/cmd/gameserver -memdb -savefile saves/dev.json   # from repo root; restart after server changes
cd client && npm run dev                                          # http://localhost:3000 (8080 = API/WS)
go build ./server/... && go test -count=1 ./server/... 2>&1 | grep -E "^(--- FAIL|FAIL|ok|panic)|_test.go:[0-9]+:"
cd client && npx tsc --noEmit
```

- `-memdb` makes every character admin level 4 (`/setadmin <name> 0` to test as a player).
- Map configs `server/internal/mapdata/maps/*.txt` (doors, spawns, town NPCs, map settings) and
  `server/internal/mapdata/walkable.json` (the editor's walkability patches) reload in the running
  server within a second. A file with an error is reported (log + admins) and the old version kept.
  The map editor saves into the same `.txt` files, changing only the lines it must.
- Map editor: http://localhost:3000/map-editor.html ([docs/map-editor.md](docs/map-editor.md)).
- Working copy is CRLF (autocrlf); `gofmt -l` flags every file, ignore that.

## Where things are

Server (`server/internal/`):
- `game/engine.go`: main loop (100 ms tick), message handling, NPC tick. `npc_combat.go`,
  `npc_movement.go`: AI. `combat.go`: damage and exp. `admin.go`: GM chat commands.
- `game/magic_handler.go` + `*_spells.go`, `fields.go`: spells. `town_npcs.go`: shop/City Hall/Gandalf clicks.
- `game/spawn_config.go`: spawns from the map configs. `map_edits.go`: live reload.
- `mapdata/`: .amd loader; **map configs** `maps/<map>.txt` (`mapconfig.go` parser, `mapconfigs.go` loader,
  [docs/map-config.md](docs/map-config.md)): doors, spawns, town NPCs, fixed-day and the other map settings.
  `mapconfig_edit.go`: the writer the map editor saves through. `teleport.go` / `door_table.go`: doors
  (exact-doors maps use exactly their tiles). `maptype.go`: safe-zone list (still hard-coded).
- `items/`, `npc/`, `magic/`: data (`*_gen.go`) and rules. `world/`: time of day, weather.
- `db/`: Postgres + in-memory store.

Client (`client/src/`):
- `scenes/GameScene.ts`: the main scene (~6000 lines). `network/MessageHandler.ts`.
- `game/spells/`: spell effects (`SpellEffects.ts` dispatcher, `OriginalFx.ts` helpers).
- `game/effects/`: weather (`OriginalRain.ts`, `OriginalSnow.ts`). `constants/`: assets, items, NPC sprites, sounds.
- `dev/mapeditor/` + `vite-plugins/mapEditorApi.ts`: map editor.

Import tools (`server/tools/`): `itemimport`, `npcimport`, `magicimport` (also `-ts` for
`SpellCosts.ts`), `mapimport` (an original MAPDATA/*.txt -> map config; for adding maps),
`teleportexport` (the map editor's view of the maps) and `mapconfigsave` (the editor's saves). Usage is in each tool's header.

## Pitfalls

- **GameScene restarts on every teleport** and Phaser reuses the instance: class fields survive,
  loader 'complete' callbacks are dropped. Reset per-map state in `create()` / on 'shutdown'.
- Tile anchor is `tile * 32` (top-left, a character's feet); pointer to tile is `floor((v + 16) / 32)`.
- NPC sprites: 40 sheets = stand 0-7, walk 8-15, attack 16-23, hit 24-31, die 32-39.
- Most original effects are additive (`PutTransSprite*`); Trans25/50/70 are alpha.
- No protoc installed: reuse existing proto messages instead of adding new ones.
- Map names: the original is inconsistent about case (`Cmdhall_1`); look maps up case-insensitively.
- After equip/stat changes call `p.RecalcCombatStats()` / `p.SyncEquipmentAppearance()`.
- Server sends time/weather as Notification type 10 (`"time:day|weather:rain"`), per player's map.
