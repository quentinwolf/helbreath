import type { Scene } from 'phaser';
import { convertPixelPosToWorldPos, convertWorldPosToPixelPos, getNextDirection, getDirectionOffset, Direction } from '../../utils/CoordinateUtils';
import { FireInstance } from './FireInstance';
import type { SoundManager } from '../../audio/SoundManager';
import type { Effect } from '../effects/Effect';

/** Wall length: 2 tiles in each direction from center */
const WALL_LENGTH = 2;

export type FireWallConfig = {
    /** Duration in milliseconds each fire instance lasts (default: 30000) */
    duration?: number;
    /** Sound manager for E45 cast sound */
    soundManager?: SoundManager;
    /** Player world X for spatial audio */
    playerWorldX?: number;
    /** Player world Y for spatial audio */
    playerWorldY?: number;
    /** Called when each fire effect is created. Returns onDestroy to remove from effects array. */
    onEffectCreated?: (effect: Effect) => () => void;
};

/**
 * Fire Wall spell. Creates a line of fire tiles from caster toward target, 5 tiles (center + 2 each direction).
 * Plays E45 when cast. Each tile uses Positional Fire 1 effect in a loop.
 */
export class FireWall {
    private instances: FireInstance[] = [];

    constructor(
        scene: Scene,
        originPixelX: number,
        originPixelY: number,
        targetPixelX: number,
        targetPixelY: number,
        config: FireWallConfig
    ) {
        const originWorldX = convertPixelPosToWorldPos(originPixelX);
        const originWorldY = convertPixelPosToWorldPos(originPixelY);
        const targetWorldX = convertPixelPosToWorldPos(targetPixelX);
        const targetWorldY = convertPixelPosToWorldPos(targetPixelY);

        const direction = getNextDirection(
            originWorldX,
            originWorldY,
            targetWorldX,
            targetWorldY
        );
        // The original server lays the wall across the cast direction (perpendicular).
        const [cx, cy] = getDirectionOffset(direction);
        const [rx, ry] = direction === Direction.None ? [1, 0] : [-cy, cx];


        // Create fire instances: center + (1..WALL_LENGTH) in each direction
        const tiles: [number, number][] = [[targetWorldX, targetWorldY]];
        for (let i = 1; i <= WALL_LENGTH; i++) {
            tiles.push([targetWorldX + i * rx, targetWorldY + i * ry]);
            tiles.push([targetWorldX - i * rx, targetWorldY - i * ry]);
        }

        for (const [wx, wy] of tiles) {
            const pixelX = convertWorldPosToPixelPos(wx);
            const pixelY = convertWorldPosToPixelPos(wy);
            const instance = new FireInstance(scene, pixelX, pixelY, {
                duration: config.duration ?? 30000,
                onEffectCreated: config.onEffectCreated,
            });
            this.instances.push(instance);
        }
    }

    public destroy(): void {
        for (const instance of this.instances) {
            instance.destroy();
        }
        this.instances = [];
    }
}
