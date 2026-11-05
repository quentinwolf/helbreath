import { TILE_SIZE } from '../Config';

// Tile/pixel/direction helpers used by the spell effects in game/spells.
// Adapted from ErkoKnoll's helbreath-base-game (MIT, Copyright (c) Ion Fraction OÜ).

/** Pixel coordinate -> tile coordinate (32 px tiles). */
export function convertPixelPosToWorldPos(pixelCoordinate: number): number {
    return pixelCoordinate >> 5;
}

/** Tile coordinate -> pixel coordinate of the tile's top-left corner. */
export function convertWorldPosToPixelPos(worldCoordinate: number): number {
    return worldCoordinate << 5;
}

/** Random pixel offset inside a circle of `radiusCells` tiles (uniform over the area). */
export function randomPixelInRadius(radiusCells: number): { dx: number; dy: number } {
    if (radiusCells <= 0) {
        return { dx: 0, dy: 0 };
    }
    const radiusPx = radiusCells * TILE_SIZE;
    const angle = Phaser.Math.FloatBetween(0, 2 * Math.PI);
    const r = Math.sqrt(Phaser.Math.FloatBetween(0, 1)) * radiusPx;
    return { dx: Math.cos(angle) * r, dy: Math.sin(angle) * r };
}

/** The 8 sprite directions, in the order directional effect sheets store them. */
export enum Direction {
    None = -1,
    North = 0,
    NorthEast = 1,
    East = 2,
    SouthEast = 3,
    South = 4,
    SouthWest = 5,
    West = 6,
    NorthWest = 7,
}

const DIRECTION_OFFSET: Record<Direction, [number, number]> = {
    [Direction.North]: [0, -1],
    [Direction.NorthEast]: [1, -1],
    [Direction.East]: [1, 0],
    [Direction.SouthEast]: [1, 1],
    [Direction.South]: [0, 1],
    [Direction.SouthWest]: [-1, 1],
    [Direction.West]: [-1, 0],
    [Direction.NorthWest]: [-1, -1],
    [Direction.None]: [0, 0],
};

/** One-tile [dx, dy] step for a direction. */
export function getDirectionOffset(direction: Direction): [number, number] {
    return DIRECTION_OFFSET[direction] ?? [0, 0];
}

/** Direction of the first step from source to destination (tile coordinates). */
export function getNextDirection(sourceX: number, sourceY: number, destinationX: number, destinationY: number): Direction {
    const x = sourceX - destinationX;
    const y = sourceY - destinationY;
    if (x === 0 && y === 0) return Direction.None;
    if (x === 0) return y > 0 ? Direction.North : Direction.South;
    if (y === 0) return x > 0 ? Direction.West : Direction.East;
    if (x > 0 && y > 0) return Direction.NorthWest;
    if (x < 0 && y > 0) return Direction.NorthEast;
    if (x > 0 && y < 0) return Direction.SouthWest;
    return Direction.SouthEast;
}

/**
 * Direction from a centre point to a target point, in eight 45-degree sectors
 * (East = -22.5..22.5 degrees). Used to pick a projectile's sprite direction.
 */
export function getDirectionFromScreenSector(
    targetPixelX: number,
    targetPixelY: number,
    centerPixelX: number,
    centerPixelY: number,
): Direction {
    const dx = targetPixelX - centerPixelX;
    const dy = targetPixelY - centerPixelY;
    if (dx === 0 && dy === 0) return Direction.None;
    let angle = Phaser.Math.Angle.Between(0, 0, dx, -dy) * (180 / Math.PI);
    if (angle < 0) angle += 360;
    const sectors = [
        Direction.East, Direction.NorthEast, Direction.North, Direction.NorthWest,
        Direction.West, Direction.SouthWest, Direction.South, Direction.SouthEast,
    ];
    return sectors[Math.floor((angle + 22.5) / 45) % 8];
}
