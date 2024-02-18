package dungen

import (
    "math/rand"
    "rpg/geometry"
)

type RogueGenerator struct {
    cols, rows  int
    maxRoomSize geometry.Point
    maxRooms    int
    random      *rand.Rand
}

func NewRogueGenerator(random *rand.Rand) *RogueGenerator {
    cols := 80
    rows := 24
    return &RogueGenerator{
        random: random,
        cols:        cols,
        rows:        rows,
        maxRooms: 9,
        maxRoomSize: geometry.Point{X: cols/3, Y: rows/3},
    }
}

func (r *RogueGenerator) Generate() *DungeonMap {
    emptyMap := NewDungeonMap(r.cols, r.rows)

    for i := 0; i < r.maxRooms; i++ {
        topLeftBox := geometry.Point{
            X: (i%3)*r.maxRoomSize.X + 1,
            Y: (i/3)*r.maxRoomSize.Y,
        }

        /* C version
           rp->r_max.x = rnd(bsze.x - 4) + 4;
           rp->r_max.y = rnd(bsze.y - 4) + 4;
           rp->r_pos.x = top.x + rnd(bsze.x - rp->r_max.x);
           rp->r_pos.y = top.y + rnd(bsze.y - rp->r_max.y);
        */
        var topLeftOfRoom, bottomRightOfRoom geometry.Point
        for topLeftOfRoom.Y == 0 {
            bottomRightOfRoom = geometry.Point{
                X: r.random.Intn(r.maxRoomSize.X-4) + 4,
                Y: r.random.Intn(r.maxRoomSize.Y-4) + 4,
            }

            topLeftOfRoom = geometry.Point{
                X: topLeftBox.X + r.random.Intn(r.maxRoomSize.X - bottomRightOfRoom.X),
                Y: topLeftBox.Y + r.random.Intn(r.maxRoomSize.Y - bottomRightOfRoom.Y),
            }
        }
        newRoom := NewDungeonRoomFromRect(r.random, geometry.NewRect(topLeftOfRoom.X, topLeftOfRoom.Y, bottomRightOfRoom.X, bottomRightOfRoom.Y))
        emptyMap.AddRoomAndSetTiles(newRoom)
    }


    return emptyMap
}
