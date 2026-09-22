// The engine rolls a lot of dice on math/rand's global source (map transitions, melee, drops). Since Go 1.20
// that source seeds itself from the OS, and rand.Seed is a no-op, so a pinned playthrough seed could not reach
// it: the directive below restores rand.Seed so `playthrough <seed>` really replays. A normal launch never calls
// Seed and keeps its automatic seeding.
//go:debug randseednop=0

package main

import "rpg/src/app"

func main() { app.Main() }
