#!/usr/bin/env python3
"""Idempotently inserts an 8x8 sprite into a palette-indexed sprite sheet PNG.

Sequence: repalletize the sprite to the sheet's own colors -> scan every existing
slot for a byte-identical tile (so re-inserting the same sprite is a no-op that
returns the same index) -> if none, use the first empty slot -> if the sheet is
full, grow it by one row at the bottom (existing tile ids never change).

Library use:
    from spritesheet import Sheet
    sheet = Sheet.load("classic_roguelike.png")
    index = sheet.insert("candidate.png")   # idempotent: same sprite -> same index
    sheet.save("classic_roguelike.png")

CLI use:
    python3 scripts/spritesheet.py <sheet.png> <sprite.png>   # prints the index, saves in place
"""
import sys

import numpy as np
from PIL import Image

TILE = 8


class Sheet:
    def __init__(self, arr, palette, transparent_index, cols):
        self.arr = arr  # HxW uint8 palette-index array
        self.palette = palette  # Nx3 uint8 RGB
        self.transparent_index = transparent_index
        self.cols = cols

    @classmethod
    def load(cls, path):
        im = Image.open(path)
        if im.mode != "P":
            raise ValueError(f"{path}: expected a palette ('P' mode) image, got {im.mode}")
        palette = np.array(im.getpalette()).reshape(-1, 3)
        arr = np.array(im)
        transparent_index = im.info.get("transparency", 0)
        return cls(arr, palette, transparent_index, im.width // TILE)

    def save(self, path):
        im = Image.fromarray(self.arr, mode="P")
        im.putpalette(self.palette.flatten().tolist())
        im.info["transparency"] = int(self.transparent_index)
        im.save(path)

    @property
    def rows(self):
        return self.arr.shape[0] // TILE

    def _cell(self, index):
        r, c = divmod(index, self.cols)
        return self.arr[r * TILE : r * TILE + TILE, c * TILE : c * TILE + TILE]

    def _set_cell(self, index, block):
        r, c = divmod(index, self.cols)
        self.arr[r * TILE : r * TILE + TILE, c * TILE : c * TILE + TILE] = block

    def _repalletize(self, rgba):
        """Maps an RGBA sprite onto the sheet's palette; background -> transparent index."""
        alpha = rgba[:, :, 3]
        if (alpha < 255).any():
            mask = alpha >= 128
        else:
            corners = [tuple(rgba[0, 0, :3]), tuple(rgba[0, -1, :3]), tuple(rgba[-1, 0, :3]), tuple(rgba[-1, -1, :3])]
            bg = max(set(corners), key=corners.count)
            mask = ~np.all(rgba[:, :, :3] == np.array(bg), axis=-1)

        block = np.full((TILE, TILE), self.transparent_index, dtype=np.uint8)
        if not mask.any():
            return block
        colors = rgba[mask][:, :3].astype(int)
        dists = ((colors[:, None, :] - self.palette[None, :, :]) ** 2).sum(axis=2)
        dists[:, self.transparent_index] = 1 << 30  # a real sprite pixel never becomes the transparent color
        block[mask] = dists.argmin(axis=1)
        return block

    def insert(self, sprite_path):
        """Returns the sprite's tile index, reusing an identical existing tile, an empty slot, or a newly
        grown row, in that order. Growing only appends rows, so every previously returned index stays valid."""
        rgba = np.array(Image.open(sprite_path).convert("RGBA"))
        if rgba.shape[:2] != (TILE, TILE):
            raise ValueError(f"{sprite_path}: expected an {TILE}x{TILE} sprite, got {rgba.shape[1]}x{rgba.shape[0]}")
        block = self._repalletize(rgba)

        total = self.rows * self.cols
        empty = None
        for i in range(total):
            cell = self._cell(i)
            if np.array_equal(cell, block):
                return i
            if empty is None and (cell == self.transparent_index).all():
                empty = i

        if empty is None:
            self.arr = np.pad(self.arr, ((0, TILE), (0, 0)), constant_values=self.transparent_index)
            empty = total  # first slot of the newly appended row

        self._set_cell(empty, block)
        return empty


def _demo():
    """Self-check: insert a sprite, insert it again (must be idempotent), fill the sheet and grow it."""
    transparent = np.array([0, 0, 0], dtype=np.uint8)
    palette = np.vstack([transparent, np.random.default_rng(0).integers(0, 256, size=(9, 3), dtype=np.uint8)])
    arr = np.zeros((TILE * 2, TILE * 2), dtype=np.uint8)  # a 2x2 sheet: 4 slots
    sheet = Sheet(arr, palette, transparent_index=0, cols=2)

    import tempfile, os
    red_sprite = np.zeros((TILE, TILE, 4), dtype=np.uint8)
    red_sprite[2:6, 2:6] = [*palette[1], 255]  # a small solid square in palette color 1
    with tempfile.TemporaryDirectory() as d:
        p = os.path.join(d, "sprite.png")
        Image.fromarray(red_sprite, mode="RGBA").save(p)

        i1 = sheet.insert(p)
        i2 = sheet.insert(p)
        assert i1 == i2, f"re-inserting the same sprite must be idempotent, got {i1} != {i2}"
        assert (sheet.arr != 0).any(), "sprite should have been written into the sheet"

        # fill the remaining 3 slots with distinct sprites, forcing the 4th insert to grow the sheet
        for n in range(1, 4):
            s = np.zeros((TILE, TILE, 4), dtype=np.uint8)
            s[2:6, 2:6] = [*palette[(n % 9) + 1], 255]
            s[0, 0] = [*palette[((n + 3) % 9) + 1], 255]  # extra pixel so each sprite is distinct
            sp = os.path.join(d, f"s{n}.png")
            Image.fromarray(s, mode="RGBA").save(sp)
            idx = sheet.insert(sp)
            assert idx == n, f"expected slot {n}, got {idx}"

        assert sheet.rows == 2, "sheet should still fit in 2 rows (4 slots, 4 sprites)"
        overflow = np.zeros((TILE, TILE, 4), dtype=np.uint8)
        overflow[:] = [*palette[5], 255]
        oi = sheet.insert(os.path.join(d, "sprite.png"))  # already present, must not grow
        assert oi == i1
        op = os.path.join(d, "overflow.png")
        Image.fromarray(overflow, mode="RGBA").save(op)
        oi = sheet.insert(op)
        assert oi == 4, f"5th distinct sprite should land in the newly grown row, got {oi}"
        assert sheet.rows == 3, "sheet should have grown by one row"
    print("demo ok")


if __name__ == "__main__":
    if len(sys.argv) == 1:
        _demo()
    elif len(sys.argv) == 3:
        sheet_path, sprite_path = sys.argv[1], sys.argv[2]
        sheet = Sheet.load(sheet_path)
        index = sheet.insert(sprite_path)
        sheet.save(sheet_path)
        print(index)
    else:
        print(f"usage: {sys.argv[0]} <sheet.png> <sprite.png>   (no args runs the self-check)", file=sys.stderr)
        sys.exit(1)
