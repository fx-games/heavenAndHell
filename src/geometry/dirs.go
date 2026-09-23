package geometry

// Dirs lists the 8 directions in circular order, so d-1/d+1 are D1's left[]/right[] and d+4 is opposite[].
var Dirs = [8]Point{{X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}}
