import type { Scene } from 'phaser';
import { LABEL_DEPTH } from '../../Config';

/**
 * The name label under a player or NPC, as the original client drew it
 * (DrawObjectName / DrawNpcName): the name in white at the object's tile anchor
 * (its feet), and 14 px below a coloured line saying what it is to you:
 * "(Enemy)" / "Neutral" / "(Friendly)" for NPCs, "Traveller" / "Aresden
 * Combatant" / "Criminal"... for players.
 *
 * Labels draw in their own layer above the map objects (LABEL_DEPTH), so trees
 * and roofs never hide them.
 */
export class EntityLabel {
    private readonly name: Phaser.GameObjects.Text;
    private readonly tag: Phaser.GameObjects.Text;

    constructor(scene: Scene, x: number, y: number, name: string) {
        const style = {
            fontSize: '11px', fontFamily: 'Arial, sans-serif', color: '#ffffff',
            stroke: '#000000', strokeThickness: 2,
        };
        this.name = scene.add.text(x, y, name, style).setVisible(false);
        this.tag = scene.add.text(x, y + 14, '', style).setVisible(false);
        this.setDepth(0);
    }

    get visible(): boolean { return this.name.visible; }

    setVisible(on: boolean): this {
        this.name.setVisible(on);
        this.tag.setVisible(on && this.tag.text !== '');
        return this;
    }

    /** (x, y) is the object's tile anchor; the label hangs from it, left-aligned. */
    setPosition(x: number, y: number): this {
        this.name.setPosition(x, y);
        this.tag.setPosition(x, y + 14);
        return this;
    }

    /** depth is the owner's world depth: it orders labels among themselves,
     * all above the map objects. */
    setDepth(depth: number): this {
        this.name.setDepth(LABEL_DEPTH + depth);
        this.tag.setDepth(LABEL_DEPTH + depth);
        return this;
    }

    setName(text: string): this {
        this.name.setText(text);
        return this;
    }

    /** The second line and its colour ('' hides it). */
    setTag(text: string, colour: string): this {
        this.tag.setText(text).setColor(colour);
        this.tag.setVisible(this.name.visible && text !== '');
        return this;
    }

    destroy(): void {
        this.name.destroy();
        this.tag.destroy();
    }
}
