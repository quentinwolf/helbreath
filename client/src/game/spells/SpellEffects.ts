import type { Scene } from 'phaser';
import type { SoundManager } from '../../audio/SoundManager';
import type { CameraManager } from '../../utils/CameraShake';
import { convertWorldPosToPixelPos } from '../../utils/CoordinateUtils';
import { ArmorBreak } from './ArmorBreak';
import { Blizzard, type BlizzardConfig } from './Blizzard';
import type { BlizzardShardConfig } from './BlizzardShard';
import { BloodyShockWave } from './BloodyShockWave';
import { ChillWind } from './ChillWind';
import { EarthShockWave } from './EarthShockWave';
import { EarthwormStrike } from './EarthwormStrike';
import { EnergyBolt } from './EnergyBolt';
import { EnergyStrike } from './EnergyStrike';
import { FireBall } from './FireBall';
import { FireField } from './FireField';
import { FireStrike } from './FireStrike';
import { FireWall } from './FireWall';
import { IceStorm } from './IceStorm';
import { castIceStrike, castMassIceStrike } from './IceStrike';
import { LightningBolt, type LightningBoltConfig } from './LightningBolt';
import { LightningStrike } from './LightningStrike';
import { castLightningArrow, castMagicFlash, castMagicMissile, castMassLightningArrow, castTransportFlash } from './MagicMissile';
import { MassChillWind } from './MassChillWind';
import { MassFireStrike } from './MassFireStrike';
import { MeteorStrike } from './MeteorStrike';
import { castCelebratingLight, castTremor } from './OriginalSpells';
import { PoisonCloud } from './PoisonCloud';
import { SpikeField } from './SpikeField';
import { TripleEnergyBolt } from './TripleEnergyBolt';

// Plays the original-style animation for a cast, keyed by the original spell ID
// (Magic.cfg / SpellEffectEvent.spriteId). The spell classes are adapted from
// ErkoKnoll's helbreath-base-game (MIT, Copyright (c) Ion Fraction OÜ); the ID mapping,
// field sizes and Lightning vs Lightning Bolt split follow the original client/server.

export type SpellEffectContext = {
    scene: Scene;
    soundManager: SoundManager;
    camera: CameraManager;
    /** Local player's tile, for positional sound. */
    listenerX: number;
    listenerY: number;
};

/** Field spells last this long (Magic.cfg LastTime is 30 s for all of them). */
const FIELD_DURATION_MS = 30000;

type Cast = {
    ctx: SpellEffectContext;
    /** Caster and target tile anchors in pixels (tile top-left, like character sprites). */
    ox: number;
    oy: number;
    tx: number;
    ty: number;
    /** Target tile. */
    tileX: number;
    tileY: number;
    common: { soundManager: SoundManager; playerWorldX: number; playerWorldY: number; cameraManager: CameraManager };
};

type SpellEntry = {
    /** Effect .spr files that must be loaded before playing. */
    sprites: string[];
    play: (c: Cast) => void;
};

const boltConfig = (c: Cast, extra?: Partial<LightningBoltConfig>): LightningBoltConfig => ({
    ...c.common,
    ...extra,
});

const blizzardShardConfig = (c: Cast): BlizzardShardConfig => ({
    dropDistanceMin: 300,
    dropDistanceMax: 500,
    dropSpeedMin: 200,
    dropSpeedMax: 500,
    fadeInDuration: 200,
    impactFadeOutDuration: 500,
    impactAnimationSpeed: 15,
    ...c.common,
});

const BLIZZARD: BlizzardConfig = {
    projectileSpeed: 300,
    emissionSteps: 10,
    startRadius: 1,
    endRadius: 3,
    startShards: 0,
    endShards: 10,
};

const SPELLS: Record<number, SpellEntry> = {
    // Magic Missile / Lightning Arrow / Mass Lightning Arrow (ported from the original client)
    0: { sprites: ['effect', 'effect2'], play: (c) => castMagicMissile(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    37: { sprites: ['effect', 'effect2'], play: (c) => castLightningArrow(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    56: { sprites: ['effect', 'effect2'], play: (c) => castMassLightningArrow(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    // Recall, Summon Creature, Invisibility: the original's effect 112 on the target
    ...Object.fromEntries([12, 31, 32].map((id) => [id, { sprites: ['effect7'], play: (c: Cast) => castTransportFlash(c.ctx.scene, c.tx, c.ty, c.common) }])),
    // Create Food, Stamina Drain, Protection From Arrow, Hold Person, Possession, Poison,
    // Protection From Magic, Detect Invisibility, Paralyze, Cure, Confuse Language, Polymorph,
    // Mass Poison, Confusion, Mass Confusion: effect 102 on the target
    ...Object.fromEntries([2, 11, 24, 25, 26, 27, 33, 34, 35, 36, 42, 52, 53, 62, 71].map((id) =>
        [id, { sprites: ['effect'], play: (c: Cast) => castMagicFlash(c.ctx.scene, c.tx, c.ty, c.common) }])),
    // Energy Bolt / Triple Energy Bolt
    10: { sprites: ['effect'], play: (c) => new EnergyBolt(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    47: { sprites: ['effect'], play: (c) => new TripleEnergyBolt(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    // Fire Ball / Fire Strike / Mass Fire Strike
    20: { sprites: ['effect'], play: (c) => new FireBall(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    30: { sprites: ['effect'], play: (c) => new FireStrike(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    61: { sprites: ['effect', 'effect3'], play: (c) => new MassFireStrike(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    // Fire Wall (5-tile line across the cast) / Fire Field (3x3)
    40: { sprites: ['effect'], play: (c) => new FireWall(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { duration: FIELD_DURATION_MS, ...c.common }) },
    41: { sprites: ['effect'], play: (c) => new FireField(c.ctx.scene, c.tileX, c.tileY, { duration: FIELD_DURATION_MS, radius: 1 }) },
    // Lightning: from the sky. Lightning Bolt: from the caster. Lightning Strike: 5 bolts
    // 120 ms apart within a tile of the target (original effect 174).
    43: { sprites: ['effect'], play: (c) => new LightningBolt(c.ctx.scene, 0, 0, c.tx, c.ty, boltConfig(c)) },
    51: { sprites: ['effect'], play: (c) => new LightningBolt(c.ctx.scene, 0, 0, c.tx, c.ty, boltConfig(c, { originPixelX: c.ox, originPixelY: c.oy - 50 })) },
    74: { sprites: ['effect'], play: (c) => new LightningStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { strikeInterval: 120, strikes: 5, radius: 1, lightningBoltConfig: boltConfig(c) }) },
    93: { sprites: ['effect'], play: (c) => new LightningStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { strikeInterval: 50, strikes: 12, radius: 3, lightningBoltConfig: boltConfig(c) }) },
    // Tremor and Celebrating Light (ported from the original client: effects 138 and 114)
    22: { sprites: ['effect2'], play: (c) => castTremor(c.ctx.scene, c.tx, c.ty, c.common) },
    14: { sprites: ['effect6'], play: (c) => castCelebratingLight(c.ctx.scene, c.tx, c.ty, c.common) },
    // Forge's Breath and Ultimate Elixir are Equilibrium spells with no surviving client
    // art: stand-ins in their element (fire 5x5, earth 11x11)
    6: { sprites: ['effect', 'effect3'], play: (c) => new MassFireStrike(c.ctx.scene, c.ox, c.oy, c.tileX, c.tileY, { projectileSpeed: 1500, ...c.common }) },
    5: { sprites: ['effect4'], play: (c) => new PoisonCloud(c.ctx.scene, c.tx, c.ty, { duration: 1500, radius: 3 }) },
    // Divine Revenge: an Equilibrium spell whose client art isn't in any source we have;
    // stand-in: bolts from the sky over its 5x5 area
    78: { sprites: ['effect'], play: (c) => new LightningStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { strikeInterval: 70, strikes: 8, radius: 3, lightningBoltConfig: boltConfig(c) }) },
    // Chill Wind / Mass Chill Wind
    45: { sprites: ['effect4', 'effect'], play: (c) => new ChillWind(c.ctx.scene, c.tx, c.ty, { dropDistance: 200, projectileSpeed: 1500, ...c.common }) },
    63: { sprites: ['effect5'], play: (c) => new MassChillWind(c.ctx.scene, c.tx, c.ty, { dropDistance: 200, projectileSpeed: 1500, ...c.common }) },
    // Poison Cloud (3x3) / Cloud Kill (7x7)
    46: { sprites: ['effect4'], play: (c) => new PoisonCloud(c.ctx.scene, c.tx, c.ty, { duration: FIELD_DURATION_MS, radius: 1 }) },
    73: { sprites: ['effect4'], play: (c) => new PoisonCloud(c.ctx.scene, c.tx, c.ty, { duration: FIELD_DURATION_MS, radius: 3 }) },
    // Spike Field (5x5) / Ice Storm (single whirlwind)
    54: { sprites: ['effect3'], play: (c) => new SpikeField(c.ctx.scene, c.tx, c.ty, { duration: FIELD_DURATION_MS, radius: 2 }) },
    55: { sprites: ['effect3'], play: (c) => new IceStorm(c.ctx.scene, c.tx, c.ty, { duration: FIELD_DURATION_MS }) },
    // Ice Strike / Mass Ice Strike (ported directly from the original client)
    57: { sprites: ['effect4', 'effect5'], play: (c) => castIceStrike(c.ctx.scene, c.tx, c.ty, c.common) },
    72: { sprites: ['effect4', 'effect5'], play: (c) => castMassIceStrike(c.ctx.scene, c.tx, c.ty, c.common) },
    // Energy Strike
    60: {
        sprites: ['effect', 'effect3', 'effect11'],
        play: (c) => new EnergyStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, {
            projectiles: 8, emissionInterval: 80, radius: 2,
            projectileConfig: { projectileSpeed: 2000, ...c.common },
        }),
    },
    // Earthworm Strike
    64: { sprites: ['effect6'], play: (c) => new EarthwormStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    // Armor Break / Mass Armor Break
    66: { sprites: ['effect7'], play: (c) => new ArmorBreak(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    92: { sprites: ['effect7'], play: (c) => new ArmorBreak(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, c.common) },
    // Bloody Shock Wave
    70: { sprites: ['effect4'], play: (c) => new BloodyShockWave(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { duration: 400, emissionInterval: 20, ...c.common }) },
    // Meteor Strike
    81: { sprites: ['crueffect1'], play: (c) => new MeteorStrike(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { projectileSpeed: 1500, ...c.common }) },
    // Blizzard / Mass Blizzard
    85: { sprites: ['effect7'], play: (c) => new Blizzard(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, BLIZZARD, blizzardShardConfig(c)) },
    91: { sprites: ['effect7'], play: (c) => new Blizzard(c.ctx.scene, c.ox, c.oy, c.tx, c.ty, { ...BLIZZARD, endRadius: 6, endShards: 20 }, blizzardShardConfig(c)) },
    // Earth Shock Wave
    96: {
        sprites: ['effect11'],
        play: (c) => new EarthShockWave(c.ctx.scene, c.ox, c.oy, c.tx, c.ty,
            { duration: 400, projectileSpeed: 1500, emissionInterval: 20, immobileEmissionInterval: 100, cameraManager: c.common.cameraManager },
            { soundManager: c.common.soundManager, playerWorldX: c.common.playerWorldX, playerWorldY: c.common.playerWorldY }),
    },
};

/** Effect .spr files a spell's animation needs, or undefined if it has no full animation. */
export function spellEffectSprites(spellId: number): string[] | undefined {
    return SPELLS[spellId]?.sprites;
}

/**
 * Plays the spell's animation from the caster's tile to the target tile.
 * Returns false if this spell has no full animation (caller shows a simple one).
 * The sprites from spellEffectSprites() must already be loaded.
 */
export function playSpellEffect(
    ctx: SpellEffectContext,
    spellId: number,
    casterTileX: number,
    casterTileY: number,
    targetTileX: number,
    targetTileY: number,
): boolean {
    const entry = SPELLS[spellId];
    if (!entry) return false;
    const cast: Cast = {
        ctx,
        ox: convertWorldPosToPixelPos(casterTileX),
        oy: convertWorldPosToPixelPos(casterTileY),
        tx: convertWorldPosToPixelPos(targetTileX),
        ty: convertWorldPosToPixelPos(targetTileY),
        tileX: targetTileX,
        tileY: targetTileY,
        common: {
            soundManager: ctx.soundManager,
            playerWorldX: ctx.listenerX,
            playerWorldY: ctx.listenerY,
            cameraManager: ctx.camera,
        },
    };
    try {
        entry.play(cast);
        return true;
    } catch (err) {
        console.warn(`[SPELL] effect for spell ${spellId} failed:`, err);
        return false;
    }
}
