# Status: what's ported, what's missing, what's changed on purpose

The single list of where the rewrite stands against the original game. Update it in the
same commit as the change. `PLAN.md` is the original author's spec from the start of the
project and is kept as it was; `docs/server/phase*.md` are his gap checklists (some of
their open items have since been done; this file wins where they disagree).

**Ground truth** for "how the original did it":
- Server: HGServer 2.24b + HB Server Files 3.51 (`R:\Helbreath\HelbreathSserver (800x600)\HelbreathServer-main`:
  `HGServer/Game.cpp`, `Files/GameServers/*/MAPDATA/*.txt`). The closest to the original game.
- Equilibrium v3.82 (`R:\Helbreath\play-helbreath\helbreath-equilibrium-master`): a customised
  server; the item, NPC and magic `.cfg` files came from it. Use it as a second reference.
- Original client: `Client.cpp` (community 3.82 client).

Legend: **Done** = works like the original. **Partial** = works, with listed gaps.
**Missing** = not started. **Changed** = deliberately different from the original (the reason is noted).

## World and maps

| System | State | Notes |
|---|---|---|
| Map configs | Done | One `server/internal/mapdata/maps/<map>.txt` per map in the original MAPDATA syntax (monster names instead of mob codes), live reload. See [map-config.md](map-config.md) for which settings are used |
| Map loading (.amd), collision | Done | Map editor can patch walkability (`walkable.json`, live reload) |
| Doors / teleports | Done | `teleport-loc` from the map configs; maps saved by the editor have `exact-doors = 1` (exact tiles). Missing destination maps: arefarm, elvfarm, gldhall_1/2, inferniaB (stand-in exits on default and 2ndmiddle). Doors to `-1 -1` use the map centre, not `initial-point` |
| Monster spawns (`spot-mob-generator`) | Done | From the map configs (type 1 rectangles; `dir=` / `respawn=` options). Type 2 (waypoint routes) not ported |
| Town NPCs (`npc` lines) | Partial | Stand on their first waypoint; walking between waypoints not ported |
| Random mob generator | Done | `random-mob-generator = 1 <level>`: every 600 ms a map below `maximum-object - 30` monsters gets a leader rolled from the original level table (`random_mobs_table.go`, ported from HGServer) plus 0-4 of the same, anywhere walkable outside `npc-avoidrect`; removed on death, the generator refills. `RandomMobDensity` (difficulty.go) scales the cap. Not ported: special abilities, berserk (level 21 Unicorns), followers following their leader, crusade/special-event spawns. Maps nobody is on don't run NPC AI (performance) |
| `maximum-object` | Done | Caps spot generators and the random mob generator |
| No-attack areas | Done | As HGServer: no attack spells cast from inside; players inside take no melee or spell damage (players or monsters) and monsters don't target them. `-10` rectangle = whole map (town buildings). Not ported: debuffs (Hold, Confuse...) landing on players inside |
| Map rules (`initial-point`, `level-limit`, `pk-mode`, `limited-cast`, `drop-type`, `RecallImpossible`, `CitizenLimit`) | Missing | In the map configs, not read yet. Safe zones are a hard-coded list (`maptype.go`) |
| Waypoints, `npc-avoidrect` | Missing | |
| Fish points, mineral points | Missing | Fishing/Mining skills succeed anywhere and give a fixed item |
| Day/night | Done | Indoor maps (`fixed-dayornight-mode`) always day |
| Weather | Partial | Rain is the original's (effect2 drops + splashes). Fog is the port's own. One global roll (the original rolled per map). Rain splashes on HUD not applicable |
| Snow | Changed | Original snowflakes (effect2 frames 39-50). Random snow only in **December and January** (rain the rest of the year); Icebound snows all year like the original. `/weather snow` works any time |
| Map editor | Done | `client/map-editor.html`, `docs/map-editor.md`: doors, walkability, spot mob generators and town NPCs, saved into the map configs. Shows no-attack / no-spawn areas, waypoints, fish, mineral, start and strike points (not editable yet) |

## Characters and combat

| System | State | Notes |
|---|---|---|
| Combat formulas (dice, STR, layered armour, criticals) | Done | Port's easy-mode knobs still in `difficulty.go` (DamageMultiplier 4, HitBonus 50, NPCDamageReduction 3, DropRateMultiplier 2) |
| Experience and levels | Done | Original level table and kill exp; XPMultiplier 1. The /4 below level 80 in aresdend1/elvined1 not ported |
| Death | Done | Body stays, Restart button, Resurrection |
| Citizenship | Partial | William (City Hall). Rest of City Hall menu (rewards, hero items, quests, crusade role) missing |
| PvP sides | Changed | Traveller↔traveller can't hurt each other; heals allowed except Aresden↔Elvine |
| Level-up cheer | Done | |
| Gender | Bug | Client uses 1 = female, server `giveStartingItems` treats 1 as male |

## Magic

| System | State | Notes |
|---|---|---|
| Spell data | Done | `magicimport` from Equilibrium Magic.cfg; tile-based casting |
| Spell visuals | Done | Most from ErkoKnoll (MIT), several straight from Client.cpp. Heal/buff effects are single-sheet guesses; Armor Break should be subtractive |
| Spell hit timing | Changed | The server applies spell damage at once (as the original); the client holds a cast's hits, deaths and loot until its effect lands (meteor ~450 ms, bolts/fireballs by distance; `client/src/game/spells/SpellTiming.ts`). The original client got the same look by sending the cast after the casting motion |
| Field spells | Done | Fire/Poison/Ice/Spike fields last 30 s. Fire spreading, weather shortening fire, NPC poison/freeze from fields not ported |
| Debuffs (Hold, Paralyze, Stamina Drain, Possession, Confuse family) | Done | Illusion Movement doesn't reverse attack direction; no-attack-area checks missing |
| Recall | Done | |
| Summon Creature | Changed | By caster level (Zac's design), not the original Magery roll |
| Invisibility | Changed | Anyone may cast (original: citizens only) |
| Confuse Language | Changed | The confused reader sees others' chat scrambled |
| Learn spells at Gandalf | Changed | Pay the Magic.cfg price near Gandalf; GMs learn anywhere |
| Haste | Missing | Not in our Magic.cfg. The 3.82 client draws a hasted player (status 0x40000) with 5 faded copies trailing behind (Client.cpp ~15945); the port had a fake one-copy trail on every step, now removed |
| Peace (16) | Missing | No known behaviour in the original server |
| Knockback, magic resistance on all spells, Protection From Arrow | Missing | |

## NPCs

| System | State | Notes |
|---|---|---|
| NPC data | Done | `npcimport` from NPC.cfg, plain names |
| Movement | Done | Original step timing, stationary shopkeepers, per-NPC turn side |
| AI, guards, faction targeting | Partial | See `docs/server/phase2-monster-ai.md` for open items |
| Sounds | Done | |
| Human-looking NPC types 1-6 | Missing | Need player-style rendering |
| Sprites | Partial | Gate.pak, Energy-Sphere.apk not converted; AirEl/IceEl/Willowisp have no sprites |
| Warehouse (item storage) | Missing | Howard is still a potion shop |

## Items

| System | State | Notes |
|---|---|---|
| Item data | Done | Item.cfg IDs are authoritative |
| Ground / inventory sprites | Done | From Item.cfg; 48 Equilibrium frames missing from our atlases (fallbacks used) |
| Drop on death, protected slots | Missing | See `docs/server/phase4-death-loot-economy.md` |

## Events and endgame

| System | State | Notes |
|---|---|---|
| Crusade (strike points, strategic points, structures) | Missing | |
| Energy Sphere (Middleland) | Missing | |
| Heldenian, Apocalypse, dynamic gates | Missing | |
| Item events (`item-event`) | Missing | |
| Guilds, parties | Partial | See `docs/server/phase5-endgame-pvp.md` |

## Technical

| Item | State | Notes |
|---|---|---|
| Concurrency | Bug | Per-client goroutines change shared state without an engine lock; StatusEffect map writes can crash under load |
| God mode persistence | Partial | JSON save file only, no Postgres column |
| Protobuf | Note | No protoc on the dev machine: new messages reuse existing protos |

## Planned next (map config unification)

1. ~~One plain-text config per map~~ Done: [map-config.md](map-config.md).
2. ~~Map editor reads/writes those files~~ Done (doors, spawns; other settings shown). Next for it:
   editing no-attack areas and fish / mineral / start points.
3. ~~No-attack areas, random mob generator~~ Done. Still to do: map rules (`initial-point` for
   `-1 -1` doors and respawn, level limits, recall, safe zones from the files).
4. **TODO: fishing and mining** (researched, not started), from HGServer:
   - Fish: `FishGenerator` puts fish (dynamic objects) near random `fish-point`s, up to `max-fish`,
     1 in 10 per map per run: RedCarp, GreenCarp, GoldCarp, CrucianCarp, BlueSeaBream, RedSeaBream,
     Salmon, GrayMullet, rarely a PowerGreenPotion etc., each with a difficulty and a lifetime
     (`iCreateFish`, `bDeleteFish`). Casting a rod at one (`iCheckFish`, `FishProcessor`,
     `ReqGetFishThisTimeHandler`, Game.cpp ~33486-33960) rolls Fishing skill vs difficulty; the
     client shows a catch-chance dialog. Needs fish objects + dialog in the client.
   - Mining: `MineralGenerator` (Game.cpp ~35918) puts mineral rocks on random `mineral-point`s
     (1 in 4 per run, up to `max-mineral`, `mineral-generator` level); digging with a pickaxe
     (`_CheckMiningAction` ~36001) gives ores/gems by level until the rock is used up
     (`bDeleteMineral`). Needs rock objects in the client.
   - Today Fishing/Mining skills succeed anywhere and give a fixed item: replace that. Check the
     fish/ore items exist in Item.cfg data and their sprites.
5. Crusade, Energy Sphere, Heldenian, Apocalypse.
