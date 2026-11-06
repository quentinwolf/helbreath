// Dev-only map editor (client/map-editor.html, served by `npm run dev`; see docs/map-editor.md).
// Shows a map as the game draws it, with its doors, arrival spots, the server's
// walk/teleport flags, its spot mob generators and town NPCs, and the map config's
// other settings (no-attack areas, waypoints, fish/mineral points). Saves doors and
// spawns into the map's config, server/internal/mapdata/maps/<map>.txt, and
// walkability into server/internal/mapdata/walkable.json.
import Phaser from 'phaser';
import { ASSET_BASE } from '../../env';
import { TILE_SIZE, HIGH_DEPTH } from '../../Config';
import { HBMap } from '../../game/assets/HBMap';
import { HBSpriteFile, SpriteType } from '../../game/assets/HBSprite';
import { AssetType, getAssetByKey, getAssetsByType } from '../../constants/Assets';
import { NPC_SPRITE_MAP } from '../../constants/NpcSprites';
import { GameAsset } from '../../game/objects/GameAsset';
import {
    DIR_DX, DIR_DY, DIR_NAMES, Door, OverrideDoor, Problem, ServerMapInfo, TileFlags, WalkOverrides,
    applyWalkEdit, builtinDoors, builtinDoorsWithUnflagged, doorLabel, doorsToOverride,
    findProblems, isBlocked, lineTiles, newDoorId, parseAmdFlags, parseTileKey, sortDoors, tileKey,
    walkEditCount, walkEditOf,
} from './doors';
import {
    KIND_COLOUR, NpcTypeInfo, Spawn, SpawnSpot, cloneSpawns, findSpawnProblems, newSpawnId,
    spawnAnchor, spawnCount, spawnFromSpot, spawnKind, spawnLabel, spawnToSpot,
} from './spawns';

const API = '/__map-editor/';
const T = TILE_SIZE;
const DEPTH = {
    blocked: HIGH_DEPTH + 1,
    flags: HIGH_DEPTH + 2,
    rules: HIGH_DEPTH + 2.5,
    spawnAreas: HIGH_DEPTH + 3,
    doors: HIGH_DEPTH + 4,
    spawns: HIGH_DEPTH + 5,
    arrivals: HIGH_DEPTH + 6,
    hover: HIGH_DEPTH + 7,
};
// The original client (and the game, see HBMap.renderMapTiles) draws each tile's ground
// centred on the tile's point and stands buildings and characters on that point
// (DrawBackground: ix - 16, iy - 16). The editor's cells (overlays, tile numbers) run
// from x*32 to x*32+32, so it puts the ground layer at 0,0 and draws objects and NPCs
// half a tile further on, in the middle of their cell: the same picture, shifted.
const OBJECT_SHIFT = TILE_SIZE / 2;
const PALETTE = [0x4fc3f7, 0xffb74d, 0x81c784, 0xf06292, 0xba68c8, 0xfff176, 0x4db6ac, 0xff8a65, 0x9fa8da, 0xaed581];
const css = (c: number) => `#${c.toString(16).padStart(6, '0')}`;

// Blocked-tile overlay colours (RGBA): as in the .amd, newly blocked, newly walkable.
const RGBA_BLOCKED = [255, 40, 40, 90];
const RGBA_NEW_BLOCKED = [255, 140, 0, 175];
const RGBA_NEW_OPEN = [40, 230, 90, 120];
const UNDO_LIMIT = 100;

interface ExportState {
    maps: ServerMapInfo[];
    walkError?: string;
    /** Map config files the server can't read (it keeps their built-in copies). */
    configErrors?: string[];
    walkable: WalkOverrides;
    npcTypes: NpcTypeInfo[];
    clientMaps: string[];
}

interface SaveResult {
    ok: boolean;
    errors: string[];
    written: string[];
    walkError?: string;
    maps?: ServerMapInfo[];
}

type Mode = 'select' | 'paint' | 'pick' | 'walk' | 'spawn' | 'place' | 'area';
type WalkPaint = 'block' | 'open' | 'restore';
type Layer = 'picture' | 'blocked' | 'flags' | 'doors' | 'arrivals' | 'spawns' | 'rules' | 'points';
const LAYER_LABELS: Record<Layer, string> = {
    picture: 'Map picture',
    blocked: 'Blocked tiles',
    flags: 'Teleport flags (.amd)',
    doors: 'Doors',
    arrivals: 'Arrivals',
    spawns: 'Spot mob generators & NPCs',
    rules: 'No-attack & no-spawn areas',
    points: 'Waypoints, fish, mineral & start points',
};
// Colours for the map config layers
const RULE_COLOUR = { noAttack: 0x4fc3f7, npcAvoid: 0xffd54f, waypoint: 0xb0bec5, fish: 0x29b6f6, mineral: 0xbcaaa4, initial: 0x66bb6a, strike: 0xef5350 };

/** One map's editable state, saved before each change for undo. */
interface Snapshot {
    doors: Door[];
    source: 'builtin' | 'json';
    doorsDirty: boolean;
    blocked?: Uint8Array;
    walkDirty: boolean;
    spawns: Spawn[];
    spawnSource: 'builtin' | 'json';
    spawnDirty: boolean;
    selectedId?: number;
    selectedSpawnId?: number;
    tag?: string;
    time: number;
}

const $ = <E extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as E;
const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!));
const cloneDoors = (doors: Door[]): Door[] => doors.map((d) => ({ id: d.id, name: d.name, tiles: new Set(d.tiles), to: { ...d.to } }));

async function getJSON<R>(url: string, init?: RequestInit): Promise<R> {
    const r = await fetch(url, init);
    const body = await r.json();
    if (!r.ok) throw new Error(body?.error ?? `${r.status} ${r.statusText}`);
    return body as R;
}

class MapEditorScene extends Phaser.Scene {
    // --- data ---
    private infos = new Map<string, ServerMapInfo>(); // by lower-case name
    private mapNames = new Map<string, string>(); // lower -> server spelling
    private walkable: WalkOverrides = { version: 1, maps: {} };
    private clientFiles = new Map<string, string>(); // lower name -> client .amd file
    private doors = new Map<string, Door[]>(); // working copies, by lower-case map
    private source = new Map<string, 'builtin' | 'json'>();
    private dirty = new Set<string>(); // maps with unsaved door changes
    private walkDirty = new Set<string>(); // maps with unsaved walkability changes
    private flagsCache = new Map<string, TileFlags>();
    private flagsLoading = new Map<string, Promise<TileFlags | undefined>>();
    private npcTypes = new Map<number, NpcTypeInfo>();
    private spawns = new Map<string, Spawn[]>(); // working copies, by lower-case map
    private spawnSource = new Map<string, 'builtin' | 'json'>();
    private spawnDirty = new Set<string>();
    // One history for all maps: each step is the map it changed and that map's state before.
    private undo: { map: string; snap: Snapshot }[] = [];
    private redo: { map: string; snap: Snapshot }[] = [];

    // --- view ---
    private current = '';
    private flags?: TileFlags;
    private hbMap?: HBMap;
    private pictureNote = '';
    private selected: { map: string; door: Door } | null = null;
    private selectedSpawn: { map: string; spawn: Spawn } | null = null;
    private placeNpc = 15; // NPC type a new spawn starts as
    private mode: Mode = 'select';
    private walkPaint: WalkPaint = 'block';
    private brush = 1;
    private history: { map: string; x: number; y: number; zoom: number }[] = [];
    private layers: Record<Layer, boolean> = { picture: true, blocked: false, flags: false, doors: true, arrivals: true, spawns: true, rules: true, points: false };
    private overlayImages: Phaser.GameObjects.Image[] = [];
    private blockedTex?: Phaser.Textures.CanvasTexture;
    private blockedPixels?: ImageData;
    private doorGfx!: Phaser.GameObjects.Graphics;
    private arrivalGfx!: Phaser.GameObjects.Graphics;
    private hoverGfx!: Phaser.GameObjects.Graphics;
    private labels: Phaser.GameObjects.Text[] = [];
    private spawnGfx!: Phaser.GameObjects.Graphics;
    private rulesGfx!: Phaser.GameObjects.Graphics;
    private ruleLabels: Phaser.GameObjects.Text[] = [];
    private spawnObjects: { destroy(): void }[] = [];
    private npcSpritesLoading = new Set<string>();
    private problems: Problem[] = [];
    private openSeq = 0;

    // --- pointer ---
    private drag: { x: number; y: number; scrollX: number; scrollY: number; moved: boolean; button: number } | null = null;
    private paintOp: 'add' | 'remove' | null = null;
    private lastPaintTile = '';
    private walkStroke: { last: { x: number; y: number } } | null = null;
    private walkRect: { x0: number; y0: number; x1: number; y1: number } | null = null;
    private spawnDrag: { spawn: Spawn; from: { x: number; y: number }; orig: Spawn; moved: boolean } | null = null;
    private areaRect: { x0: number; y0: number; x1: number; y1: number } | null = null;

    constructor() {
        super('MapEditor');
    }

    preload(): void {
        for (const a of getAssetsByType(AssetType.TILE_SPRITE)) {
            this.load.binary(a.key, `${ASSET_BASE}/assets/sprites/${a.fileName}`);
        }
        this.load.on('progress', (v: number) => { $('loading').textContent = `Loading map sprites… ${Math.round(v * 100)}%`; });
    }

    async create(): Promise<void> {
        const tileAssets = getAssetsByType(AssetType.TILE_SPRITE);
        let done = 0;
        for (const a of tileAssets) {
            const buffer = this.cache.binary.get(a.key) as ArrayBuffer | undefined;
            if (buffer) {
                try {
                    await new HBSpriteFile(a.fileName.replace('.spr', ''), a.spriteType ?? SpriteType.Tiles, a.tileStartIndex).load(this, buffer);
                } catch (err) {
                    console.warn(`sprite ${a.key}:`, err);
                }
                this.cache.binary.remove(a.key);
            }
            $('loading').textContent = `Preparing map sprites… ${++done}/${tileAssets.length}`;
        }

        this.spawnGfx = this.add.graphics().setDepth(DEPTH.spawnAreas);
        this.rulesGfx = this.add.graphics().setDepth(DEPTH.rules);
        this.doorGfx = this.add.graphics().setDepth(DEPTH.doors);
        this.arrivalGfx = this.add.graphics().setDepth(DEPTH.arrivals);
        this.hoverGfx = this.add.graphics().setDepth(DEPTH.hover);
        this.cameras.main.setBackgroundColor('#0b0c0f');
        this.setupInput();
        this.setupPanel();

        $('loading').textContent = 'Reading the map configs (go run)…';
        try {
            await this.loadState(false);
        } catch (err) {
            $('loading').textContent = `Could not load the doors: ${err instanceof Error ? err.message : err}`;
            return;
        }
        $('loading').style.display = 'none';

        const params = new URLSearchParams(location.search);
        const start = params.get('map') ?? 'elvine';
        const x = params.get('x');
        const y = params.get('y');
        await this.openMap(this.mapNames.get(start.toLowerCase()) ?? this.infos.values().next().value!.name,
            x && y ? { x: Number(x), y: Number(y) } : undefined, false);
    }

    // ---------------------------------------------------------------------
    // Data
    // ---------------------------------------------------------------------

    private async loadState(refresh: boolean): Promise<void> {
        const s = await getJSON<ExportState>(`${API}state${refresh ? '?refresh=1' : ''}`);
        this.infos.clear();
        this.mapNames.clear();
        for (const m of s.maps) {
            this.infos.set(m.name.toLowerCase(), m);
            this.mapNames.set(m.name.toLowerCase(), m.name);
        }
        this.clientFiles.clear();
        for (const f of s.clientMaps) this.clientFiles.set(f.replace(/\.amd$/i, '').toLowerCase(), f);
        this.walkable = s.walkable ?? { version: 1, maps: {} };
        this.npcTypes.clear();
        for (const t of s.npcTypes ?? []) this.npcTypes.set(t.id, t);
        this.spawns.clear();
        this.spawnSource.clear();
        this.spawnDirty.clear();
        this.selectedSpawn = null;
        this.doors.clear();
        this.source.clear();
        this.dirty.clear();
        this.walkDirty.clear();
        this.flagsCache.clear();
        this.flagsLoading.clear();

        const banners: string[] = [];
        if (s.walkError) banners.push(`The server can't read walkable.json: ${esc(s.walkError)}. It is using the .amd walkability.`);
        for (const e of s.configErrors ?? []) {
            banners.push(`Map config error: ${esc(e)}. The server keeps that map's previous config; fix the file (the editor shows what the server uses).`);
        }
        const unknown = Object.keys(this.walkable.maps).filter((m) => !this.infos.has(m.toLowerCase()));
        if (unknown.length) banners.push(`walkable.json lists maps the server doesn't have: ${[...new Set(unknown)].map(esc).join(', ')} (kept as they are).`);
        $('banner').innerHTML = banners.map((b) => `<div class="banner">${b}</div>`).join('');

        const select = $<HTMLSelectElement>('mapSelect');
        select.innerHTML = s.maps.map((m) => `<option value="${esc(m.name)}">${esc(m.name)}</option>`).join('');
        this.refreshMapSelect();
    }

    private walkKey(lower: string): string | undefined {
        return Object.keys(this.walkable.maps).find((k) => k.toLowerCase() === lower);
    }

    /** The working copy of a map's spawns (from its map config). */
    private spawnsOf(map: string): Spawn[] {
        const lower = map.toLowerCase();
        let list = this.spawns.get(lower);
        if (!list) {
            list = (this.infos.get(lower)?.spawns ?? []).map(spawnFromSpot);
            this.spawnSource.set(lower, 'builtin');
            this.spawns.set(lower, list);
        }
        return list;
    }

    /** A change to a map's spawns: saved into its map config. */
    private spawnsTouched(map: string): void {
        const lower = map.toLowerCase();
        this.spawnsOf(map);
        this.spawnSource.set(lower, 'json');
        this.spawnDirty.add(lower);
        this.drawSpawns();
        this.refreshSpawnList();
        this.refreshSpawnEditorState();
        this.refreshMapSelect();
        this.refreshProblems();
        this.refreshSaveBar();
        this.refreshUndoButtons();
    }

    /**
     * The working copy of a map's doors (from its map config). Source 'json' means
     * exact doors (exact-doors = 1: each door works on exactly its tiles), the way
     * every map the editor saves works; 'builtin' means the .amd flag rule.
     */
    private doorsOf(map: string): Door[] {
        const lower = map.toLowerCase();
        let list = this.doors.get(lower);
        if (!list) {
            const info = this.infos.get(lower);
            list = info ? builtinDoors(info) : [];
            this.source.set(lower, info?.exact ? 'json' : 'builtin');
            this.doors.set(lower, list);
        }
        return list;
    }

    private isEdited(map: string): boolean {
        this.doorsOf(map);
        return this.source.get(map.toLowerCase()) === 'json';
    }

    /** A change to a map's doors: saved into its map config as exact doors. */
    private touched(map: string): void {
        const lower = map.toLowerCase();
        this.doorsOf(map);
        this.source.set(lower, 'json');
        this.dirty.add(lower);
        this.refreshAll();
    }

    private async flagsFor(map: string): Promise<TileFlags | undefined> {
        const lower = map.toLowerCase();
        const cached = this.flagsCache.get(lower);
        if (cached) return cached;
        const info = this.infos.get(lower);
        if (!info) return undefined;
        let p = this.flagsLoading.get(lower);
        if (!p) {
            p = fetch(`${API}server-map?file=${encodeURIComponent(info.file)}`)
                .then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(`${r.status}`))))
                .then((buf) => {
                    const f = parseAmdFlags(buf);
                    const key = this.walkKey(lower);
                    if (key) applyWalkEdit(f, this.walkable.maps[key]);
                    this.flagsCache.set(lower, f);
                    return f;
                })
                .catch((err) => {
                    console.warn(`server map ${info.file}:`, err);
                    return undefined;
                });
            this.flagsLoading.set(lower, p);
        }
        return p;
    }

    // ---------------------------------------------------------------------
    // Undo
    // ---------------------------------------------------------------------

    private snapshot(map: string, tag?: string): Snapshot {
        const lower = map.toLowerCase();
        const flags = this.flagsCache.get(lower);
        return {
            doors: cloneDoors(this.doorsOf(map)),
            source: this.source.get(lower) ?? 'builtin',
            doorsDirty: this.dirty.has(lower),
            blocked: flags?.blocked.slice(),
            walkDirty: this.walkDirty.has(lower),
            spawns: cloneSpawns(this.spawnsOf(map)),
            spawnSource: this.spawnSource.get(lower) ?? 'builtin',
            spawnDirty: this.spawnDirty.has(lower),
            selectedId: this.selected?.map.toLowerCase() === lower ? this.selected.door.id : undefined,
            selectedSpawnId: this.selectedSpawn?.map.toLowerCase() === lower ? this.selectedSpawn.spawn.id : undefined,
            tag,
            time: Date.now(),
        };
    }

    /**
     * Call before changing a map. Changes with the same tag less than 1.5s apart
     * (typing a name) share one undo step.
     */
    private beforeChange(map: string, tag?: string): void {
        const last = this.undo[this.undo.length - 1];
        if (tag && last?.map.toLowerCase() === map.toLowerCase() && last.snap.tag === tag && Date.now() - last.snap.time < 1500) {
            last.snap.time = Date.now();
            return;
        }
        this.undo.push({ map, snap: this.snapshot(map, tag) });
        if (this.undo.length > UNDO_LIMIT) this.undo.shift();
        this.redo = [];
        this.refreshUndoButtons();
    }

    /** Drops the last undo step (a stroke or drag that changed nothing). */
    private dropUndoStep(): void {
        this.undo.pop();
        this.refreshUndoButtons();
    }

    private restore(map: string, s: Snapshot): void {
        const lower = map.toLowerCase();
        this.doors.set(lower, s.doors);
        this.source.set(lower, s.source);
        if (s.doorsDirty) this.dirty.add(lower);
        else this.dirty.delete(lower);
        const flags = this.flagsCache.get(lower);
        if (flags && s.blocked) flags.blocked.set(s.blocked);
        if (s.walkDirty) this.walkDirty.add(lower);
        else this.walkDirty.delete(lower);
        this.spawns.set(lower, s.spawns);
        this.spawnSource.set(lower, s.spawnSource);
        if (s.spawnDirty) this.spawnDirty.add(lower);
        else this.spawnDirty.delete(lower);
        const spawn = s.selectedSpawnId !== undefined ? s.spawns.find((x) => x.id === s.selectedSpawnId) : undefined;
        if (this.selectedSpawn?.map.toLowerCase() === lower || spawn) {
            this.selectedSpawn = spawn ? { map, spawn } : null;
        }
        this.refreshSpawnEditor();
        this.drawSpawns();
        const door = s.selectedId !== undefined ? s.doors.find((d) => d.id === s.selectedId) : undefined;
        if (this.selected?.map.toLowerCase() === lower || door) {
            this.selected = door ? { map, door } : null;
            if (this.mode === 'paint' && !door) this.setMode('select');
        }
        this.refreshDoorEditor();
        this.redrawBlocked();
        this.refreshAll();
    }

    /** Undoes (or redoes) the last change, on whichever map it was made. */
    private undoStep(redo: boolean): void {
        const step = (redo ? this.redo : this.undo).pop();
        if (!step) return;
        (redo ? this.undo : this.redo).push({ map: step.map, snap: this.snapshot(step.map) });
        this.restore(step.map, step.snap);
        const where = step.map.toLowerCase() === this.current.toLowerCase() ? '' : ` (on ${step.map})`;
        this.setStatus(`${redo ? 'Redone' : 'Undone'}${where}.`);
    }

    private refreshUndoButtons(): void {
        const next = this.undo[this.undo.length - 1];
        const again = this.redo[this.redo.length - 1];
        $<HTMLButtonElement>('undoBtn').disabled = !next;
        $<HTMLButtonElement>('redoBtn').disabled = !again;
        const other = next && next.map.toLowerCase() !== this.current.toLowerCase() ? `next undo is on ${next.map}` : '';
        $('undoNote').textContent = other;
    }

    // ---------------------------------------------------------------------
    // Opening a map
    // ---------------------------------------------------------------------

    private async openMap(name: string, at?: { x: number; y: number }, pushHistory = true): Promise<void> {
        const seq = ++this.openSeq;
        const cam = this.cameras.main;
        if (pushHistory && this.current) {
            const c = this.viewCentreTile();
            this.history.push({ map: this.current, x: c.x, y: c.y, zoom: cam.zoom });
        }
        this.setStatus(`Loading ${name}…`);
        const flags = await this.flagsFor(name);
        if (seq !== this.openSeq) return;
        if (!flags) {
            this.setStatus(`Could not read the server's ${name}.amd`);
            return;
        }

        this.clearMap();
        this.current = name;
        this.flags = flags;
        if (this.selectedSpawn && this.selectedSpawn.map !== name) {
            this.selectedSpawn = null;
            this.refreshSpawnEditor();
        }
        $<HTMLSelectElement>('mapSelect').value = name;

        // The picture comes from the client's copy of the map, as the game draws it.
        this.pictureNote = '';
        const clientFile = this.clientFiles.get(name.toLowerCase());
        if (!clientFile) {
            this.pictureNote = 'The client has no copy of this map, so there is no picture.';
        } else {
            const key = `editor-map-${clientFile.toLowerCase()}`;
            try {
                if (!this.cache.binary.has(key)) {
                    const buf = await (await fetch(`${ASSET_BASE}/assets/maps/${clientFile}`)).arrayBuffer();
                    this.cache.binary.add(key, buf);
                }
                if (seq !== this.openSeq) return;
                const hbMap = new HBMap(key);
                hbMap.load(this);
                if (hbMap.sizeX !== flags.width || hbMap.sizeY !== flags.height) {
                    this.pictureNote = `The client's copy is ${hbMap.sizeX}×${hbMap.sizeY}, the server's ${flags.width}×${flags.height}: the picture may not line up.`;
                }
                hbMap.renderMapTiles(this);
                hbMap.renderMapObjects(this, false);
                hbMap.renderMapObjects(this, true);
                hbMap.getTilemapLayer()?.setPosition(0, 0);
                for (const o of hbMap.getMapObjects()) {
                    const p = o.getBasePosition();
                    o.setPosition(p.x + OBJECT_SHIFT, p.y + OBJECT_SHIFT);
                }
                this.hbMap = hbMap;
                this.applyPictureLayer();
            } catch (err) {
                this.pictureNote = `Could not draw the map picture: ${err instanceof Error ? err.message : err}`;
            }
        }

        this.buildFlagOverlays();
        cam.removeBounds();
        const c = at ?? { x: Math.floor(flags.width / 2), y: Math.floor(flags.height / 2) };
        cam.centerOn(c.x * T + T / 2, c.y * T + T / 2);

        const url = new URL(location.href);
        url.searchParams.set('map', name);
        url.searchParams.delete('x');
        url.searchParams.delete('y');
        if (at) {
            url.searchParams.set('x', String(at.x));
            url.searchParams.set('y', String(at.y));
        }
        history.replaceState(null, '', url);

        this.refreshAll();
        this.drawSpawns();
        this.refreshSpawnEditorState();
        this.setStatus(`${name}: ${flags.width}×${flags.height}`);

        // Destination maps' flags are needed for the arrival checks.
        const dests = new Set(this.doorsOf(name).map((d) => d.to.map));
        await Promise.all([...dests].map((m) => this.flagsFor(m)));
        if (seq === this.openSeq) this.refreshProblems();
    }

    private clearMap(): void {
        if (this.hbMap) {
            this.hbMap.destroyMapObjects();
            this.hbMap.destroyMapTiles(this);
            this.hbMap = undefined;
        }
        for (const img of this.overlayImages) img.destroy();
        this.overlayImages = [];
        this.blockedTex = undefined;
        this.blockedPixels = undefined;
        this.clearSpawnObjects();
        this.rulesGfx.clear();
        for (const t of this.ruleLabels) t.destroy();
        this.ruleLabels = [];
        for (const key of ['editor-overlay-blocked', 'editor-overlay-flags']) {
            if (this.textures.exists(key)) this.textures.remove(key);
        }
    }

    /** One pixel per tile, scaled up to tile size: cheap even for the biggest maps. */
    private buildFlagOverlays(): void {
        const f = this.flags!;
        const make = (key: string, depth: number, layer: Layer) => {
            const tex = this.textures.createCanvas(key, f.width, f.height)!;
            const image = this.add.image(0, 0, key).setOrigin(0).setScale(T).setDepth(depth).setVisible(this.layers[layer]);
            image.setData('layer', layer);
            this.overlayImages.push(image);
            return tex;
        };
        this.blockedTex = make('editor-overlay-blocked', DEPTH.blocked, 'blocked');
        this.blockedPixels = this.blockedTex.getContext().createImageData(f.width, f.height);
        this.redrawBlocked();

        const flagTex = make('editor-overlay-flags', DEPTH.flags, 'flags');
        const ctx = flagTex.getContext();
        const img = ctx.createImageData(f.width, f.height);
        for (let i = 0; i < f.teleport.length; i++) if (f.teleport[i]) img.data.set([255, 215, 0, 150], i * 4);
        ctx.putImageData(img, 0, 0);
        flagTex.refresh();
    }

    /** Repaints the blocked overlay from the current flags (edited tiles in their own colours). */
    private redrawBlocked(tiles?: Iterable<number>): void {
        const f = this.flags;
        const px = this.blockedPixels;
        const tex = this.blockedTex;
        if (!f || !px || !tex) return;
        const paint = (i: number) => {
            const now = f.blocked[i];
            const was = f.origBlocked[i];
            const c = now && was ? RGBA_BLOCKED : now ? RGBA_NEW_BLOCKED : was ? RGBA_NEW_OPEN : null;
            if (c) px.data.set(c, i * 4);
            else px.data.fill(0, i * 4, i * 4 + 4);
        };
        if (tiles) for (const i of tiles) paint(i);
        else for (let i = 0; i < f.blocked.length; i++) paint(i);
        tex.getContext().putImageData(px, 0, 0);
        tex.refresh();
    }

    private applyPictureLayer(): void {
        if (!this.hbMap) return;
        this.hbMap.getTilemapLayer()?.setVisible(this.layers.picture);
        for (const o of this.hbMap.getMapObjects()) o.setVisible(this.layers.picture);
    }

    private setLayer(layer: Layer, on: boolean): void {
        this.layers[layer] = on;
        const box = document.querySelector<HTMLInputElement>(`#layers input[data-layer=${layer}]`);
        if (box) box.checked = on;
        if (layer === 'picture') this.applyPictureLayer();
        for (const img of this.overlayImages) img.setVisible(this.layers[img.getData('layer') as Layer]);
        this.drawDoors();
        this.drawArrivals();
        if (layer === 'spawns') this.drawSpawns();
        if (layer === 'rules' || layer === 'points') this.drawRules();
    }

    /** Unfolds a panel section (selecting something in it, or starting its tool). */
    private openSection(name: string): void {
        const d = document.querySelector<HTMLDetailsElement>(`details.section[data-section="${name}"]`);
        if (d && !d.open) d.open = true;
    }

    // ---------------------------------------------------------------------
    // Drawing
    // ---------------------------------------------------------------------

    private refreshAll(): void {
        this.drawDoors();
        this.drawArrivals();
        this.refreshMapSelect();
        this.refreshMapInfo();
        this.refreshWalkInfo();
        this.refreshDoorList();
        this.refreshArrivalList();
        this.refreshSpawnList();
        this.refreshProblems();
        this.refreshSaveBar();
        this.refreshUndoButtons();
    }

    private doorColour(map: string, door: Door): number {
        const i = this.doorsOf(map).indexOf(door);
        return PALETTE[(i < 0 ? 0 : i) % PALETTE.length];
    }

    private drawDoors(): void {
        const g = this.doorGfx;
        g.clear();
        g.setVisible(this.layers.doors);
        if (!this.current || !this.flags) return;
        const doors = this.doorsOf(this.current);
        const sel = this.selectedHere();
        const drawSet = (tiles: Set<string>, colour: number, fillAlpha: number, line: number, lineColour: number) => {
            g.fillStyle(colour, fillAlpha);
            for (const k of tiles) {
                const [x, y] = parseTileKey(k);
                g.fillRect(x * T, y * T, T, T);
            }
            // Outline only the door's outer edges.
            g.lineStyle(line, lineColour, 1);
            for (const k of tiles) {
                const [x, y] = parseTileKey(k);
                const px = x * T;
                const py = y * T;
                if (!tiles.has(tileKey(x, y - 1))) g.lineBetween(px, py, px + T, py);
                if (!tiles.has(tileKey(x, y + 1))) g.lineBetween(px, py + T, px + T, py + T);
                if (!tiles.has(tileKey(x - 1, y))) g.lineBetween(px, py, px, py + T);
                if (!tiles.has(tileKey(x + 1, y))) g.lineBetween(px + T, py, px + T, py + T);
            }
        };
        // While editing walkability the doors are drawn lighter, so the tiles show through.
        const dim = this.mode === 'walk' ? 0.4 : 1;
        for (const d of doors) {
            const c = this.doorColour(this.current, d);
            if (d === sel) drawSet(d.tiles, c, 0.7 * dim, 3, 0xffffff);
            else drawSet(d.tiles, c, 0.4 * dim, 2, c);
        }
        // Door tiles players can't step on.
        g.lineStyle(2, 0xff3030, 1);
        for (const d of doors) {
            for (const k of d.tiles) {
                const [x, y] = parseTileKey(k);
                if (!isBlocked(this.flags, x, y)) continue;
                g.lineBetween(x * T + 6, y * T + 6, x * T + T - 6, y * T + T - 6);
                g.lineBetween(x * T + T - 6, y * T + 6, x * T + 6, y * T + T - 6);
            }
        }
        // Built-in entries that never fire (no teleport flag on the tile).
        if (!this.isEdited(this.current)) {
            const info = this.infos.get(this.current.toLowerCase());
            g.lineStyle(2, 0xff40ff, 1);
            for (const t of info?.unflagged ?? []) {
                g.strokeRect(t.x * T + 3, t.y * T + 3, T - 6, T - 6);
                g.lineBetween(t.x * T + 3, t.y * T + 3, t.x * T + T - 3, t.y * T + T - 3);
            }
        }
    }

    /** Every door (on any map) whose arrival spot is on the current map. */
    private arrivalsHere(): { map: string; door: Door }[] {
        const out: { map: string; door: Door }[] = [];
        const here = this.current.toLowerCase();
        for (const info of this.infos.values()) {
            for (const door of this.doorsOf(info.name)) {
                if (door.to.map.toLowerCase() === here) out.push({ map: info.name, door });
            }
        }
        return out;
    }

    private drawArrivals(): void {
        const g = this.arrivalGfx;
        g.clear();
        for (const l of this.labels) l.destroy();
        this.labels = [];
        g.setVisible(this.layers.arrivals);
        if (!this.current || !this.layers.arrivals) return;
        const seen = new Map<string, string[]>();
        for (const a of this.arrivalsHere()) {
            const { x, y, dir } = a.door.to;
            const isSel = this.selected?.door === a.door;
            const colour = isSel ? 0xffffff : this.doorColour(a.map, a.door);
            const cx = x * T + T / 2;
            const cy = y * T + T / 2;
            g.fillStyle(colour, isSel ? 0.9 : 0.55);
            g.fillCircle(cx, cy, 9);
            g.lineStyle(isSel ? 3 : 2, isSel ? 0x000000 : 0x101010, 1);
            g.strokeCircle(cx, cy, 9);
            if (dir > 0) {
                const ex = cx + DIR_DX[dir] * 20;
                const ey = cy + DIR_DY[dir] * 20;
                g.lineStyle(3, colour, 1);
                g.lineBetween(cx, cy, ex, ey);
                const ang = Math.atan2(ey - cy, ex - cx);
                g.fillStyle(colour, 1);
                g.fillTriangle(
                    ex + Math.cos(ang) * 6, ey + Math.sin(ang) * 6,
                    ex + Math.cos(ang + 2.4) * 6, ey + Math.sin(ang + 2.4) * 6,
                    ex + Math.cos(ang - 2.4) * 6, ey + Math.sin(ang - 2.4) * 6,
                );
            }
            const k = tileKey(x, y);
            if (!seen.has(k)) seen.set(k, []);
            if (!seen.get(k)!.includes(a.map)) seen.get(k)!.push(a.map);
        }
        for (const [k, maps] of seen) {
            const [x, y] = parseTileKey(k);
            const t = this.add.text(x * T + T / 2, y * T - 4, `from ${maps.join(', ')}`, {
                fontSize: '11px', color: '#ffffff', backgroundColor: '#000000a0', padding: { x: 3, y: 1 },
            }).setOrigin(0.5, 1).setDepth(DEPTH.arrivals);
            this.labels.push(t);
        }
    }

    // ---------------------------------------------------------------------
    // Panel
    // ---------------------------------------------------------------------

    private setupPanel(): void {
        const select = $<HTMLSelectElement>('mapSelect');
        select.addEventListener('change', () => this.openMap(select.value));
        $('backBtn').addEventListener('click', () => this.goBack());
        $('newDoorBtn').addEventListener('click', () => this.newDoor());
        $('saveBtn').addEventListener('click', () => this.save());
        $('revertBtn').addEventListener('click', () => this.revertMap());
        $('undoBtn').addEventListener('click', () => this.undoStep(false));
        $('redoBtn').addEventListener('click', () => this.undoStep(true));

        const layers = $('layers');
        layers.innerHTML = (Object.keys(LAYER_LABELS) as Layer[])
            .map((l) => `<label><input type="checkbox" data-layer="${l}" ${this.layers[l] ? 'checked' : ''}> ${LAYER_LABELS[l]}</label>`)
            .join('');
        layers.addEventListener('change', (e) => {
            const input = e.target as HTMLInputElement;
            this.setLayer(input.dataset.layer as Layer, input.checked);
        });

        // Walkability tool
        $('walkBtn').addEventListener('click', () => this.setMode(this.mode === 'walk' ? 'select' : 'walk'));
        $('walkPaint').addEventListener('click', (e) => {
            const b = (e.target as HTMLElement).closest<HTMLButtonElement>('button[data-paint]');
            if (b) this.setWalkPaint(b.dataset.paint as WalkPaint);
        });
        $<HTMLSelectElement>('brushSize').addEventListener('change', (e) => { this.brush = Number((e.target as HTMLSelectElement).value); });
        $('walkResetBtn').addEventListener('click', () => this.resetWalk());

        // Spawns
        $('spawnBtn').addEventListener('click', () => this.setMode(this.mode === 'spawn' ? 'select' : 'spawn'));
        $('newSpawnBtn').addEventListener('click', () => this.setMode('place'));
        $('spawnRevertBtn').addEventListener('click', () => this.revertSpawns());
        this.refreshSpawnEditor();

        // Folding sections: remembered per browser.
        for (const d of document.querySelectorAll<HTMLDetailsElement>('details.section')) {
            const key = `hb-map-editor-section-${d.dataset.section}`;
            try {
                const saved = localStorage.getItem(key);
                if (saved !== null) d.open = saved === '1';
            } catch { /* storage blocked: keep the defaults */ }
            d.addEventListener('toggle', () => {
                try { localStorage.setItem(key, d.open ? '1' : '0'); } catch { /* ignore */ }
            });
        }

        window.addEventListener('beforeunload', (e) => {
            if (this.dirty.size || this.walkDirty.size || this.spawnDirty.size) e.preventDefault();
        });
        this.refreshDoorEditor();
        this.setWalkPaint('block');
    }

    private refreshMapSelect(): void {
        const select = $<HTMLSelectElement>('mapSelect');
        for (const opt of Array.from(select.options)) {
            const lower = opt.value.toLowerCase();
            const marks = [
                this.dirty.has(lower) || this.walkDirty.has(lower) || this.spawnDirty.has(lower) ? '● unsaved'
                    : this.walkKey(lower) ? 'walk edits' : '',
                this.clientFiles.has(lower) ? '' : 'no picture',
            ].filter(Boolean);
            opt.textContent = marks.length ? `${opt.value}  (${marks.join(', ')})` : opt.value;
        }
        if (this.current) select.value = this.current;
        $<HTMLButtonElement>('backBtn').disabled = this.history.length === 0;
    }

    private refreshMapInfo(): void {
        if (!this.current) return;
        const lower = this.current.toLowerCase();
        const info = this.infos.get(lower);
        const tag = this.dirty.has(lower)
            ? '<span class="tag dirty">unsaved door changes</span>'
            : this.isEdited(this.current) ? '<span class="tag json">exact doors</span>' : '<span class="tag">doors by teleport flag</span>';
        const how = this.isEdited(this.current)
            ? 'A door works on exactly its tiles; the teleport flags are ignored.'
            : 'A tile works as a door if it has the teleport flag and a door entry within 2 tiles. Saving this map\'s doors makes them exact.';
        const file = info?.config
            ? `<code>maps/${esc(info.config)}</code>`
            : '<span class="sev-error">no map config: doors and spawns can\'t be saved</span>';
        $('mapInfo').innerHTML = `${file} ${this.settingsSummary(info)}<br>${tag} ${how}${this.pictureNote ? `<br>${esc(this.pictureNote)}` : ''}`;
    }

    /** The map config's settings in a line ("town aresden · up to 600 monsters · ..."). */
    private settingsSummary(info: ServerMapInfo | undefined): string {
        const s = info?.settings;
        if (!s) return '';
        const parts: string[] = [];
        if (s.location && s.location.toLowerCase() !== info!.name.toLowerCase()) parts.push(`side ${s.location}`);
        if (s.maxObjects) parts.push(`up to ${s.maxObjects} monsters`);
        if (s.randomMobs) parts.push(`random monsters level ${s.randomMobs} (not ported yet)`);
        if (s.fixedDay) parts.push('indoors: always day, no weather');
        if (s.levelLimit) parts.push(`level ${s.levelLimit}+`);
        if (s.upperLevelLimit) parts.push(`up to level ${s.upperLevelLimit}`);
        const n = (k: string, v: unknown[]) => (v.length ? `${v.length} ${k}` : '');
        const pts = [n('no-attack', s.noAttack), n('waypoints', s.waypoints), n('fish', s.fishPoints), n('mineral', s.mineralPoints)].filter(Boolean);
        if (pts.length) parts.push(pts.join(', '));
        return parts.length ? `· ${esc(parts.join(' · '))}` : '';
    }

    private refreshWalkInfo(): void {
        const f = this.flags;
        if (!f) return;
        const lower = this.current.toLowerCase();
        const n = walkEditCount(f);
        $('walkSummary').textContent = n ? `${n} changed${this.walkDirty.has(this.current.toLowerCase()) ? ' · unsaved' : ''}` : '';
        const state = this.walkDirty.has(lower) ? '<span class="tag dirty">unsaved</span> ' : n ? '<span class="tag json">saved</span> ' : '';
        $('walkInfo').innerHTML = n
            ? `${state}${n} tile${n === 1 ? '' : 's'} changed from the .amd on this map.`
            : 'No walkability changes on this map.';
        $<HTMLButtonElement>('walkResetBtn').disabled = n === 0;
        $('walkBtn').classList.toggle('on', this.mode === 'walk');
        $('walkBtn').textContent = this.mode === 'walk' ? 'Done editing walkability' : 'Edit walkability';
    }

    private setWalkPaint(p: WalkPaint): void {
        this.walkPaint = p;
        for (const b of document.querySelectorAll<HTMLButtonElement>('#walkPaint button')) {
            b.classList.toggle('on', b.dataset.paint === p);
        }
    }

    private refreshDoorList(): void {
        const list = $('doorList');
        if (!this.current) return;
        const doors = this.doorsOf(this.current);
        $('doorsSummary').textContent = `${doors.length} door${doors.length === 1 ? '' : 's'}${this.dirty.has(this.current.toLowerCase()) ? ' · unsaved' : ''}`;
        list.innerHTML = doors.length
            ? doors.map((d) => `<li data-id="${d.id}" class="${this.selected?.door === d ? 'sel' : ''}">
                <span class="sw" style="background:${css(this.doorColour(this.current, d))}"></span>
                <span class="grow">${esc(doorLabel(d))}</span><span class="count">${d.tiles.size} tile${d.tiles.size === 1 ? '' : 's'}</span></li>`).join('')
            : '<li class="note">No doors.</li>';
        list.onclick = (e) => {
            const li = (e.target as HTMLElement).closest('li');
            const door = doors.find((d) => String(d.id) === li?.dataset.id);
            if (door) this.select(this.current, door, true);
        };
    }

    private refreshArrivalList(): void {
        const list = $('arrivalList');
        const arrivals = this.arrivalsHere();
        list.innerHTML = arrivals.length
            ? arrivals.map((a, i) => `<li data-i="${i}" class="${this.selected?.door === a.door ? 'sel' : ''}">
                <span class="sw" style="background:${css(this.doorColour(a.map, a.door))}"></span>
                <span class="grow">${esc(a.map)}${a.door.name ? ` (${esc(a.door.name)})` : ''} → ${a.door.to.x}, ${a.door.to.y}</span>
                <span class="count">${a.door.to.dir ? `facing ${DIR_NAMES[a.door.to.dir]}` : 'keeps facing'}</span></li>`).join('')
            : '<li class="note">No doors lead to this map.</li>';
        list.onclick = (e) => {
            const li = (e.target as HTMLElement).closest('li');
            if (!li?.dataset.i) return;
            const a = arrivals[Number(li.dataset.i)];
            this.cameras.main.centerOn(a.door.to.x * T + T / 2, a.door.to.y * T + T / 2);
            this.select(a.map, a.door, false);
        };
        list.ondblclick = (e) => {
            const li = (e.target as HTMLElement).closest('li');
            if (!li?.dataset.i) return;
            const a = arrivals[Number(li.dataset.i)];
            this.goToDoor(a.map, a.door);
        };
    }

    private refreshProblems(): void {
        const list = $('problemList');
        if (!this.current || !this.flags) return;
        this.problems = findProblems({
            mapName: this.current,
            doors: this.doorsOf(this.current),
            flags: this.flags,
            edited: this.isEdited(this.current),
            info: this.infos.get(this.current.toLowerCase())!,
            mapNames: this.mapNames,
            doorsOf: (m) => this.doorsOf(m),
            flagsOf: (m) => this.flagsCache.get(m.toLowerCase()),
        });
        const spawnProblems = findSpawnProblems({
            spawns: this.spawnsOf(this.current),
            flags: this.flags,
            doors: this.doorsOf(this.current),
            types: this.npcTypes,
        });
        const rank = { error: 0, warn: 1, info: 2 };
        this.problems = [...this.problems, ...spawnProblems].sort((a, b) => rank[a.severity] - rank[b.severity]);
        const errors = this.problems.filter((p) => p.severity === 'error').length;
        const warns = this.problems.filter((p) => p.severity === 'warn').length;
        $('problemsSummary').textContent = [errors && `${errors} error${errors === 1 ? '' : 's'}`, warns && `${warns} warning${warns === 1 ? '' : 's'}`]
            .filter(Boolean).join(', ');
        list.innerHTML = this.problems.length
            ? this.problems.map((p, i) => `<li data-i="${i}"><span class="sev-${p.severity}">${p.severity === 'error' ? '✖' : p.severity === 'warn' ? '▲' : '•'}</span>
                <span class="grow">${esc(p.text)}${p.fix === 'enable-unflagged' ? ' <button data-fix="1">Make them work</button>' : ''}</span></li>`).join('')
            : '<li class="note">Nothing found.</li>';
        list.onclick = (e) => {
            const target = e.target as HTMLElement;
            const li = target.closest('li');
            if (!li?.dataset.i) return;
            const p = this.problems[Number(li.dataset.i)];
            if (target.dataset.fix) {
                this.enableUnflagged();
                return;
            }
            if (p.x !== undefined && p.y !== undefined) this.cameras.main.centerOn(p.x * T + T / 2, p.y * T + T / 2);
            const door = this.doorsOf(this.current).find((d) => d.id === p.doorId);
            if (door) this.select(this.current, door, false);
            const spawn = this.spawnsOf(this.current).find((x) => x.id === p.spawnId);
            if (spawn) this.selectSpawn(this.current, spawn, false);
        };
    }

    private refreshSaveBar(): void {
        const n = new Set([...this.dirty, ...this.walkDirty, ...this.spawnDirty]).size;
        $<HTMLButtonElement>('saveBtn').disabled = n === 0;
        $<HTMLButtonElement>('saveBtn').textContent = n ? `Save (${n} map${n === 1 ? '' : 's'})` : 'Save';
        const lower = this.current.toLowerCase();
        $<HTMLButtonElement>('revertBtn').disabled = !this.current || !this.dirty.has(lower);
        $<HTMLButtonElement>('spawnRevertBtn').disabled = !this.current || !this.spawnDirty.has(lower);
    }

    /** The door editor is rebuilt only when the selection changes, so typing keeps focus. */
    private refreshDoorEditor(): void {
        const box = $('doorEditor');
        const sel = this.selected;
        if (!sel) {
            box.innerHTML = '<div class="note">Click a door on the map or in the list below.</div>';
            return;
        }
        const d = sel.door;
        const mapOptions = [...this.mapNames.values()]
            .map((m) => `<option value="${esc(m)}" ${m === d.to.map ? 'selected' : ''}>${esc(m)}</option>`).join('');
        const unknownMap = d.to.map && !this.mapNames.has(d.to.map.toLowerCase()) ? `<option selected value="${esc(d.to.map)}">${esc(d.to.map)} (missing)</option>` : '';
        box.innerHTML = `
          <div class="row"><span class="note">On <b>${esc(sel.map)}</b>, <span id="deTiles"></span></span></div>
          <div class="row"><label>Name</label><input id="deName" class="grow" placeholder="optional, e.g. Shop" value="${esc(d.name)}"></div>
          <div class="row"><label>Leads to</label><select id="deMap" class="grow"><option value="" ${d.to.map ? '' : 'selected'}>(choose a map)</option>${unknownMap}${mapOptions}</select></div>
          <div class="row"><label>x</label><input id="deX" type="number" value="${d.to.x}"><label>y</label><input id="deY" type="number" value="${d.to.y}">
            <label>facing</label><select id="deDir">${DIR_NAMES.map((n, i) => `<option value="${i}" ${i === d.to.dir ? 'selected' : ''}>${n}</option>`).join('')}</select></div>
          <div class="row">
            <button id="dePaint" title="Click or drag on the map to add or remove this door's tiles">Edit tiles</button>
            <button id="dePick" title="Click the tile where players should arrive (switch maps first if needed)">Pick arrival</button>
            <button id="deGo" title="Show the arrival spot">Go to arrival</button>
          </div>
          <div class="row">
            <button id="deShow">Show door</button>
            <button id="deDelete" class="danger">Delete door</button>
          </div>`;
        this.refreshDoorEditorState();

        const changed = () => {
            this.touched(sel.map);
            this.refreshDoorEditorState();
        };
        $<HTMLInputElement>('deName').addEventListener('input', (e) => {
            this.beforeChange(sel.map, `name-${d.id}`);
            d.name = (e.target as HTMLInputElement).value;
            changed();
        });
        $<HTMLSelectElement>('deMap').addEventListener('change', async (e) => {
            this.beforeChange(sel.map);
            d.to.map = (e.target as HTMLSelectElement).value;
            await this.flagsFor(d.to.map);
            changed();
        });
        const num = (id: string, set: (v: number) => void) => $<HTMLInputElement>(id).addEventListener('change', (e) => {
            const v = Math.trunc(Number((e.target as HTMLInputElement).value));
            if (Number.isFinite(v)) {
                this.beforeChange(sel.map);
                set(v);
                changed();
            }
        });
        num('deX', (v) => { d.to.x = v; });
        num('deY', (v) => { d.to.y = v; });
        $<HTMLSelectElement>('deDir').addEventListener('change', (e) => {
            this.beforeChange(sel.map);
            d.to.dir = Number((e.target as HTMLSelectElement).value);
            changed();
        });
        $('dePaint').addEventListener('click', () => this.setMode(this.mode === 'paint' ? 'select' : 'paint'));
        $('dePick').addEventListener('click', () => this.startPick());
        $('deGo').addEventListener('click', () => this.goToArrival());
        $('deShow').addEventListener('click', () => this.goToDoor(sel.map, d));
        $('deDelete').addEventListener('click', () => this.deleteDoor());
    }

    /** The parts of the door editor that change without a new selection. */
    private refreshDoorEditorState(): void {
        const sel = this.selected;
        if (!sel || !$('deTiles')) return;
        const n = sel.door.tiles.size;
        $('deTiles').textContent = `${n} tile${n === 1 ? '' : 's'}`;
        const here = sel.map === this.current;
        const paint = $<HTMLButtonElement>('dePaint');
        paint.disabled = !here;
        paint.classList.toggle('on', this.mode === 'paint');
        paint.textContent = this.mode === 'paint' ? 'Done editing tiles' : 'Edit tiles';
        $('dePick').classList.toggle('on', this.mode === 'pick');
        $<HTMLButtonElement>('deGo').disabled = !this.mapNames.has(sel.door.to.map.toLowerCase());
        $<HTMLInputElement>('deX').value = String(sel.door.to.x);
        $<HTMLInputElement>('deY').value = String(sel.door.to.y);
        $<HTMLSelectElement>('deMap').value = sel.door.to.map;
        $<HTMLSelectElement>('deDir').value = String(sel.door.to.dir);
        if (document.activeElement !== $('deName')) $<HTMLInputElement>('deName').value = sel.door.name;
    }

    // ---------------------------------------------------------------------
    // Actions
    // ---------------------------------------------------------------------

    private selectedHere(): Door | undefined {
        return this.selected && this.selected.map === this.current ? this.selected.door : undefined;
    }

    private select(map: string, door: Door | null, centre: boolean): void {
        if (this.mode === 'paint' && (!door || door !== this.selected?.door)) this.setMode('select');
        this.selected = door ? { map, door } : null;
        if (door) this.openSection('doors');
        if (door && this.selectedSpawn) {
            this.selectedSpawn = null;
            this.refreshSpawnEditor();
            this.drawSpawns();
            this.refreshSpawnList();
        }
        if (door && centre && map === this.current) {
            const [x, y] = parseTileKey([...door.tiles][0] ?? '0,0');
            this.cameras.main.centerOn(x * T + T / 2, y * T + T / 2);
        }
        this.refreshDoorEditor();
        this.drawDoors();
        this.drawArrivals();
        this.refreshDoorList();
        this.refreshArrivalList();
    }

    private setMode(mode: Mode): void {
        this.mode = mode;
        this.walkStroke = null;
        this.walkRect = null;
        this.spawnDrag = null;
        this.areaRect = null;
        const banner = $('mode');
        const text = mode === 'paint'
            ? 'Editing door tiles: click or drag to add/remove tiles of the selected door. Esc when done.'
            : mode === 'pick' ? 'Click the tile where players should arrive (switch maps first if needed). Esc cancels.'
            : mode === 'walk' ? 'Editing walkability: click or drag to paint, Shift+drag for a rectangle. B / O / R pick the paint, [ and ] the brush size. Esc when done.'
            : mode === 'spawn' ? 'Editing spawns: drag an NPC (or a spawn area) to move it, click to select. Delete removes the selected spawn. Esc when done.'
            : mode === 'place' ? 'Click where the new spawn goes. Esc cancels.'
            : mode === 'area' ? 'Drag a rectangle for the selected spawn: its NPCs spawn and wander inside it. Esc cancels.'
            : '';
        banner.textContent = text;
        banner.style.display = text ? 'block' : 'none';
        if (mode === 'walk') {
            this.setLayer('blocked', true);
            this.openSection('walk');
        }
        this.refreshDoorEditorState();
        this.refreshWalkInfo();
        this.refreshSpawnEditorState();
        this.drawDoors();
        if (mode === 'spawn' || mode === 'place' || mode === 'area') this.setLayer('spawns', true);
    }

    private newDoor(): void {
        if (!this.current) return;
        this.beforeChange(this.current);
        const door: Door = { id: newDoorId(), name: '', tiles: new Set(), to: { map: '', x: 0, y: 0, dir: 0 } };
        this.doorsOf(this.current).push(door);
        this.touched(this.current);
        this.select(this.current, door, false);
        this.setMode('paint');
    }

    private deleteDoor(): void {
        const sel = this.selected;
        if (!sel || !confirm(`Delete the door ${doorLabel(sel.door)} on ${sel.map}?`)) return;
        this.beforeChange(sel.map);
        const list = this.doorsOf(sel.map);
        list.splice(list.indexOf(sel.door), 1);
        this.selected = null;
        this.setMode('select');
        this.touched(sel.map);
        this.refreshDoorEditor();
    }

    private startPick(): void {
        const sel = this.selected;
        if (!sel) return;
        if (this.mode === 'pick') {
            this.setMode('select');
            return;
        }
        this.setMode('pick');
        const dest = this.mapNames.get(sel.door.to.map.toLowerCase());
        if (dest && dest !== this.current) this.openMap(dest, { x: sel.door.to.x, y: sel.door.to.y });
    }

    private goToArrival(): void {
        const sel = this.selected;
        const dest = sel && this.mapNames.get(sel.door.to.map.toLowerCase());
        if (!sel || !dest) return;
        if (dest === this.current) {
            this.cameras.main.centerOn(sel.door.to.x * T + T / 2, sel.door.to.y * T + T / 2);
            return;
        }
        this.openMap(dest, { x: sel.door.to.x, y: sel.door.to.y });
    }

    private async goToDoor(map: string, door: Door): Promise<void> {
        const [x, y] = parseTileKey([...door.tiles][0] ?? '0,0');
        if (map !== this.current) await this.openMap(map, { x, y });
        else this.cameras.main.centerOn(x * T + T / 2, y * T + T / 2);
        this.select(map, door, false);
    }

    private async goBack(): Promise<void> {
        const h = this.history.pop();
        if (!h) return;
        await this.openMap(h.map, { x: h.x, y: h.y }, false);
        this.cameras.main.setZoom(h.zoom);
        this.refreshMapSelect();
    }

    private enableUnflagged(): void {
        const info = this.infos.get(this.current.toLowerCase());
        if (!info) return;
        this.beforeChange(this.current);
        this.doors.set(this.current.toLowerCase(), builtinDoorsWithUnflagged(info));
        this.selected = null;
        this.touched(this.current);
        this.refreshDoorEditor();
    }

    private revertMap(): void {
        const lower = this.current.toLowerCase();
        const info = this.infos.get(lower);
        if (!info || !confirm(`Throw away ${this.current}'s unsaved door changes and go back to its map config? (Undo can bring them back.)`)) return;
        this.beforeChange(this.current);
        this.doors.set(lower, builtinDoors(info));
        this.source.set(lower, info.exact ? 'json' : 'builtin');
        this.dirty.delete(lower);
        this.selected = null;
        this.setMode('select');
        this.refreshDoorEditor();
        this.refreshAll();
    }

    private resetWalk(): void {
        const f = this.flags;
        if (!f || !confirm(`Put every tile on ${this.current} back to the .amd's walkability? (Undo can bring them back.)`)) return;
        this.beforeChange(this.current);
        f.blocked.set(f.origBlocked);
        this.walkChanged();
        this.redrawBlocked();
    }

    /** Sets the tiles to the current walk paint; returns the changed tile indexes. */
    private paintWalk(tiles: Iterable<[number, number]>): number[] {
        const f = this.flags;
        if (!f) return [];
        const changed: number[] = [];
        for (const [x, y] of tiles) {
            if (x < 0 || y < 0 || x >= f.width || y >= f.height) continue;
            const i = y * f.width + x;
            const v = this.walkPaint === 'block' ? 1 : this.walkPaint === 'open' ? 0 : f.origBlocked[i];
            if (f.blocked[i] !== v) {
                f.blocked[i] = v;
                changed.push(i);
            }
        }
        if (changed.length) {
            this.redrawBlocked(changed);
            this.walkDirty.add(this.current.toLowerCase());
        }
        return changed;
    }

    /** The brush's tiles around (x, y). */
    private brushTiles(x: number, y: number): [number, number][] {
        const out: [number, number][] = [];
        const lo = -Math.floor((this.brush - 1) / 2);
        for (let dy = lo; dy < lo + this.brush; dy++) {
            for (let dx = lo; dx < lo + this.brush; dx++) out.push([x + dx, y + dy]);
        }
        return out;
    }

    private rectTiles(r: { x0: number; y0: number; x1: number; y1: number }): [number, number][] {
        const out: [number, number][] = [];
        for (let y = Math.min(r.y0, r.y1); y <= Math.max(r.y0, r.y1); y++) {
            for (let x = Math.min(r.x0, r.x1); x <= Math.max(r.x0, r.x1); x++) out.push([x, y]);
        }
        return out;
    }

    /** After a walk stroke: the things that depend on walkability. */
    private walkChanged(): void {
        this.walkDirty.add(this.current.toLowerCase());
        this.drawDoors();
        this.refreshMapSelect();
        this.refreshWalkInfo();
        this.refreshProblems();
        this.refreshSaveBar();
        this.refreshUndoButtons();
    }

    private async save(): Promise<void> {
        const status = $('saveStatus');
        if (!this.dirty.size && !this.walkDirty.size && !this.spawnDirty.size) return;
        const issues: string[] = [];
        const nameOf = (lower: string) => this.mapNames.get(lower) ?? lower;

        // Doors and spawns of the changed maps go into their map configs
        let doors: Record<string, OverrideDoor[]> | undefined;
        for (const lower of this.dirty) {
            const name = nameOf(lower);
            if (!this.infos.get(lower)?.config) { issues.push(`${name}: has no map config to save doors in`); continue; }
            for (const d of this.doorsOf(name)) {
                if (!d.tiles.size) issues.push(`${name}: a door has no tiles (${doorLabel(d)})`);
                if (!this.infos.get(d.to.map.toLowerCase())?.config) issues.push(`${name}: ${doorLabel(d)} leads to a map without a map config`);
            }
            (doors ??= {})[name] = doorsToOverride(this.doorsOf(name));
        }
        let spawns: Record<string, SpawnSpot[]> | undefined;
        for (const lower of this.spawnDirty) {
            const name = nameOf(lower);
            if (!this.infos.get(lower)?.config) { issues.push(`${name}: has no map config to save spawns in`); continue; }
            for (const sp of this.spawnsOf(name)) {
                const t = this.npcTypes.get(sp.npc);
                if (!t) issues.push(`${name}: a spawn has an unknown NPC type (${sp.npc})`);
                else if (!t.configName) issues.push(`${name}: ${t.name} (type ${sp.npc}) shares its name with other types and can't be saved; pick another`);
            }
            (spawns ??= {})[name] = this.spawnsOf(name).map(spawnToSpot);
        }

        let walkable: WalkOverrides | undefined;
        if (this.walkDirty.size) {
            const maps: WalkOverrides['maps'] = { ...this.walkable.maps };
            for (const lower of this.walkDirty) {
                const f = this.flagsCache.get(lower);
                if (!f) continue;
                const name = nameOf(lower);
                const key = this.walkKey(lower);
                if (key) delete maps[key];
                const edit = walkEditOf(f);
                if (edit.block || edit.open) maps[name] = edit;
            }
            walkable = { version: 1, maps };
        }

        if (issues.length) {
            status.innerHTML = `<span class="sev-error">Not saved:</span><br>${issues.map(esc).join('<br>')}`;
            return;
        }
        status.textContent = 'Saving…';
        try {
            const r = await getJSON<SaveResult>(`${API}save`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ doors, spawns, walkable }),
            });
            if (!r.ok) {
                const why = [...(r.errors ?? []), r.walkError ?? ''].filter(Boolean);
                status.innerHTML = `<span class="sev-error">Not saved:</span><br>${why.map(esc).join('<br>')}`;
                return;
            }
            const n = new Set([...this.dirty, ...this.walkDirty, ...this.spawnDirty]).size;
            if (walkable) this.walkable = walkable;
            // The saved maps' configs as the server now reads them (exact doors, spawn lines)
            for (const m of r.maps ?? []) {
                const lower = m.name.toLowerCase();
                if (this.dirty.has(lower) || this.spawnDirty.has(lower)) this.infos.set(lower, m);
            }
            // Saved maps keep their working copies; only the "unsaved" marks go.
            this.dirty.clear();
            this.walkDirty.clear();
            this.spawnDirty.clear();
            this.refreshAll();
            const files = [
                ...r.written.map((m) => `maps/${m}.txt`),
                ...(walkable ? ['walkable.json'] : []),
            ];
            status.innerHTML = `Saved ${n} map${n === 1 ? '' : 's'}${files.length ? `: ${files.map((x) => `<code>${esc(x)}</code>`).join(', ')}` : ' (no file changed)'}
                in <code>server/internal/mapdata/</code>. A running game server loads the changes within a second; admins see a notice in game.
                ${spawns ? 'Maps with changed spawns get all their spawned NPCs replaced.' : ''}`;
        } catch (err) {
            status.innerHTML = `<span class="sev-error">Save failed:</span> ${esc(err instanceof Error ? err.message : String(err))}`;
        }
    }

    // ---------------------------------------------------------------------
    // Map config layers (no-attack areas, points): shown, not edited yet
    // ---------------------------------------------------------------------

    private drawRules(): void {
        const g = this.rulesGfx;
        g.clear();
        for (const t of this.ruleLabels) t.destroy();
        this.ruleLabels = [];
        const s = this.infos.get(this.current.toLowerCase())?.settings;
        if (!s) return;
        const label = (text: string, x: number, y: number, colour: number) => {
            this.ruleLabels.push(this.add.text(x, y, text, {
                fontFamily: 'monospace', fontSize: '11px', color: css(colour), backgroundColor: '#000000aa', padding: { x: 2, y: 1 },
            }).setOrigin(0, 0).setDepth(DEPTH.rules + 0.1));
        };
        if (this.layers.rules) {
            for (const [num, x1, y1, x2, y2] of s.noAttack) {
                g.fillStyle(RULE_COLOUR.noAttack, 0.18);
                g.fillRect(x1 * T, y1 * T, (x2 - x1 + 1) * T, (y2 - y1 + 1) * T);
                g.lineStyle(2, RULE_COLOUR.noAttack, 0.9);
                g.strokeRect(x1 * T, y1 * T, (x2 - x1 + 1) * T, (y2 - y1 + 1) * T);
                label(`no-attack ${num}`, x1 * T + 2, y1 * T + 2, RULE_COLOUR.noAttack);
            }
            for (const [num, x1, y1, x2, y2] of s.npcAvoid) {
                g.lineStyle(2, RULE_COLOUR.npcAvoid, 0.8);
                g.strokeRect(x1 * T, y1 * T, (x2 - x1 + 1) * T, (y2 - y1 + 1) * T);
                label(`no random monsters ${num}`, x1 * T + 2, y2 * T + T - 16, RULE_COLOUR.npcAvoid);
            }
        }
        if (this.layers.points) {
            const dot = (x: number, y: number, colour: number, r: number) => {
                g.fillStyle(colour, 0.9);
                g.fillCircle(x * T + T / 2, y * T + T / 2, r);
                g.lineStyle(1, 0x000000, 0.8);
                g.strokeCircle(x * T + T / 2, y * T + T / 2, r);
            };
            for (const [num, x, y] of s.waypoints) { dot(x, y, RULE_COLOUR.waypoint, 4); label(`wp ${num}`, x * T + T / 2 + 5, y * T + 4, RULE_COLOUR.waypoint); }
            for (const [num, x, y] of s.fishPoints) { dot(x, y, RULE_COLOUR.fish, 7); label(`fish ${num}`, x * T + T / 2 + 8, y * T + 4, RULE_COLOUR.fish); }
            for (const [num, x, y] of s.mineralPoints) { dot(x, y, RULE_COLOUR.mineral, 7); label(`mineral ${num}`, x * T + T / 2 + 8, y * T + 4, RULE_COLOUR.mineral); }
            for (const [num, x, y] of s.strikePoints) { dot(x, y, RULE_COLOUR.strike, 7); label(`strike ${num}`, x * T + T / 2 + 8, y * T + 4, RULE_COLOUR.strike); }
            for (const [num, x, y] of s.initialPoints) { dot(x, y, RULE_COLOUR.initial, 8); label(`start ${num}`, x * T + T / 2 + 9, y * T + 4, RULE_COLOUR.initial); }
        }
    }

    /** What the map config says about a tile, for the status bar. */
    private rulesAt(x: number, y: number): string[] {
        const s = this.infos.get(this.current.toLowerCase())?.settings;
        if (!s) return [];
        const out: string[] = [];
        const inside = (r: number[]) => x >= r[1] && x <= r[3] && y >= r[2] && y <= r[4];
        for (const r of s.noAttack) if (inside(r)) out.push(`no-attack area ${r[0]}`);
        for (const r of s.npcAvoid) if (inside(r)) out.push(`no random monsters (${r[0]})`);
        const at = (list: [number, number, number][], what: string) => {
            for (const [n, px, py] of list) if (px === x && py === y) out.push(`${what} ${n}`);
        };
        at(s.waypoints, 'waypoint');
        at(s.fishPoints, 'fish point');
        at(s.mineralPoints, 'mineral point');
        at(s.strikePoints, 'strike point');
        at(s.initialPoints, 'start point');
        return out;
    }

    // ---------------------------------------------------------------------
    // Spawns
    // ---------------------------------------------------------------------

    private clearSpawnObjects(): void {
        for (const o of this.spawnObjects) o.destroy();
        this.spawnObjects = [];
        this.spawnGfx?.clear();
    }

    /** Draws the current map's spawns: areas, spread squares, the NPC sprites and labels. */
    private drawSpawns(): void {
        this.clearSpawnObjects();
        if (!this.current || !this.layers.spawns) return;
        const g = this.spawnGfx;
        const selected = this.selectedSpawn?.map === this.current ? this.selectedSpawn.spawn : undefined;
        for (const sp of this.spawnsOf(this.current)) {
            const type = this.npcTypes.get(sp.npc);
            const colour = KIND_COLOUR[spawnKind(type)];
            const isSel = sp === selected;
            if (sp.area) {
                const [x1, y1, x2, y2] = sp.area;
                g.fillStyle(colour, isSel ? 0.22 : 0.1);
                g.fillRect(x1 * T, y1 * T, (x2 - x1 + 1) * T, (y2 - y1 + 1) * T);
                g.lineStyle(isSel ? 3 : 2, isSel ? 0xffffff : colour, 0.9);
                g.strokeRect(x1 * T, y1 * T, (x2 - x1 + 1) * T, (y2 - y1 + 1) * T);
            } else if (sp.radius > 0) {
                const r = sp.radius;
                g.lineStyle(isSel ? 3 : 2, isSel ? 0xffffff : colour, 0.7);
                g.strokeRect((sp.x - r) * T, (sp.y - r) * T, (2 * r + 1) * T, (2 * r + 1) * T);
            }
            const at = spawnAnchor(sp);
            // Marker on the spawn tile, then the NPC as the game draws it.
            g.fillStyle(colour, isSel ? 0.75 : 0.45);
            g.fillRect(at.x * T + 4, at.y * T + 4, T - 8, T - 8);
            g.lineStyle(isSel ? 3 : 2, isSel ? 0xffffff : 0x101010, 1);
            g.strokeRect(at.x * T + 4, at.y * T + 4, T - 8, T - 8);
            const sprite = NPC_SPRITE_MAP[sp.npc];
            if (sprite && this.npcSpriteReady(sprite)) {
                const dir = sp.dir > 0 ? sp.dir : 5;
                try {
                    const a = new GameAsset(this, { x: at.x * T + OBJECT_SHIFT, y: at.y * T + OBJECT_SHIFT, spriteName: sprite, spriteSheetIndex: dir - 1, frameRate: 6 });
                    a.setDepth(DEPTH.spawns);
                    this.spawnObjects.push(a);
                } catch (err) {
                    console.warn(`sprite ${sprite}:`, err);
                }
            }
            const label = this.add.text(at.x * T + T / 2, at.y * T + T + 2, spawnLabel(sp, this.npcTypes), {
                fontSize: '11px', color: isSel ? '#ffffff' : css(colour), backgroundColor: '#000000b0', padding: { x: 3, y: 1 },
            }).setOrigin(0.5, 0).setDepth(DEPTH.spawns + 0.5);
            this.spawnObjects.push(label);
        }
    }

    /** True when an NPC sprite's textures exist; otherwise starts loading it (and redraws when done). */
    private npcSpriteReady(name: string): boolean {
        if (this.textures.exists(`${name}-0`)) return true;
        if (this.npcSpritesLoading.has(name)) return false;
        const asset = getAssetByKey(name);
        if (!asset) return false;
        this.npcSpritesLoading.add(name);
        fetch(`${ASSET_BASE}/assets/sprites/${asset.fileName}`)
            .then((r) => r.arrayBuffer())
            .then((buf) => new HBSpriteFile(asset.fileName.replace('.spr', ''), asset.spriteType ?? SpriteType.Monster).load(this, buf))
            .then(() => this.drawSpawns())
            .catch((err) => console.warn(`npc sprite ${name}:`, err));
        return false;
    }

    /** The spawn at a tile: an NPC drawn there (its tile or up to 2 above, where the body is), or with areas, any area holding it. */
    private spawnAt(x: number, y: number, areas: boolean): Spawn | undefined {
        if (!this.current) return undefined;
        const list = this.spawnsOf(this.current);
        const sel = this.selectedSpawn?.map === this.current ? this.selectedSpawn.spawn : undefined;
        const near = list.filter((sp) => {
            const at = spawnAnchor(sp);
            return Math.abs(at.x - x) <= 0 && y <= at.y && y >= at.y - 2;
        });
        const hit = near.find((sp) => sp === sel) ?? near.sort((a, b) => spawnAnchor(b).y - spawnAnchor(a).y)[0];
        if (hit || !areas) return hit;
        const inside = list.filter((sp) => sp.area && x >= sp.area[0] && x <= sp.area[2] && y >= sp.area[1] && y <= sp.area[3]);
        // The smallest area is the one you most likely meant.
        const size = (sp: Spawn) => (sp.area![2] - sp.area![0] + 1) * (sp.area![3] - sp.area![1] + 1);
        return inside.find((sp) => sp === sel) ?? inside.sort((a, b) => size(a) - size(b))[0];
    }

    private dragSpawn(t: { x: number; y: number }): void {
        const d = this.spawnDrag;
        if (!d) return;
        const dx = t.x - d.from.x;
        const dy = t.y - d.from.y;
        if (!dx && !dy && !d.moved) return;
        d.moved = true;
        const sp = d.spawn;
        sp.x = d.orig.x + dx;
        sp.y = d.orig.y + dy;
        if (d.orig.area) {
            const [x1, y1, x2, y2] = d.orig.area;
            sp.area = [x1 + dx, y1 + dy, x2 + dx, y2 + dy];
        }
        this.drawSpawns();
        this.refreshSpawnEditorState();
    }

    private selectSpawn(map: string, spawn: Spawn | null, centre: boolean): void {
        this.selectedSpawn = spawn ? { map, spawn } : null;
        if (spawn) {
            this.openSection('spawns');
            this.placeNpc = spawn.npc;
            if (this.selected) {
                this.selected = null;
                this.refreshDoorEditor();
                this.drawDoors();
                this.drawArrivals();
                this.refreshDoorList();
            }
            if (centre && map === this.current) {
                const at = spawnAnchor(spawn);
                this.cameras.main.centerOn(at.x * T + T / 2, at.y * T + T / 2);
            }
        }
        this.refreshSpawnEditor();
        this.refreshSpawnList();
        this.drawSpawns();
    }

    private placeSpawn(x: number, y: number): void {
        this.beforeChange(this.current);
        const sp: Spawn = { id: newSpawnId(), npc: this.placeNpc, count: 0, x, y, radius: 0, area: null, dir: 0, respawn: 0, note: '', src: '' };
        this.spawnsOf(this.current).push(sp);
        this.setMode('spawn');
        this.selectSpawn(this.current, sp, false);
        this.spawnsTouched(this.current);
    }

    private deleteSpawn(): void {
        const sel = this.selectedSpawn;
        if (!sel) return;
        this.beforeChange(sel.map);
        const list = this.spawnsOf(sel.map);
        list.splice(list.indexOf(sel.spawn), 1);
        this.selectedSpawn = null;
        this.refreshSpawnEditor();
        this.spawnsTouched(sel.map);
    }

    private duplicateSpawn(): void {
        const sel = this.selectedSpawn;
        if (!sel) return;
        this.beforeChange(sel.map);
        const copy: Spawn = { ...sel.spawn, id: newSpawnId(), src: '', area: sel.spawn.area ? [...sel.spawn.area] as Spawn['area'] : null };
        copy.x += 1;
        if (copy.area) copy.area = [copy.area[0] + 1, copy.area[1], copy.area[2] + 1, copy.area[3]];
        this.spawnsOf(sel.map).push(copy);
        this.selectSpawn(sel.map, copy, false);
        this.spawnsTouched(sel.map);
    }

    private revertSpawns(): void {
        const lower = this.current.toLowerCase();
        if (!confirm(`Throw away ${this.current}'s unsaved spawn changes and go back to its map config? (Undo can bring them back.)`)) return;
        this.beforeChange(this.current);
        this.spawns.set(lower, (this.infos.get(lower)?.spawns ?? []).map(spawnFromSpot));
        this.spawnSource.set(lower, 'builtin');
        this.spawnDirty.delete(lower);
        this.selectedSpawn = null;
        this.refreshSpawnEditor();
        this.drawSpawns();
        this.refreshSpawnList();
        this.refreshMapSelect();
        this.refreshProblems();
        this.refreshSaveBar();
    }

    private refreshSpawnList(): void {
        const list = $('spawnList');
        if (!this.current) return;
        const spawns = this.spawnsOf(this.current);
        const lower = this.current.toLowerCase();
        $('spawnsSummary').textContent = `${spawns.length} spawn${spawns.length === 1 ? '' : 's'}${this.spawnDirty.has(lower) ? ' · unsaved' : ''}`;
        $('spawnInfo').innerHTML = (this.spawnDirty.has(lower)
            ? '<span class="tag dirty">unsaved spawn changes</span> '
            : '<span class="tag">from the map config</span> ')
            + `${spawns.length} spawn${spawns.length === 1 ? '' : 's'}, ${spawns.reduce((n, s) => n + spawnCount(s), 0)} NPCs.`;
        list.innerHTML = spawns.length
            ? spawns.map((sp) => {
                const at = sp.area ? `area ${sp.area[0]},${sp.area[1]}–${sp.area[2]},${sp.area[3]}` : `${sp.x}, ${sp.y}${sp.radius ? ` ±${sp.radius}` : ''}`;
                const colour = KIND_COLOUR[spawnKind(this.npcTypes.get(sp.npc))];
                return `<li data-id="${sp.id}" class="${this.selectedSpawn?.spawn === sp ? 'sel' : ''}">
                    <span class="sw" style="background:${css(colour)}"></span>
                    <span class="grow">${esc(spawnLabel(sp, this.npcTypes))}</span><span class="count">${at}</span></li>`;
            }).join('')
            : '<li class="note">No spawns on this map.</li>';
        list.onclick = (e) => {
            const li = (e.target as HTMLElement).closest('li');
            const sp = spawns.find((x) => String(x.id) === li?.dataset.id);
            if (sp) this.selectSpawn(this.current, sp, true);
        };
    }

    /** The spawn editor is rebuilt only when the selection changes, so typing keeps focus. */
    private refreshSpawnEditor(): void {
        const box = $('spawnEditor');
        const sel = this.selectedSpawn;
        if (!sel) {
            box.innerHTML = '<div class="note">Click an NPC or a spawn area on the map, or one in the list below.</div>';
            return;
        }
        const sp = sel.spawn;
        const types = [...this.npcTypes.values()]
            .map((t) => `<option value="${t.id}" ${t.id === sp.npc ? 'selected' : ''}>${esc(t.name)} (${t.id})</option>`).join('');
        box.innerHTML = `
          <div class="row"><span class="note">On <b>${esc(sel.map)}</b></span></div>
          <div class="row"><label>NPC</label><select id="seNpc" class="grow">${types}</select></div>
          <div class="row"><label>Count</label><input id="seCount" type="number" min="1" max="500" value="${spawnCount(sp)}">
            <label>Respawn</label><input id="seRespawn" type="number" min="0" value="${sp.respawn}" title="Seconds; 0 = the NPC type's own delay"><span class="note">s</span></div>
          <div class="row"><label><input id="seUseArea" type="checkbox" ${sp.area ? 'checked' : ''}> Spawn area</label>
            <span class="note">(NPCs spawn and wander anywhere inside)</span></div>
          <div class="row" id="sePointRow"><label>x</label><input id="seX" type="number" value="${sp.x}"><label>y</label><input id="seY" type="number" value="${sp.y}">
            <label title="NPCs spread over ±this many tiles">±</label><input id="seRadius" type="number" min="0" value="${sp.radius}"></div>
          <div class="row" id="seAreaRow"><label>x1</label><input id="seX1" type="number"><label>y1</label><input id="seY1" type="number">
            <label>x2</label><input id="seX2" type="number"><label>y2</label><input id="seY2" type="number"></div>
          <div class="row"><label>Facing</label><select id="seDir">${DIR_NAMES.map((n, i) => `<option value="${i}" ${i === sp.dir ? 'selected' : ''}>${i === 0 ? 'default' : n}</option>`).join('')}</select>
            <label>Note</label><input id="seNote" class="grow" placeholder="optional" value="${esc(sp.note)}"></div>
          <div class="row">
            <button id="seDrawArea" title="Drag a rectangle on the map for this spawn's area">Draw area</button>
            <button id="seShow">Show</button>
            <button id="seDup">Duplicate</button>
            <button id="seDelete" class="danger">Delete</button>
          </div>`;
        this.refreshSpawnEditorState();

        const change = (tag: string, apply: () => void) => {
            this.beforeChange(sel.map, `spawn-${tag}-${sp.id}`);
            apply();
            this.spawnsTouched(sel.map);
        };
        const num = (id: string) => Math.trunc(Number($<HTMLInputElement>(id).value)) || 0;
        $<HTMLSelectElement>('seNpc').addEventListener('change', (e) => change('npc', () => {
            sp.npc = Number((e.target as HTMLSelectElement).value);
            this.placeNpc = sp.npc;
        }));
        $('seCount').addEventListener('input', () => change('count', () => { sp.count = Math.max(1, Math.min(500, num('seCount'))); }));
        $('seRespawn').addEventListener('input', () => change('respawn', () => { sp.respawn = Math.max(0, num('seRespawn')); }));
        $('seX').addEventListener('change', () => change('x', () => { sp.x = num('seX'); }));
        $('seY').addEventListener('change', () => change('y', () => { sp.y = num('seY'); }));
        $('seRadius').addEventListener('change', () => change('radius', () => { sp.radius = Math.max(0, num('seRadius')); }));
        const areaInput = () => change('area', () => {
            const a: [number, number, number, number] = [num('seX1'), num('seY1'), num('seX2'), num('seY2')];
            sp.area = [Math.min(a[0], a[2]), Math.min(a[1], a[3]), Math.max(a[0], a[2]), Math.max(a[1], a[3])];
        });
        for (const id of ['seX1', 'seY1', 'seX2', 'seY2']) $(id).addEventListener('change', areaInput);
        $<HTMLInputElement>('seUseArea').addEventListener('change', (e) => change('usearea', () => {
            if ((e.target as HTMLInputElement).checked) {
                const r = Math.max(sp.radius, 3);
                sp.area = [sp.x - r, sp.y - r, sp.x + r, sp.y + r];
            } else if (sp.area) {
                const at = spawnAnchor(sp);
                sp.x = at.x;
                sp.y = at.y;
                sp.area = null;
            }
        }));
        $<HTMLSelectElement>('seDir').addEventListener('change', (e) => change('dir', () => { sp.dir = Number((e.target as HTMLSelectElement).value); }));
        $<HTMLInputElement>('seNote').addEventListener('input', (e) => change('note', () => { sp.note = (e.target as HTMLInputElement).value; }));
        $('seDrawArea').addEventListener('click', () => this.setMode(this.mode === 'area' ? 'spawn' : 'area'));
        $('seShow').addEventListener('click', () => this.selectSpawn(sel.map, sp, true));
        $('seDup').addEventListener('click', () => this.duplicateSpawn());
        $('seDelete').addEventListener('click', () => this.deleteSpawn());
    }

    /** The parts of the spawn editor that change without a new selection. */
    private refreshSpawnEditorState(): void {
        $('spawnBtn').classList.toggle('on', this.mode === 'spawn' || this.mode === 'area');
        $('spawnBtn').textContent = this.mode === 'spawn' || this.mode === 'area' ? 'Done editing spawns' : 'Edit spawns';
        $('newSpawnBtn').classList.toggle('on', this.mode === 'place');
        const sel = this.selectedSpawn;
        if (!sel || !$('seX')) return;
        const sp = sel.spawn;
        const set = (id: string, v: number) => {
            const el = $<HTMLInputElement>(id);
            if (document.activeElement !== el) el.value = String(v);
        };
        set('seX', sp.x);
        set('seY', sp.y);
        set('seRadius', sp.radius);
        const a = sp.area ?? [sp.x, sp.y, sp.x, sp.y];
        set('seX1', a[0]);
        set('seY1', a[1]);
        set('seX2', a[2]);
        set('seY2', a[3]);
        $<HTMLInputElement>('seUseArea').checked = !!sp.area;
        $('sePointRow').style.display = sp.area ? 'none' : '';
        $('seAreaRow').style.display = sp.area ? '' : 'none';
        $('seDrawArea').classList.toggle('on', this.mode === 'area');
    }

    // ---------------------------------------------------------------------
    // Input
    // ---------------------------------------------------------------------

    private tileAt(px: number, py: number): { x: number; y: number } {
        const cam = this.cameras.main;
        const wx = cam.scrollX + cam.width / 2 + (px - cam.width / 2) / cam.zoom;
        const wy = cam.scrollY + cam.height / 2 + (py - cam.height / 2) / cam.zoom;
        return { x: Math.floor(wx / T), y: Math.floor(wy / T) };
    }

    private viewCentreTile(): { x: number; y: number } {
        const cam = this.cameras.main;
        return { x: Math.floor((cam.scrollX + cam.width / 2) / T), y: Math.floor((cam.scrollY + cam.height / 2) / T) };
    }

    private setupInput(): void {
        this.input.mouse?.disableContextMenu();
        const cam = this.cameras.main;

        this.input.on('wheel', (p: Phaser.Input.Pointer, _o: unknown, _dx: number, dy: number) => {
            const zoom = Phaser.Math.Clamp(cam.zoom * (dy > 0 ? 1 / 1.2 : 1.2), 0.1, 4);
            // Keep the point under the mouse where it is.
            const wx = cam.scrollX + cam.width / 2 + (p.x - cam.width / 2) / cam.zoom;
            const wy = cam.scrollY + cam.height / 2 + (p.y - cam.height / 2) / cam.zoom;
            cam.setZoom(zoom);
            cam.setScroll(wx - cam.width / 2 - (p.x - cam.width / 2) / zoom, wy - cam.height / 2 - (p.y - cam.height / 2) / zoom);
        });

        this.input.on('pointerdown', (p: Phaser.Input.Pointer) => {
            this.drag = { x: p.x, y: p.y, scrollX: cam.scrollX, scrollY: cam.scrollY, moved: false, button: p.button };
            if (p.button !== 0) return;
            const t = this.tileAt(p.x, p.y);
            if (this.mode === 'paint') {
                const door = this.selectedHere();
                if (door) {
                    this.beforeChange(this.current);
                    this.paintOp = door.tiles.has(tileKey(t.x, t.y)) ? 'remove' : 'add';
                    this.lastPaintTile = '';
                    this.paint(t.x, t.y);
                }
            } else if (this.mode === 'spawn') {
                const sp = this.spawnAt(t.x, t.y, true);
                if (sp) {
                    this.beforeChange(this.current);
                    this.selectSpawn(this.current, sp, false);
                    this.spawnDrag = { spawn: sp, from: t, orig: { ...sp, area: sp.area ? [...sp.area] as Spawn['area'] : null }, moved: false };
                }
            } else if (this.mode === 'area' && this.selectedSpawn) {
                this.areaRect = { x0: t.x, y0: t.y, x1: t.x, y1: t.y };
            } else if (this.mode === 'walk' && this.flags) {
                this.beforeChange(this.current);
                if ((p.event as MouseEvent).shiftKey) {
                    this.walkRect = { x0: t.x, y0: t.y, x1: t.x, y1: t.y };
                } else {
                    this.walkStroke = { last: t };
                    this.paintWalk(this.brushTiles(t.x, t.y));
                }
                this.showHover(t.x, t.y);
            }
        });

        this.input.on('pointermove', (p: Phaser.Input.Pointer) => {
            const t = this.tileAt(p.x, p.y);
            if (this.spawnDrag && p.isDown) {
                this.dragSpawn(t);
            } else if (this.areaRect && p.isDown) {
                this.areaRect.x1 = t.x;
                this.areaRect.y1 = t.y;
            }
            if (this.walkRect && p.isDown) {
                this.walkRect.x1 = t.x;
                this.walkRect.y1 = t.y;
            } else if (this.walkStroke && p.isDown) {
                const { last } = this.walkStroke;
                if (last.x !== t.x || last.y !== t.y) {
                    const tiles: [number, number][] = [];
                    for (const [lx, ly] of lineTiles(last.x, last.y, t.x, t.y)) tiles.push(...this.brushTiles(lx, ly));
                    this.paintWalk(tiles);
                    this.walkStroke.last = t;
                }
            }
            this.showHover(t.x, t.y);
            if (!this.drag || !p.isDown) return;
            if (this.paintOp) {
                this.paint(t.x, t.y);
                return;
            }
            if (this.walkStroke || this.walkRect || this.spawnDrag || this.areaRect) return;
            const dx = p.x - this.drag.x;
            const dy = p.y - this.drag.y;
            if (!this.drag.moved && Math.hypot(dx, dy) < 5) return;
            this.drag.moved = true;
            cam.setScroll(this.drag.scrollX - dx / cam.zoom, this.drag.scrollY - dy / cam.zoom);
        });

        this.input.on('pointerup', (p: Phaser.Input.Pointer) => {
            const drag = this.drag;
            this.drag = null;
            if (this.paintOp) {
                this.paintOp = null;
                return;
            }
            if (this.spawnDrag) {
                const d = this.spawnDrag;
                this.spawnDrag = null;
                if (d.moved) this.spawnsTouched(this.current);
                else {
                    this.dropUndoStep();
                }
                return;
            }
            if (this.areaRect && this.selectedSpawn) {
                const r = this.areaRect;
                this.areaRect = null;
                this.beforeChange(this.selectedSpawn.map);
                this.selectedSpawn.spawn.area = [Math.min(r.x0, r.x1), Math.min(r.y0, r.y1), Math.max(r.x0, r.x1), Math.max(r.y0, r.y1)];
                this.setMode('spawn');
                this.refreshSpawnEditor();
                this.spawnsTouched(this.selectedSpawn.map);
                return;
            }
            if (this.walkRect) {
                const changed = this.paintWalk(this.rectTiles(this.walkRect));
                this.walkRect = null;
                this.finishWalkStroke(changed.length > 0);
                return;
            }
            if (this.walkStroke) {
                this.walkStroke = null;
                this.finishWalkStroke(true);
                return;
            }
            if (!drag || drag.moved || drag.button !== 0) return;
            const t = this.tileAt(p.x, p.y);
            this.click(t.x, t.y);
        });

        window.addEventListener('keydown', (e) => {
            const typing = (e.target as HTMLElement)?.closest?.('input, select, textarea');
            const ctrl = e.ctrlKey || e.metaKey;
            if (ctrl && e.key.toLowerCase() === 's') {
                e.preventDefault();
                this.save();
                return;
            }
            if (typing) return;
            if (ctrl && e.key.toLowerCase() === 'z') {
                e.preventDefault();
                this.undoStep(e.shiftKey);
                return;
            }
            if (ctrl && e.key.toLowerCase() === 'y') {
                e.preventDefault();
                this.undoStep(true);
                return;
            }
            if (e.key === 'Escape') this.setMode(this.mode === 'area' || this.mode === 'place' ? 'spawn' : 'select');
            if ((e.key === 'Delete' || e.key === 'Backspace') && this.mode === 'spawn' && this.selectedSpawn) this.deleteSpawn();
            if (this.mode === 'walk') {
                const k = e.key.toLowerCase();
                if (k === 'b') this.setWalkPaint('block');
                if (k === 'o') this.setWalkPaint('open');
                if (k === 'r') this.setWalkPaint('restore');
                const sizes = [1, 2, 3, 5];
                if (e.key === '[' || e.key === ']') {
                    const i = Phaser.Math.Clamp(sizes.indexOf(this.brush) + (e.key === ']' ? 1 : -1), 0, sizes.length - 1);
                    this.brush = sizes[i];
                    $<HTMLSelectElement>('brushSize').value = String(this.brush);
                }
            }
            const step = 8 * T / cam.zoom;
            if (e.key === 'ArrowLeft') cam.scrollX -= step;
            if (e.key === 'ArrowRight') cam.scrollX += step;
            if (e.key === 'ArrowUp') cam.scrollY -= step;
            if (e.key === 'ArrowDown') cam.scrollY += step;
        });
    }

    private finishWalkStroke(changed: boolean): void {
        if (!changed) {
            // Nothing changed: drop the undo step this stroke saved.
            this.dropUndoStep();
            return;
        }
        this.walkChanged();
    }

    private click(x: number, y: number): void {
        if (!this.current) return;
        if (this.mode === 'walk' || this.mode === 'spawn' || this.mode === 'area') return;
        if (this.mode === 'place') {
            this.placeSpawn(x, y);
            return;
        }
        if (this.mode === 'pick') {
            const sel = this.selected;
            if (!sel) return this.setMode('select');
            this.beforeChange(sel.map);
            sel.door.to.map = this.current;
            sel.door.to.x = x;
            sel.door.to.y = y;
            this.setMode('select');
            this.touched(sel.map);
            this.refreshDoorEditorState();
            this.setStatus(`Arrival for ${doorLabel(sel.door)} set. Back returns to ${sel.map}.`);
            return;
        }
        const k = tileKey(x, y);
        // NPCs first (they stand in front), then doors, then spawn areas.
        const point = this.layers.spawns ? this.spawnAt(x, y, false) : undefined;
        if (point) {
            this.selectSpawn(this.current, point, false);
            return;
        }
        const door = this.doorsOf(this.current).find((d) => d.tiles.has(k));
        if (door || !this.layers.spawns) {
            this.select(this.current, door ?? null, false);
            if (!door) this.selectSpawn(this.current, null, false);
            return;
        }
        const area = this.spawnAt(x, y, true);
        this.select(this.current, null, false);
        this.selectSpawn(this.current, area ?? null, false);
    }

    private paint(x: number, y: number): void {
        const door = this.selectedHere();
        const f = this.flags;
        if (!door || !this.paintOp || !f || x < 0 || y < 0 || x >= f.width || y >= f.height) return;
        const k = tileKey(x, y);
        if (k === this.lastPaintTile) return;
        this.lastPaintTile = k;
        if (this.paintOp === 'add') {
            if (door.tiles.has(k)) return;
            // A tile belongs to one door only.
            const list = this.doorsOf(this.current);
            for (const other of list) {
                if (other !== door && other.tiles.delete(k) && other.tiles.size === 0) list.splice(list.indexOf(other), 1);
            }
            door.tiles.add(k);
        } else if (!door.tiles.delete(k)) {
            return;
        }
        sortDoors(this.doorsOf(this.current));
        this.touched(this.current);
        this.refreshDoorEditorState();
    }

    private showHover(x: number, y: number): void {
        const g = this.hoverGfx;
        g.clear();
        const f = this.flags;
        if (!f) return;
        if (this.mode === 'area' && this.areaRect) {
            const r = this.areaRect;
            const x0 = Math.min(r.x0, r.x1);
            const y0 = Math.min(r.y0, r.y1);
            const w = Math.abs(r.x1 - r.x0) + 1;
            const h = Math.abs(r.y1 - r.y0) + 1;
            g.fillStyle(0xff7043, 0.2);
            g.lineStyle(2, 0xff7043, 1);
            g.fillRect(x0 * T, y0 * T, w * T, h * T);
            g.strokeRect(x0 * T, y0 * T, w * T, h * T);
            this.setStatus(`Spawn area ${w}×${h}: (${x0}, ${y0}) to (${x0 + w - 1}, ${y0 + h - 1})`);
            return;
        }
        if (this.mode === 'walk') {
            const colour = this.walkPaint === 'block' ? 0xff8c00 : this.walkPaint === 'open' ? 0x28e65a : 0xffffff;
            const tiles = this.walkRect ? this.rectTiles(this.walkRect) : this.brushTiles(x, y);
            g.fillStyle(colour, 0.25);
            g.lineStyle(2, colour, 1);
            const xs = tiles.map((t) => t[0]);
            const ys = tiles.map((t) => t[1]);
            const x0 = Math.min(...xs);
            const y0 = Math.min(...ys);
            const w = Math.max(...xs) - x0 + 1;
            const h = Math.max(...ys) - y0 + 1;
            g.fillRect(x0 * T, y0 * T, w * T, h * T);
            g.strokeRect(x0 * T, y0 * T, w * T, h * T);
            if (this.walkRect) this.setStatus(`Rectangle ${w}×${h} (${x0}, ${y0}) to (${x0 + w - 1}, ${y0 + h - 1})`);
            if (this.walkRect) return;
        } else {
            if (x < 0 || y < 0 || x >= f.width || y >= f.height) return;
            g.lineStyle(2, this.mode === 'pick' ? 0xffd76a : 0xffffff, 0.9);
            g.strokeRect(x * T, y * T, T, T);
        }
        if (x < 0 || y < 0 || x >= f.width || y >= f.height) return;

        const i = y * f.width + x;
        const edited = f.blocked[i] !== f.origBlocked[i] ? ' (edited)' : '';
        const parts = [`${this.current} (${x}, ${y})`, `${isBlocked(f, x, y) ? 'BLOCKED' : 'walkable'}${edited}`];
        if (f.teleport[i]) parts.push('teleport flag');
        const k = tileKey(x, y);
        const door = this.doorsOf(this.current).find((d) => d.tiles.has(k));
        if (door) parts.push(`door ${doorLabel(door)}`);
        if (!this.isEdited(this.current)) {
            const dead = this.infos.get(this.current.toLowerCase())?.unflagged.find((t) => t.x === x && t.y === y);
            if (dead) parts.push(`built-in door to ${dead.to.map} that never fires (no flag)`);
        }
        const arrivals = this.arrivalsHere().filter((a) => a.door.to.x === x && a.door.to.y === y);
        if (arrivals.length) parts.push(`arrival from ${arrivals.map((a) => a.map).join(', ')}`);
        parts.push(...this.rulesAt(x, y));
        if (this.layers.spawns) {
            const sp = this.spawnAt(x, y, false);
            if (sp) parts.push(`spawn ${spawnLabel(sp, this.npcTypes)}`);
        }
        this.setStatus(parts.join('  ·  '));
    }

    private setStatus(text: string): void {
        $('status').textContent = text;
    }
}

const game = new Phaser.Game({
    type: Phaser.AUTO,
    parent: 'game',
    pixelArt: true,
    backgroundColor: '#0b0c0f',
    scale: { mode: Phaser.Scale.RESIZE, width: '100%', height: '100%' },
    scene: [MapEditorScene],
});
(window as any).__mapEditor = game;
