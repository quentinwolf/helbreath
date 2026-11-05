// Dev-only page (client/spell-preview.html, served by `npm run dev`): plays each spell's
// effect on an empty grid so effects can be checked and tuned without logging in.
import Phaser from 'phaser';
import { ASSET_BASE } from '../env';
import { TILE_SIZE } from '../Config';
import { HBSpriteFile, SpriteType } from '../game/assets/HBSprite';
import { SoundManager } from '../audio/SoundManager';
import { CameraShake } from '../utils/CameraShake';
import { EFFECT_SOUND_FILES } from '../constants/EffectSounds';
import { playSpellEffect } from '../game/spells/SpellEffects';
import { clearSpellUpdates } from '../game/spells/SceneUpdate';
import { GameAsset } from '../game/objects/GameAsset';

// orc: stand-in characters, anchored at the tile corner like GameScene's NPCs
const SPRITES = ['orc', 'effect', 'effect3', 'effect4', 'effect5', 'effect6', 'effect7', 'effect11', 'crueffect1'];

const SPELLS: [number, string][] = [
    [10, 'Energy Bolt'], [20, 'Fire Ball'], [30, 'Fire Strike'], [40, 'Fire Wall'], [41, 'Fire Field'],
    [43, 'Lightning'], [45, 'Chill Wind'], [46, 'Poison Cloud'], [47, 'Triple Energy Bolt'],
    [51, 'Lightning Bolt'], [54, 'Spike Field'], [55, 'Ice Storm'], [57, 'Ice Strike'],
    [60, 'Energy Strike'], [61, 'Mass Fire Strike'], [63, 'Mass Chill Wind'], [64, 'Earthworm Strike'],
    [66, 'Armor Break'], [70, 'Bloody Shock Wave'], [72, 'Mass Ice Strike'], [73, 'Cloud Kill'],
    [74, 'Lightning Strike'], [81, 'Meteor Strike'], [85, 'Blizzard'], [91, 'Mass Blizzard'],
    [93, 'Mass Lightning Strike'], [96, 'Earth Shock Wave'],
];

const COLS = 20;
const ROWS = 15;

class SpellPreviewScene extends Phaser.Scene {
    private soundManager!: SoundManager;
    private shake!: CameraShake;
    private caster = { x: 5, y: 8 };
    private target = { x: 13, y: 7 };
    private targetMarker!: Phaser.GameObjects.Rectangle;
    private ready = false;
    private targetOrc?: GameAsset;

    constructor() {
        super('SpellPreview');
    }

    preload(): void {
        for (const name of SPRITES) this.load.binary(name, `${ASSET_BASE}/assets/sprites/${name}.spr`);
        for (const [key, file] of Object.entries(EFFECT_SOUND_FILES)) this.load.audio(key, `${ASSET_BASE}/assets/sounds/${file}`);
    }

    async create(): Promise<void> {
        this.soundManager = new SoundManager(this);
        // Spell effect sounds (fx-*): at most 2 copies of one sound at a time.
        this.soundManager.setStackLimit('fx-', 2);
        this.shake = new CameraShake(this);
        this.events.once('shutdown', () => clearSpellUpdates(this));

        const g = this.add.graphics().setDepth(-1000);
        for (let y = 0; y < ROWS; y++) {
            for (let x = 0; x < COLS; x++) {
                g.fillStyle((x + y) % 2 ? 0x3a4a2a : 0x34432a);
                g.fillRect(x * TILE_SIZE, y * TILE_SIZE, TILE_SIZE, TILE_SIZE);
            }
        }
        this.add.rectangle(this.caster.x * TILE_SIZE, this.caster.y * TILE_SIZE, TILE_SIZE, TILE_SIZE)
            .setStrokeStyle(2, 0x66aaff).setDepth(-999);
        this.targetMarker = this.add.rectangle(0, 0, TILE_SIZE, TILE_SIZE)
            .setStrokeStyle(2, 0xff6666).setDepth(-999);
        this.moveTarget(this.target.x, this.target.y);
        this.input.on('pointerdown', (p: Phaser.Input.Pointer) => {
            this.moveTarget(Math.floor(p.worldX / TILE_SIZE), Math.floor(p.worldY / TILE_SIZE));
        });

        for (const name of SPRITES) {
            const buffer = this.cache.binary.get(name) as ArrayBuffer | undefined;
            if (buffer) await new HBSpriteFile(name, name === 'orc' ? SpriteType.Monster : SpriteType.Effect).load(this, buffer);
        }
        const orc = (t: { x: number; y: number }, dir: number) => {
            const a = new GameAsset(this, { x: t.x * TILE_SIZE, y: t.y * TILE_SIZE, spriteName: 'orc', spriteSheetIndex: dir, frameRate: 8 });
            a.setDepth(t.y * 100 + 1);
            return a;
        };
        orc(this.caster, 2);
        this.targetOrc = orc(this.target, 6);
        this.ready = true;

        const params = new URLSearchParams(location.search);
        const sheet = params.get('sheet');
        if (sheet) this.showSheet(sheet);
        const auto = Number(params.get('spell'));
        if (auto) this.cast(auto);
        (window as any).__previewReady = true;
    }

    update(): void {
        this.cameras.main.setScroll(0, 0);
        this.shake.apply();
    }

    /** ?sheet=effect4-2: every frame of one sheet on a grid, numbered, pivot at the cell dot. */
    private showSheet(key: string): void {
        const [name, idx] = [key.slice(0, key.lastIndexOf('-')), Number(key.slice(key.lastIndexOf('-') + 1))];
        const tex = this.textures.get(key);
        const count = Object.keys(tex.frames).filter((f) => f !== '__BASE').length;
        this.add.rectangle(0, 0, COLS * TILE_SIZE, ROWS * TILE_SIZE, 0x202020).setOrigin(0).setDepth(9000);
        const cols = 8;
        const cw = (COLS * TILE_SIZE) / cols;
        const ch = Math.max(60, Math.floor((ROWS * TILE_SIZE) / Math.ceil(count / cols)));
        for (let i = 0; i < count; i++) {
            const cx = (i % cols) * cw + cw / 2;
            const cy = Math.floor(i / cols) * ch + ch * 0.6;
            new GameAsset(this, { x: cx, y: cy, spriteName: name, spriteSheetIndex: idx, frameIndex: i }).setDepth(9001);
            this.add.circle(cx, cy, 2, 0xff0000).setDepth(9002);
            this.add.text(cx - cw / 2 + 2, cy - ch * 0.6 + 2, String(i), { fontSize: '10px', color: '#ff0' }).setDepth(9002);
        }
    }

    private moveTarget(x: number, y: number): void {
        this.target = { x, y };
        this.targetMarker.setPosition(x * TILE_SIZE, y * TILE_SIZE);
        this.targetOrc?.setPosition(x * TILE_SIZE, y * TILE_SIZE);
        this.targetOrc?.setDepth(y * 100 + 1);
    }

    public cast(spellId: number): void {
        if (!this.ready) return;
        playSpellEffect({
            scene: this,
            soundManager: this.soundManager,
            camera: this.shake,
            listenerX: this.caster.x,
            listenerY: this.caster.y,
        }, spellId, this.caster.x, this.caster.y, this.target.x, this.target.y);
    }
}

const scene = new SpellPreviewScene();
new Phaser.Game({
    type: Phaser.WEBGL,
    width: COLS * TILE_SIZE,
    height: ROWS * TILE_SIZE,
    parent: 'game',
    pixelArt: true,
    backgroundColor: '#000000',
    scene: [scene],
    preserveDrawingBuffer: true,
} as Phaser.Types.Core.GameConfig);
(window as any).__cast = (id: number) => scene.cast(id);

const list = document.getElementById('spells')!;
for (const [id, name] of SPELLS) {
    const b = document.createElement('button');
    b.textContent = `${name} (${id})`;
    b.onclick = () => scene.cast(id);
    list.appendChild(b);
}
