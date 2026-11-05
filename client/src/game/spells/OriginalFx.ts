import type { Scene } from 'phaser';
import { getPivotData } from '../assets/HBSprite';
import { DEPTH_MULTIPLIER, EFFECT_LAYER_DEPTH } from '../../Config';
import { convertPixelPosToWorldPos } from '../../utils/CoordinateUtils';

// Building blocks for effects ported straight from the original client's
// bAddNewEffect / effect update loop / DrawEffects (Client.cpp).

/**
 * One effect sprite drawn the way the original's Put*Sprite calls do: a given frame of
 * a sheet at a point, offset by that frame's pivot, with additive or normal blending.
 */
export class PivotSprite {
    public readonly sprite: Phaser.GameObjects.Sprite;
    private readonly textureKey: string;
    private readonly pivots: { pivotX: number; pivotY: number; width: number; height: number }[] | undefined;

    constructor(scene: Scene, spriteName: string, sheet: number) {
        this.textureKey = `${spriteName}-${sheet}`;
        this.pivots = getPivotData(spriteName)?.spriteSheetPivots[sheet];
        this.sprite = scene.add.sprite(0, 0, this.textureKey, 0).setOrigin(0, 0).setVisible(false);
    }

    /**
     * Draws `frame` at (x, y). `brightness` 0..1 scales an additive draw (the original's
     * PutTransSpriteRGB with negative values, or Trans25/50/70); additive=false is PutSpriteFast.
     */
    public draw(frame: number, x: number, y: number, additive = true, brightness = 1): void {
        const p = this.pivots?.[frame];
        if (!p || p.width === 0 || brightness <= 0) {
            this.sprite.setVisible(false);
            return;
        }
        this.sprite.setFrame(frame);
        this.sprite.setPosition(x + p.pivotX, y + p.pivotY);
        this.sprite.setBlendMode(additive ? Phaser.BlendModes.ADD : Phaser.BlendModes.NORMAL);
        this.sprite.setAlpha(Math.min(1, brightness));
        this.sprite.setVisible(true);
    }

    /** Effect-layer depth for a point (above all objects; lower on screen draws later). */
    public setEffectDepth(y: number, offset = 0): void {
        this.sprite.setDepth(EFFECT_LAYER_DEPTH + convertPixelPosToWorldPos(y) * DEPTH_MULTIPLIER + offset);
    }

    public destroy(): void {
        this.sprite.destroy();
    }
}

/**
 * Runs an effect's frame counter like the original: the frame advances every
 * `frameTimeMs`, a negative `startFrame` is a delay of that many frames, and `onFrame`
 * is called for each frame up to `maxFrame`. `onEnd` runs when it passes `maxFrame`.
 * Returns a cancel function. Timers die with the scene, so a restart stops it too.
 */
export function runFrames(
    scene: Scene,
    frameTimeMs: number,
    startFrame: number,
    maxFrame: number,
    onFrame: (frame: number) => void,
    onEnd: () => void,
): () => void {
    let frame = startFrame;
    if (frame >= 0) onFrame(frame);
    const timer = scene.time.addEvent({
        delay: frameTimeMs,
        loop: true,
        callback: () => {
            frame++;
            if (frame > maxFrame) {
                timer.remove();
                onEnd();
                return;
            }
            if (frame >= 0) onFrame(frame);
        },
    });
    return () => timer.remove();
}

/** The original's PutTransSpriteRGB darkening: value -32 (or less) is black, 0 is full. */
export function rgbBrightness(value: number): number {
    return Math.max(0, Math.min(1, 1 + value / 32));
}
