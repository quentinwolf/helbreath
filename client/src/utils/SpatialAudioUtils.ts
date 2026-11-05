import { SoundManager, type SpatialAudioConfig } from '../audio/SoundManager';

/**
 * Pan and volume for a sound at a tile, heard from the listener's tile.
 * Same falloff as the rest of the game's positional sounds; silent beyond range.
 */
export function calculateSpatialAudio(config: {
    sourceX: number;
    sourceY: number;
    listenerX: number;
    listenerY: number;
}): SpatialAudioConfig {
    return SoundManager.computeSpatialConfig(config.sourceX, config.sourceY, config.listenerX, config.listenerY)
        ?? { pan: 0, distanceVolume: 0 };
}
