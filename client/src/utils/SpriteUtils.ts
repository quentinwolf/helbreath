import type { Scene } from 'phaser';
import { getPivotData } from '../game/assets/HBSprite';
import { DEPTH_MULTIPLIER } from '../Config';
import { convertPixelPosToWorldPos } from './CoordinateUtils';

const LIGHT_TEXTURE_KEY = 'effect-0';
const LIGHT_FRAME = 1;

export function isTreeSpriteIndex(spriteIndex: number): boolean {
    return spriteIndex >= 100 && spriteIndex <= 145;
}

/**
 * The soft light the original client draws under bright effects (effect.spr sheet 0
 * frame 1, additive). Returns undefined until effect.spr is loaded.
 */
export function createLightRadiusOverlay(scene: Scene, pixelX: number, pixelY: number): Phaser.GameObjects.Sprite | undefined {
    if (!scene.textures.exists(LIGHT_TEXTURE_KEY)) return undefined;
    const pivot = getPivotData('effect')?.spriteSheetPivots[0]?.[LIGHT_FRAME];
    const overlay = scene.add.sprite(pixelX + (pivot?.pivotX ?? 0), pixelY + (pivot?.pivotY ?? 0), LIGHT_TEXTURE_KEY, LIGHT_FRAME);
    overlay.setOrigin(0, 0);
    overlay.setBlendMode(Phaser.BlendModes.ADD);
    overlay.setDepth(convertPixelPosToWorldPos(pixelY) * DEPTH_MULTIPLIER - 10);
    return overlay;
}
