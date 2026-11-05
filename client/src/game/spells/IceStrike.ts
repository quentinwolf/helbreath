import type { Scene } from 'phaser';
import type { SoundManager } from '../../audio/SoundManager';
import type { CameraManager } from '../../utils/CameraShake';
import { EFFECT_ICE_SHARD, EFFECT_SNOW_IMPACT } from '../../constants/EffectSounds';
import { calculateSpatialAudio } from '../../utils/SpatialAudioUtils';
import { convertPixelPosToWorldPos } from '../../utils/CoordinateUtils';
import { PivotSprite, runFrames, rgbBrightness } from './OriginalFx';

// Ice Strike (spell 57) and Mass Ice Strike (72), ported from the original client:
// effect types 157/172 spawn ice shards (types 41-46), which land as impact bursts (50)
// with sparkles (51). Sprites: effect4.spr sheet 2 (shards, frame 48 = shadow),
// sheet 3 (impact), effect5.spr sheet 5 (sparkles; EffectSpr[24+i] = effect5 sheet i+1).

export type IceStrikeConfig = {
    soundManager?: SoundManager;
    playerWorldX?: number;
    playerWorldY?: number;
    cameraManager?: CameraManager;
};

const rand = (n: number) => Math.floor(Math.random() * n);

class IceStrikeCast {
    constructor(private scene: Scene, private config: IceStrikeConfig) {}

    // Every shard plays a sound, as in the original; SoundManager's stack limit keeps
    // the pile-up to two at a time.
    private playSound(key: string, x: number, y: number): void {
        const { soundManager, playerWorldX, playerWorldY } = this.config;
        if (!soundManager || playerWorldX === undefined || playerWorldY === undefined) return;
        soundManager.playOnce(key, undefined, calculateSpatialAudio({
            sourceX: convertPixelPosToWorldPos(x),
            sourceY: convertPixelPosToWorldPos(y),
            listenerX: playerWorldX,
            listenerY: playerWorldY,
        }));
    }

    /**
     * A shard (types 41-44 large, 45-46 small) aimed at pixel (sx, sy). It appears 220 px
     * up, fades in for 7 frames, then drops with growing speed; large ones burst on landing.
     */
    public shard(type: number, sx: number, sy: number, startFrame: number): void {
        const base = 8 * (type - 41);
        let mx = sx;
        let my = sy - 220;
        let v = 20;
        const shard = new PivotSprite(this.scene, 'effect4', 2);
        const shadow = new PivotSprite(this.scene, 'effect4', 2);
        this.playSound(EFFECT_ICE_SHARD, sx, sy);

        runFrames(this.scene, 20, startFrame, 14, (f) => {
            if (f >= 7) {
                mx -= 1;
                my += v;
                v += 1;
            }
            shadow.draw(48, sx, sy, false);
            shadow.sprite.setTint(0x000000).setAlpha(0.5);
            shadow.setEffectDepth(sy, -1);
            if (f < 7) {
                shard.draw(base + f, mx, my, true, rgbBrightness(-8 * (6 - f)));
            } else {
                let g = f - 5;
                if (g >= 8) g = g - 8 + 5;
                shard.draw(base + g, mx, my, false);
            }
            shard.setEffectDepth(sy);
        }, () => {
            shard.destroy();
            shadow.destroy();
            if (type <= 44) {
                this.impact(mx, my);
                this.sparkle(mx + rand(20) - 10, my + rand(20) - 10);
                this.sparkle(mx + rand(20) - 10, my + rand(20) - 10);
            }
        });
    }

    /** Effect 50: icy burst, full for 7 frames then darkening out. Shakes the screen 1 in 4. */
    private impact(x: number, y: number): void {
        const s = new PivotSprite(this.scene, 'effect4', 3);
        s.setEffectDepth(y, 2);
        this.playSound(EFFECT_SNOW_IMPACT, x, y);
        if (rand(4) === 1) this.config.cameraManager?.setCameraShake(x, y);
        runFrames(this.scene, 50, 0, 12, (f) => {
            if (f <= 6) s.draw(f, x, y, true);
            else s.draw(6, x, y, true, rgbBrightness(-5 * (f - 6)));
        }, () => s.destroy());
    }

    /** Effect 51: faint sparkle (EffectSpr[28] = effect5 sheet 5, frames 11-20, 25%). */
    private sparkle(x: number, y: number): void {
        const s = new PivotSprite(this.scene, 'effect5', 5);
        s.setEffectDepth(y, 3);
        runFrames(this.scene, 80, 0, 9, (f) => s.draw(f + 11, x, y, true, 0.25), () => s.destroy());
    }
}

/** Ice Strike at a target anchor (pixels): one icicle on target, 14 around it, 6 small bits. */
export function castIceStrike(scene: Scene, targetPixelX: number, targetPixelY: number, config: IceStrikeConfig): void {
    const cast = new IceStrikeCast(scene, config);
    const tx = targetPixelX;
    const ty = targetPixelY;
    cast.shard(41, tx, ty, 0);
    for (let x = 0; x < 14; x++) {
        cast.shard(41 + rand(3), tx + rand(100) - 50 + 10, ty + rand(90) - 45, -x - 1);
    }
    for (let x = 0; x < 6; x++) {
        cast.shard(45 + rand(2), tx + rand(100) - 50 + 10, ty + rand(90) - 45, -x - 1 - 10);
    }
}

/** Mass Ice Strike: 25 ice chunks (type 44) over a wider area plus 8 small bits. */
export function castMassIceStrike(scene: Scene, targetPixelX: number, targetPixelY: number, config: IceStrikeConfig): void {
    const cast = new IceStrikeCast(scene, config);
    const tx = targetPixelX;
    const ty = targetPixelY;
    cast.shard(44, tx, ty, 0);
    for (let x = 0; x < 8; x++) {
        cast.shard(44, tx + rand(110) - 55 + 10, ty + rand(100) - 50, -rand(3));
    }
    for (let x = 0; x < 16; x++) {
        cast.shard(44, tx + rand(110) - 55 + 10, ty + rand(100) - 50, -x - 1);
    }
    for (let x = 0; x < 8; x++) {
        cast.shard(45 + rand(2), tx + rand(100) - 50 + 10, ty + rand(90) - 45, -x - 1 - 10);
    }
}
