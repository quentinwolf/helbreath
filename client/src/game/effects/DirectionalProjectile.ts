import type { Scene } from 'phaser';
import { EFFECT_LAYER_DEPTH } from '../../Config';
import { onSceneUpdate, offSceneUpdate } from '../spells/SceneUpdate';
import { AnimationType, GameAsset } from '../objects/GameAsset';
import { convertWorldPosToPixelPos, getDirectionFromScreenSector, Direction } from '../../utils/CoordinateUtils';

export type DirectionalProjectileConfig = {
    /** Projectile speed in pixels per second */
    projectileSpeed: number;
    /** Y offset from origin for projectile start (above player), default 40 */
    originOffsetY?: number;
    /** Sprite name (e.g. 'effect') */
    spriteName: string;
    /** Sprite sheet index */
    spriteSheetIndex: number;
    /** Frames per direction (default: 4) */
    framesPerDirection?: number;
    /** Animation frame rate (default: 20) */
    frameRate?: number;
};

const DEFAULT_ORIGIN_OFFSET_Y = 40;
const DEFAULT_FRAMES_PER_DIRECTION = 4;
const DEFAULT_FRAME_RATE = 20;

/**
 * Base class for directional projectile spells.
 * Moves a directional sprite from origin to target, then invokes onReachDestination.
 * Subclasses implement onReachDestination to spawn effects.
 */
export abstract class DirectionalProjectile {
    protected scene: Scene;
    protected config: DirectionalProjectileConfig;
    protected asset: GameAsset;
    protected originX: number;
    protected originY: number;
    protected destPixelX: number;
    protected destPixelY: number;
    protected totalDistance: number;
    protected traveledDistance: number = 0;
    private updateCallback: (time: number, delta: number) => void;

    constructor(
        scene: Scene,
        originPixelX: number,
        originPixelY: number,
        targetWorldX: number,
        targetWorldY: number,
        config: DirectionalProjectileConfig
    ) {
        this.scene = scene;
        this.config = config;
        const originOffsetY = config.originOffsetY ?? DEFAULT_ORIGIN_OFFSET_Y;
        this.originX = originPixelX;
        this.originY = originPixelY - originOffsetY;
        this.destPixelX = convertWorldPosToPixelPos(targetWorldX);
        this.destPixelY = convertWorldPosToPixelPos(targetWorldY);

        this.totalDistance = Phaser.Math.Distance.Between(
            this.originX,
            this.originY,
            this.destPixelX,
            this.destPixelY
        );

        const direction = this.getProjectileDirection();
        const framesPerDirection = config.framesPerDirection ?? DEFAULT_FRAMES_PER_DIRECTION;
        const frameRate = config.frameRate ?? DEFAULT_FRAME_RATE;

        this.asset = new GameAsset(scene, {
            x: this.originX,
            y: this.originY,
            spriteName: config.spriteName,
            spriteSheetIndex: config.spriteSheetIndex,
            direction,
            framesPerDirection,
            animationType: AnimationType.DirectionalSubFrame,
            frameRate,
        });
        // Fire Ball / Fire Strike flames: additive, like the original.
        this.asset.sprite.setBlendMode(Phaser.BlendModes.ADD);
        this.asset.setDepth(EFFECT_LAYER_DEPTH + 90000);

        this.updateCallback = (_time: number, delta: number) => this.update(delta);
        onSceneUpdate(this.scene, this.updateCallback);
    }

    private getProjectileDirection(): number {
        const direction = getDirectionFromScreenSector(
            this.destPixelX,
            this.destPixelY,
            this.originX,
            this.originY
        );
        return direction === Direction.None ? Direction.East : direction;
    }

    private update(delta: number): void {
        const speedPxPerMs = this.config.projectileSpeed / 1000;
        const moveDistance = speedPxPerMs * delta;
        this.traveledDistance += moveDistance;

        if (this.traveledDistance >= this.totalDistance) {
            this.onReachDestination();
            this.destroy();
            return;
        }

        const progress = this.traveledDistance / this.totalDistance;
        const currentX = this.originX + (this.destPixelX - this.originX) * progress;
        const currentY = this.originY + (this.destPixelY - this.originY) * progress;

        this.asset.setPosition(currentX, currentY);
    }

    /** Called when projectile reaches destination. Subclasses spawn effects here. */
    protected abstract onReachDestination(): void;

    public destroy(): void {
        offSceneUpdate(this.scene, this.updateCallback);
        this.asset.destroy();
    }
}
