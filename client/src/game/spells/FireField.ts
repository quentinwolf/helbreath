import type { Scene } from 'phaser';
import { convertWorldPosToPixelPos } from '../../utils/CoordinateUtils';
import { FireInstance } from './FireInstance';
import type { Effect } from '../effects/Effect';

export type FireFieldConfig = {
    /** Milliseconds each fire tile burns (default: 30000) */
    duration?: number;
    /** Half-size in tiles (Magic.cfg value12): 1 = 3x3 */
    radius?: number;
    onEffectCreated?: (effect: Effect) => () => void;
};

/**
 * Fire Field: the same burning tile as Fire Wall, laid as a square around the target
 * (the original server's "field" layout for DEF_DYNAMICOBJECT_FIRE).
 */
export class FireField {
    private instances: FireInstance[] = [];

    constructor(scene: Scene, targetWorldX: number, targetWorldY: number, config: FireFieldConfig) {
        const radius = config.radius ?? 1;
        for (let dy = -radius; dy <= radius; dy++) {
            for (let dx = -radius; dx <= radius; dx++) {
                this.instances.push(new FireInstance(
                    scene,
                    convertWorldPosToPixelPos(targetWorldX + dx),
                    convertWorldPosToPixelPos(targetWorldY + dy),
                    { duration: config.duration, onEffectCreated: config.onEffectCreated },
                ));
            }
        }
    }

    public destroy(): void {
        for (const instance of this.instances) instance.destroy();
        this.instances = [];
    }
}
