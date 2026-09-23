package dungen

import "rpg/src/rules"

// checkQuests ports DRLG_CheckQuests: the quest set pieces' tiles and spots, at the set piece room (x, y).
func (g *gen) checkQuests(x, y int) {
	for i := range g.quests {
		if !g.available(i) {
			continue
		}
		switch i {
		case rules.QButcher:
			g.rectTrans(2*g.setPiece.x+19, 2*g.setPiece.y+19, 2*g.setPiece.x+26, 2*g.setPiece.y+26)
		case rules.QLtBanner:
			g.drawAlternate("l1/banner1", x, y) // Ogden's sign opens the way down
		case rules.QBlind:
			g.drawAlternate("l2/blind1", x, y) // the halls once the book is read
		case rules.QBlood:
			d := g.loadDun("l2/blood2")
			g.setPiece = rect{x, y, d.w(), d.h()}
			for j := 0; j < d.h(); j++ {
				for i := 0; i < d.w(); i++ {
					if t := d.tile(i, j); t != 0 {
						g.sd(x+i, y+j, t)
					}
				}
			}
		case rules.QWarlord:
			g.drawFilled("l4/warlord2", x, y, 6)
		case rules.QSkelKing:
			g.quests[i].X, g.quests[i].Y = 2*x+28, 2*y+23
		case rules.QSchamb:
			g.drawFilled("l2/bonestr1", x, y, 3)
			g.quests[i].X, g.quests[i].Y = 2*x+22, 2*y+23
		}
	}
}

// drawFilled writes a set piece into dungeon, empty cells as floor.
func (g *gen) drawFilled(name string, x, y int, floor byte) {
	d := g.loadDun(name)
	g.setPiece = rect{x, y, d.w(), d.h()}
	for j := 0; j < d.h(); j++ {
		for i := 0; i < d.w(); i++ {
			t := d.tile(i, j)
			if t == 0 {
				t = floor
			}
			g.sd(x+i, y+j, t)
		}
	}
}

// drawAlternate writes a set piece into pdungeon: the layout an object switches to later (ObjChangeMap).
func (g *gen) drawAlternate(name string, x, y int) {
	d := g.loadDun(name)
	g.setPiece = rect{x, y, d.w(), d.h()}
	for j := 0; j < d.h(); j++ {
		for i := 0; i < d.w(); i++ {
			if t := d.tile(i, j); t != 0 {
				g.setPD(x+i, y+j, t)
			}
		}
	}
}

// resyncQuests ports ResyncQuests for a fresh game: Ogden's sign at qvar1 1 (InitQuests). Later quest states,
// Lazarus' lair included, open the rest on the built level (levels.rec patches). DRLG_MRectTrans is skipped: it only
// marks rooms (dTransVal) for drawing walls see-through and the circling AIs' same-room check, which our grid
// doesn't need.
func (g *gen) resyncQuests() {
	if r := g.setPiece; g.available(rules.QLtBanner) {
		g.objChangeMapResync(r.w+r.x-2, r.h+r.y-2, r.w+r.x+1, r.h+r.y+1)
	}
}

// objChangeMapResync ports ObjChangeMapResync: megatiles (x1, y1)..(x2, y2) switch to pdungeon (ObjSetMini). The
// ObjL1Special/ObjL2Special render flags are skipped.
func (g *gen) objChangeMapResync(x1, y1, x2, y2 int) {
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			t := g.pd(i, j)
			g.sd(i, j, t)
			m := g.mega(int(t) - 1)
			xx, yy := 16+2*i, 16+2*j
			g.piece[xx][yy], g.piece[xx+1][yy], g.piece[xx][yy+1], g.piece[xx+1][yy+1] = m[0], m[1], m[2], m[3]
		}
	}
}

// available ports QuestStatus.
func (g *gen) available(i int) bool {
	q := &g.quests[i]
	return !g.setlevel && g.currlevel == q.Level && q.Active != rules.QuestNotAvail
}
