// Dev-server API for the map editor (client/map-editor.html, docs/map-editor.md).
// Only runs under `npm run dev`; nothing here is part of the game build.
//
//   GET  /__map-editor/state               server maps with their doors, spawns and map config settings,
//                                          walkable.json, client map files
//   GET  /__map-editor/server-map?file=X   a .amd file from the server's map folder (assets/MAPDATA)
//   POST /__map-editor/save                {doors, spawns, settings, walkable}: doors, spawns and settings go into the maps'
//                                          configs (server/internal/mapdata/maps/<map>.txt, through the Go
//                                          tool mapconfigsave); walkable.json is written here
import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const PREFIX = '/__map-editor/';

interface WalkOverrides {
    version: number;
    maps: Record<string, { block?: [number, number][]; open?: [number, number][] }>;
}

export function mapEditorApi(): Plugin {
    let repo = '';
    let exportCache: string | null = null;

    const paths = () => ({
        serverMaps: path.join(repo, 'assets', 'MAPDATA'),
        clientMaps: path.join(repo, 'client', 'public', 'assets', 'maps'),
        walkable: path.join(repo, 'server', 'internal', 'mapdata', 'walkable.json'),
    });

    const goBinary = (): string => {
        if (process.env.GO_BIN) return process.env.GO_BIN;
        const windowsDefault = 'C:\\Program Files\\Go\\bin\\go.exe';
        return process.platform === 'win32' && fs.existsSync(windowsDefault) ? windowsDefault : 'go';
    };

    // The Go tool reads the map configs and resolves the doors exactly as the server does.
    const runExport = (): Promise<string> =>
        new Promise((resolve, reject) => {
            execFile(goBinary(), ['run', './server/tools/teleportexport', '-maps', paths().serverMaps],
                { cwd: repo, maxBuffer: 64 * 1024 * 1024 },
                (err, stdout, stderr) => {
                    if (err) reject(new Error(`teleportexport failed: ${stderr || err.message}`));
                    else resolve(stdout);
                });
        });

    // The Go tool writes doors and spawns into the map configs, changing only the lines it must.
    const runSave = (edits: unknown): Promise<{ written: string[]; errors: string[] }> =>
        new Promise((resolve, reject) => {
            const child = execFile(goBinary(), ['run', './server/tools/mapconfigsave'],
                { cwd: repo, maxBuffer: 16 * 1024 * 1024 },
                (err, stdout, stderr) => {
                    if (err) reject(new Error(`mapconfigsave failed: ${stderr || err.message}`));
                    else resolve(JSON.parse(stdout));
                });
            child.stdin?.end(JSON.stringify(edits));
        });

    const send = (res: ServerResponse, status: number, body: unknown) => {
        res.statusCode = status;
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(body));
    };

    const readBody = (req: IncomingMessage): Promise<string> =>
        new Promise((resolve, reject) => {
            let data = '';
            req.setEncoding('utf8');
            req.on('data', (chunk: string) => { data += chunk; });
            req.on('end', () => resolve(data));
            req.on('error', reject);
        });

    const state = async (refresh: boolean) => {
        if (refresh || exportCache === null) exportCache = await runExport();
        const exported = JSON.parse(exportCache);
        const p = paths();
        const walkable = JSON.parse(fs.readFileSync(p.walkable, 'utf8')) as WalkOverrides;
        const clientMaps = fs.readdirSync(p.clientMaps).filter((f) => f.toLowerCase().endsWith('.amd'));
        return { ...exported, walkable, clientMaps };
    };

    return {
        name: 'hb-map-editor-api',
        apply: 'serve',
        configResolved(config) {
            repo = path.resolve(config.root, '..');
        },
        configureServer(server) {
            server.middlewares.use(async (req, res, next) => {
                if (!req.url?.startsWith(PREFIX)) return next();
                const url = new URL(req.url, 'http://localhost');
                const route = url.pathname.slice(PREFIX.length);
                try {
                    if (route === 'state' && req.method === 'GET') {
                        send(res, 200, await state(url.searchParams.has('refresh')));
                    } else if (route === 'server-map' && req.method === 'GET') {
                        const file = url.searchParams.get('file') ?? '';
                        const dir = paths().serverMaps;
                        const match = fs.readdirSync(dir).find((f) => f.toLowerCase() === file.toLowerCase());
                        if (!match || path.basename(file) !== file) return send(res, 404, { error: `no server map ${file}` });
                        res.setHeader('Content-Type', 'application/octet-stream');
                        res.end(fs.readFileSync(path.join(dir, match)));
                    } else if (route === 'save' && req.method === 'POST') {
                        const body = JSON.parse(await readBody(req)) as {
                            doors?: Record<string, unknown[]>; spawns?: Record<string, unknown[]>; walkable?: WalkOverrides;
                            settings?: Record<string, Record<string, string>>;
                        };
                        if (body.walkable && (body.walkable.version !== 1 || typeof body.walkable.maps !== 'object')) {
                            return send(res, 400, { error: 'walkable.json needs version 1 and maps' });
                        }
                        // Map configs first: the Go tool checks every changed file the way the
                        // server reads it and writes none if one has a problem.
                        let written: string[] = [];
                        if (body.doors || body.spawns || body.settings) {
                            const r = await runSave({ doors: body.doors ?? {}, spawns: body.spawns ?? {}, settings: body.settings ?? {} });
                            if (r.errors.length) return send(res, 200, { ok: false, errors: r.errors, written: [] });
                            written = r.written;
                        }
                        // The game server reloads walkable.json when it changes, so write it in
                        // one step (it never sees half a file).
                        if (body.walkable) writeWhole(paths().walkable, formatWalkable(body.walkable));
                        // Re-run the export: the new files as the server will read them.
                        const after = await state(true);
                        const errors = [...(after.configErrors ?? [])];
                        send(res, 200, {
                            ok: !after.walkError && errors.length === 0,
                            errors,
                            written,
                            walkError: after.walkError ?? '',
                            maps: after.maps,
                        });
                    } else {
                        send(res, 404, { error: `unknown map editor route ${route}` });
                    }
                } catch (err) {
                    send(res, 500, { error: err instanceof Error ? err.message : String(err) });
                }
            });
        },
    };
}

/** Writes a temp file and renames it over the target (plain write if the rename fails). */
function writeWhole(file: string, text: string): void {
    const tmp = `${file}.tmp`;
    fs.writeFileSync(tmp, text);
    try {
        fs.renameSync(tmp, file);
    } catch {
        fs.writeFileSync(file, text);
        fs.rmSync(tmp, { force: true });
    }
}

/** walkable.json with one line per map and list, maps sorted. */
export function formatWalkable(o: WalkOverrides): string {
    const names = Object.keys(o.maps)
        .filter((m) => o.maps[m].block?.length || o.maps[m].open?.length)
        .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    if (names.length === 0) return '{\n  "version": 1,\n  "maps": {}\n}\n';
    const blocks = names.map((name) => {
        const e = o.maps[name];
        const lists = (['block', 'open'] as const)
            .filter((k) => e[k]?.length)
            .map((k) => `      ${JSON.stringify(k)}: ${JSON.stringify(e[k])}`);
        return `    ${JSON.stringify(name)}: {\n${lists.join(',\n')}\n    }`;
    });
    return `{\n  "version": 1,\n  "maps": {\n${blocks.join(',\n')}\n  }\n}\n`;
}
