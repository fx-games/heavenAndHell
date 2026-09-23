package gui

import (
	"image/color"
	"math"
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/renderer"
	"rpg/src/util"

	"github.com/hajimehoshi/ebiten/v2"
)

// deathScreen is the hero's funeral: the killing blow lands, the world stops, and the screen itself bleeds
// out over deathMenuAt seconds before the game over menu is allowed to appear. Nothing skips it - a character
// with hours in it doesn't get dismissed with a keystroke.
//
// The whole effect is one full-screen shader pass over a snapshot of the last frame (see deathShaderSrc), so
// it costs no art and works on any level, any mod, any resolution.
type deathScreen struct {
	ticks      uint64        // since the hero died
	frame      *ebiten.Image // the last frame of the world, frozen at deathFreezeAt and bled on from then on
	center     [2]float32    // the wound, in 0..1 screen coordinates: the effect radiates from the corpse
	nextBeat   int           // index into deathBeats of the heartbeat still to be heard
	menuOpen   bool          // the ceremony is over; the overlay is now just the backdrop under the menu
	collapseAt float64       // when the hero started coming apart, or -1 while the killing blow is still in the air
	openMenu   func()        // opens the game over menu, once
}

const (
	deathFreezeAt = 2.40 // the world keeps running this long: long enough for the hero to fall and come apart
	deathDrownAt  = 6.40 // the blood has closed over the last of the world
	deathTextAt   = 6.60 // "YOU DIED" starts fading in
	deathMenuAt   = 7.90 // and only now may the menu come up

	// The hero's own death, in seconds from the moment the killing blow lands on screen.
	deathCollapse       = 1.50 // he falls, burns away and leaves a corpse over this long
	deathCollapseLatest = 0.50 // and starts no later than this, so it is always over before the freeze
)

// deathBeats are the hero's last heartbeats, in seconds: six of them, each further apart than the last, the
// last one landing in the dark after there is nothing left to see.
var deathBeats = []float64{0.95, 1.70, 2.65, 3.85, 5.35, 7.10}

// ShowGameOverMenu begins the death ceremony and opens the game over menu when it ends. Coming back here from
// a submenu (the death menu's Back) must not replay the funeral, so only the first call starts it.
func (u *UIController) ShowGameOverMenu() {
	openMenu := func() { u.OpenUncancelableMenu(u.context.StartMenuItems(true)) }
	if u.death != nil {
		openMenu()
		return
	}
	u.death = &deathScreen{center: u.playerScreenUV(), openMenu: openMenu, collapseAt: -1}
	u.context.AudioPlayer.StopMusic() // the ceremony is played out in silence, on a heartbeat
}

// playerScreenUV is where the hero is on screen, as 0..1 of each axis, kept off the very edge so the iris and
// the heartbeat ripple always have somewhere to close in from.
func (u *UIController) playerScreenUV() [2]float32 {
	screen := u.GetScreenSize()
	if screen.X == 0 || screen.Y == 0 {
		return [2]float32{0.5, 0.5}
	}
	pos := u.mapRenderer.MapFloatToScreen(u.context.GameState().GetPlayer().DrawPosition())
	clamp01 := func(v float64) float32 { return float32(min(max(v, 0.15), 0.85)) }
	return [2]float32{clamp01(float64(pos.X) / float64(screen.X)), clamp01(float64(pos.Y) / float64(screen.Y))}
}

func (d *deathScreen) seconds() float64 {
	return float64(d.ticks) / float64(util.SecondsToTicks(1))
}

// beginCollapse starts the hero's own death clock on the first frame the killing blow is actually on screen.
func (d *deathScreen) beginCollapse() {
	if d.collapseAt < 0 {
		d.collapseAt = min(d.seconds(), deathCollapseLatest)
	}
}

// collapseSeconds is how long the hero has been coming apart.
func (d *deathScreen) collapseSeconds() float64 { return d.seconds() - d.collapseAt }

// frozen is true once the world has stopped ticking: from here the snapshot is all that is left of it.
func (d *deathScreen) frozen() bool { return d.seconds() >= deathFreezeAt }

func (u *UIController) updateDeathScreen() {
	d := u.death
	d.ticks++
	t := d.seconds()
	if d.nextBeat < len(deathBeats) && t >= deathBeats[d.nextBeat] {
		d.nextBeat++
		common.PlayUICue("heartbeat")
	}
	if t >= deathMenuAt && !d.menuOpen {
		d.menuOpen = true
		d.openMenu()
	}
}

// ease01 is x clamped to 0..1 with a smoothstep on it - every phase of the ceremony fades in and out on it.
func ease01(x float64) float32 {
	x = min(max(x, 0), 1)
	return float32(x * x * (3 - 2*x))
}

// beatStrength is how hard the hero's heart is hitting right now: a spike on each beat, decaying fast.
func (d *deathScreen) beatStrength(t float64) float32 {
	strength := 0.0
	for _, beat := range deathBeats {
		if t >= beat {
			strength = max(strength, math.Exp(-(t-beat)*8.5))
		}
	}
	return float32(strength)
}

func (u *UIController) drawDeathScreen(screen *ebiten.Image) {
	d := u.death
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w == 0 || h == 0 {
		return
	}
	// Until the world freezes it is still moving, so the snapshot is retaken every frame; after that the one
	// frame the hero died on is what gets bled on for the rest of the ceremony.
	if d.frame == nil || d.frame.Bounds().Dx() != w || d.frame.Bounds().Dy() != h {
		d.frame = ebiten.NewImage(w, h)
		d.frame.DrawImage(screen, nil)
	} else if !d.frozen() {
		d.frame.DrawImage(screen, nil)
	}

	t := d.seconds()
	beat := d.beatStrength(t)
	flood := ease01((t - 3.6) / 2.8) // squared below: the pool creeps, then surges over the last of the world
	pixels := float32(h) / 720.0     // so the shake and the colour fringe are the same size at any resolution
	// Deterministic jitter: a real RNG here would be a second source of randomness in a game that has one.
	shake := (16*float32(math.Exp(-t*4.5)) + 5*beat) * pixels

	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = d.frame
	op.Uniforms = map[string]any{
		"Center":    []float32{d.center[0], d.center[1]},
		"Shake":     []float32{shake * float32(math.Sin(t*47.3)), shake * float32(math.Sin(t*61.7+1.7))},
		"Split":     (9*float32(math.Exp(-t*3.2)) + 4*beat) * pixels,
		"Flash":     0.75 * float32(math.Exp(-t*16)), // the blow itself, white-hot for a quarter of a second
		"Zoom":      1 + 0.07*ease01(t/deathMenuAt) + 0.05*ease01(t/1.2) + 0.03*beat,
		"Beat":      beat,
		"Desat":     ease01((t - 0.6) / 2.6),
		"Sepia":     0.9 * ease01((t-1.0)/3.5),
		"Darken":    0.45 * ease01((t-3.2)/3.0),
		"Vignette":  1.7 * ease01((t-0.8)/4.8),
		"Iris":      1.7 - 1.7*ease01((t-4.0)/2.4), // closes over the last of the world at ~6.4s
		"Pool":      1.25 * flood * flood,          // rises past the top of the screen, so it really does drown
		"Submerge":  ease01((t - deathDrownAt) / 0.9),
		"BloodTime": float32(max(t-deathBeats[0], 0)), // the glass gets hit on the first heartbeat, not before
	}
	screen.DrawRectShader(w, h, deathShader, op)

	u.drawDeathText(ease01((t - deathTextAt) / 1.6))
}

// drawDeathText carves YOU DIED into the blood, sized to the screen so it reads the same in a window or
// fullscreen, and set high enough to leave the menu that follows it the middle of the screen.
func (u *UIController) drawDeathText(alpha float32) {
	if alpha <= 0 {
		return
	}
	const text = "YOU DIED"
	screen := u.GetScreenSize()
	textW, textH := u.tileRenderer.MeasureString(text)
	if textW <= 0 {
		return
	}
	scale := float64(screen.X) * 0.45 / textW
	pos := geometry.PointF{
		X: (float64(screen.X) - textW*scale) / 2,
		Y: float64(screen.Y)*0.30 + textH*scale/2,
	}
	// Drawn twice: a wide dark bloom underneath, then the letters, so they glow out of the dark instead of
	// sitting flat on it.
	bloom := color.RGBA{R: uint8(90 * alpha), A: uint8(255 * alpha)}
	letters := color.RGBA{R: uint8(196 * alpha), G: uint8(18 * alpha), B: uint8(18 * alpha), A: uint8(255 * alpha)}
	for _, offset := range []geometry.PointF{{X: -2, Y: -2}, {X: 2, Y: -2}, {X: -2, Y: 2}, {X: 2, Y: 2}} {
		u.tileRenderer.DrawTTFOnScreenScaled(pos.X+offset.X, pos.Y+offset.Y, text, bloom, scale)
	}
	u.tileRenderer.DrawTTFOnScreenScaled(pos.X, pos.Y, text, letters, scale)
}

// drawDyingHero is the hero's death up close, in place of the single-frame swap to a corpse tile that it
// replaces - at 8 pixels a tile that swap is over before the player's eye has even moved. He flashes white,
// shakes, then falls in on himself and burns away from the top down while the corpse rises back through the
// same ragged edge from the bottom up, in a pool of blood spreading out from under him.
func (u *UIController) drawDyingHero(hero *common.Actor, atlas renderer.TextureAtlas, lit color.Color) {
	d := u.death
	flipped := hero.FlipX() && u.configuration.FlipActorsHorizontally
	screenPos := u.mapRenderer.MapFloatToScreen(hero.DrawPosition())
	x, y := float64(screenPos.X), float64(screenPos.Y)
	if hero.VisualsPending() { // the blow that killed him has not reached him on screen yet: he is still alive
		u.tileRenderer.DrawDefaultScaleTileWithFlip(x, y, atlas, u.getActorIcon(hero, u.worldTicks), lit, flipped)
		return
	}
	corpse := u.getActorIcon(hero, u.worldTicks) // the blow has landed, so this now answers with the corpse
	d.beginCollapse()
	c := d.collapseSeconds()
	tile := u.mapRenderer.GetScaledTileSize()

	// The pool goes down first, under everything else, and keeps spreading after he is gone.
	if spread := ease01((c - 0.30) / 1.60); spread > 0 {
		const poolTiles = 2.8 // wide enough to read at all, which one 8x8 tile of blood would not be
		u.tileRenderer.DrawScaledTileWithShader(
			x-tile.X*(poolTiles-1)/2, y-tile.Y*(poolTiles-1)/2,
			atlas, corpse, geometry.PointF{X: poolTiles, Y: poolTiles}, lit, false,
			bloodPoolShader, map[string]any{"Spread": spread})
	}

	// The body: white-hot and shaking on the blow, then squashing down and burning away.
	fall := float64(ease01((c - 0.12) / 0.70))
	if burn := 1.15 * ease01((c-0.35)/0.85); burn < 1.15 {
		scale := geometry.PointF{X: 1 + 0.45*fall, Y: 1 - 0.68*fall}
		jitter := math.Exp(-c*11) * 2.5 // deterministic: the game has one RNG and this is not it
		u.tileRenderer.DrawScaledTileWithShader(
			x+tile.X*(1-scale.X)/2+math.Sin(c*97)*jitter,
			y+tile.Y*(1-scale.Y)+math.Sin(c*83+1.1)*jitter*0.6,
			atlas, u.livingHeroIcon, scale, lit, flipped,
			dissolveShader, map[string]any{"Burn": burn, "Glow": float32(math.Exp(-c * 9))})
	}

	// And the corpse, rising back through the same edge once the body is most of the way gone.
	if rise := ease01((c - 0.60) / 0.80); rise > 0 {
		u.tileRenderer.DrawScaledTileWithShader(x, y, atlas, corpse, geometry.PointF{X: 1, Y: 1}, lit, flipped,
			dissolveShader, map[string]any{"Burn": 1.15 * (1 - rise), "Glow": float32(0)})
	}
}

// dissolveShaderSrc eats a sprite away from the top down along a ragged line that bleeds where it passes, and
// run backwards (Burn falling to 0) puts one back together from the bottom up.
const dissolveShaderSrc = `//kage:unit pixels

package main

var Tint vec4
var Burn float // 0 keeps the whole sprite, 1.15 has eaten all of it
var Glow float // white hot, for the instant the blow lands

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	src := imageSrc0UnsafeAt(srcPos)
	local := (srcPos - imageSrc0Origin()) / imageSrc0Size()

	// Three harmonics that do not share a period: a straight line would read as a wipe, not as coming apart.
	edge := Burn + (sin(local.x*23.0)*0.5+sin(local.x*47.0+1.7)*0.3+sin(local.x*11.0-0.6)*0.2)*0.10
	kept := smoothstep(edge-0.10, edge, local.y)
	rim := clamp(1.0-(local.y-edge)/0.22, 0.0, 1.0) * step(edge, local.y) * smoothstep(0.0, 0.12, Burn)

	lit := src * Tint
	out := lit.rgb * kept
	out = mix(out, vec3(1.0, 0.95, 0.88)*src.a, Glow)  // struck white hot
	out += vec3(0.95, 0.07, 0.07) * rim * kept * src.a // and bleeding where the edge has passed
	return vec4(out, src.a*kept)
}
`

// bloodPoolShaderSrc draws a ragged, flattened pool of blood across the whole quad it is given, ignoring that
// quad's texture: it needs the geometry, not the sprite. Spread runs 0..1 as the pool grows.
const bloodPoolShaderSrc = `//kage:unit pixels

package main

var Tint vec4
var Spread float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	local := (srcPos - imageSrc0Origin()) / imageSrc0Size()
	d := (local - vec2(0.5, 0.58)) * vec2(1.0, 1.55) // wider than it is deep, like liquid on a floor
	angle := atan2(d.y, d.x)
	ragged := 1.0 + 0.18*sin(angle*3.0+0.7) + 0.11*sin(angle*5.0-1.9) + 0.07*sin(angle*9.0+2.4)
	r := length(d) * ragged / max(Spread*0.44, 0.001)

	col := mix(vec3(0.10, 0.005, 0.009), vec3(0.27, 0.015, 0.020), smoothstep(0.15, 1.0, r))
	col += vec3(0.55, 0.09, 0.09) * clamp(1.0-abs(r-0.93)/0.09, 0.0, 1.0) * 0.35 // the wet rim, and only the rim
	col *= mix(0.35, 1.0, dot(Tint.rgb, vec3(0.333, 0.333, 0.333)))              // it lies in the room's light

	a := (1.0 - smoothstep(0.90, 1.0, r)) * Tint.a
	return vec4(col*a, a)
}
`

var (
	dissolveShader  = mustCompileShader(dissolveShaderSrc)
	bloodPoolShader = mustCompileShader(bloodPoolShaderSrc)
)

// deathShaderSrc bleeds on a snapshot of the world. Everything in it is procedural and branchless: the
// spatter cells, the rivulets running down the glass, the pool rising from the bottom edge and the ragged
// iris closing in from the corners are four masks, maxed together into one blood coverage value, which then
// stains the image where it is thin and replaces it where it is thick.
//
// The pixel-unit directive is required: in Kage's default texel mode srcPos is normalized against the
// internal GPU atlas, so the 0..1 screen coordinates below would be meaningless.
const deathShaderSrc = `//kage:unit pixels

package main

var Center vec2
var Shake vec2
var Split float
var Flash float
var Zoom float
var Beat float
var Desat float
var Sepia float
var Darken float
var Vignette float
var Iris float
var Pool float
var Submerge float
var BloodTime float

func hash12(p vec2) float {
	p3 := fract(vec3(p.x, p.y, p.x) * 0.1031)
	p3 += dot(p3, p3.yzx+33.33)
	return fract((p3.x + p3.y) * p3.z)
}

// spatter is what the killing blow throws onto the glass: one droplet per cell of a grid, each landing at
// its own moment and never leaving. The radius runs off the cube of a random number, so most cells get fine
// mist and only a few get the fat drops - an even spread of equal circles reads as polka dots, not gore.
func spatter(uv vec2, t float, cells vec2, seed float, maxRadius float, strength float) float {
	grid := uv*cells + seed
	id := floor(grid)
	f := fract(grid)
	r1 := hash12(id)
	r2 := hash12(id + 31.7)
	r3 := hash12(id + 91.3)

	born := 0.02 + r1*2.6
	age := max(t-born, 0.0) * step(born, t)
	radius := max((0.06+maxRadius*r2*r2*r2)*min(1.0, age*8.0), 0.001)

	// Thrown drops are elongated along the way they flew, not round, and their outline is broken up by three
	// harmonics that do not share a period - one frequency alone just scallops the edge into a cog.
	d := f - vec2(0.15+r2*0.7, 0.15+r3*0.7)
	fly := r1 * 6.28318
	d = vec2(d.x*cos(fly)-d.y*sin(fly), d.x*sin(fly)+d.y*cos(fly))
	d.y /= 1.0 + r3*1.7
	angle := atan2(d.y, d.x)
	ragged := 1.0 + 0.09*sin(angle*3.0+r1*37.0) + 0.06*sin(angle*7.0-r3*23.0) + 0.04*sin(angle*13.0+r2*51.0)
	// A drop is thickest in the middle and thins to a stain at its edge; a flat disc of one value reads as
	// a paper cutout stuck on the screen.
	dome := 1.0 - smoothstep(0.25, 1.0, length(d)*ragged/radius)
	return dome * (0.55 + 0.75*dome) * strength * (0.75 + r3*0.5)
}

// rivulets are the runs of blood crawling down the glass: per column, a trail that starts at its own moment,
// runs at its own speed, wanders as it goes and carries a fat bead at its head. Both ends are soft - a hard
// cut at the head is a bar, not a run.
func rivulets(uv vec2, t float, columns float, seed float) float {
	col := floor(uv.x*columns) + seed
	f := fract(uv.x * columns)
	r1 := hash12(vec2(col, 3.0))
	r2 := hash12(vec2(col, 17.0))
	r3 := hash12(vec2(col, 29.0))

	start := 0.20 + r1*3.6
	head := (t - start) * (0.13 + r2*0.26)
	y := uv.y - r3*0.45 // trails start where the blood hit, not all from the top edge
	along := clamp(y/max(head, 0.001), 0.0, 1.0)

	width := (0.06 + r2*0.14) * (1.0 - 0.55*along)
	width += (0.04 + r1*0.06) * smoothstep(0.86, 1.0, along) // the bead at the head of the run
	center := 0.5 + (r3-0.5)*0.5 + sin(y*14.0+r1*30.0)*0.05

	body := 1.0 - smoothstep(width*0.5, width, abs(f-center))
	body *= smoothstep(0.0, 0.035, y)                   // it wells up at the source instead of starting square
	body *= 1.0 - smoothstep(head-0.02, head+0.008, y)  // and the head is a bead, not a cut
	return body * step(0.0, head) * (0.9 + r1*0.5)
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	uv := (srcPos - origin) / size
	aspect := size.x / size.y

	// --- the lens: a slow push in toward the corpse, a ripple on every heartbeat, and the kick of the blow
	toCenter := (uv - Center) * vec2(aspect, 1.0)
	radius := length(toCenter)
	ripple := sin(radius*22.0-Beat*12.0) * Beat * 0.02 * smoothstep(0.9, 0.0, radius)
	p := clamp(Center+(uv-Center)/Zoom+ripple+Shake/size, 0.004, 0.996)

	// --- the blow splits the colour channels apart, radially, and they crawl back together
	fringe := normalize(toCenter+vec2(0.0001, 0.0001)) * Split / size
	lit := imageSrc0At(origin + p*size)
	lit.r = imageSrc0At(origin + clamp(p+fringe, 0.004, 0.996)*size).r
	lit.b = imageSrc0At(origin + clamp(p-fringe, 0.004, 0.996)*size).b

	// --- the grade: the colour drains out of the world, what is left goes the colour of dried blood
	luma := dot(lit.rgb, vec3(0.299, 0.587, 0.114))
	graded := mix(lit.rgb, vec3(luma), Desat)
	graded = mix(graded, vec3(luma*1.15, luma*0.32, luma*0.30), Sepia)
	graded *= 1.0 + 0.5*Beat // every beat drives one last surge of blood through the hero's eyes
	graded += vec3(0.85, 0.05, 0.05) * Beat * smoothstep(0.8, 0.0, radius) * 0.9
	graded *= clamp(1.0-radius*radius*Vignette, 0.0, 1.0) * (1.0 - Darken)

	// --- the blood: spatter, runs, the rising pool, and the ragged iris closing in from the edges
	blood := spatter(uv, BloodTime, vec2(6.0, 4.0), 0.0, 0.28, 1.25) // the fat drops
	blood = max(blood, spatter(uv, BloodTime, vec2(17.0, 11.0), 13.0, 0.20, 0.55)) // and the mist between them
	blood = max(blood, rivulets(uv, BloodTime, 9.0, 0.0))
	blood = max(blood, rivulets(uv, BloodTime, 17.0, 57.0))

	surface := 1.0 - Pool + sin(uv.x*13.0+BloodTime*1.7)*0.012 + sin(uv.x*31.0-BloodTime*2.3)*0.006
	depth := smoothstep(surface-0.006, surface+0.006, uv.y) * (1.0 + max(uv.y-surface, 0.0)*1.6)
	blood = max(blood, depth) // deeper in the pool is thicker, and thicker is darker

	angle := atan2(toCenter.y, toCenter.x)
	wobble := 0.06*sin(angle*7.0+1.3) + 0.04*sin(angle*13.0-2.1) + 0.03*sin(angle*23.0+BloodTime*0.4)
	blood = max(blood, clamp((radius-(Iris+wobble))/0.10, 0.0, 1.0))

	// Thin blood is light coming through it and stains what is behind; thick blood is opaque; the band
	// between the two is the wet edge, and it catches the light.
	stained := graded * mix(vec3(1.0), vec3(0.45, 0.04, 0.04), clamp(blood, 0.0, 1.0))
	bloodColor := mix(vec3(0.36, 0.022, 0.028), vec3(0.15, 0.008, 0.012), smoothstep(0.75, 1.7, blood))
	out := mix(stained, bloodColor, smoothstep(0.30, 0.75, blood))
	rim := smoothstep(0.24, 0.42, blood) * (1.0 - smoothstep(0.42, 0.62, blood))
	out += vec3(0.50, 0.08, 0.08) * rim * 0.45 // the wet edge of a run, and the meniscus of the pool

	out = mix(out, vec3(1.0, 0.28, 0.22), Flash)      // the blow
	out = mix(out, vec3(0.035, 0.0, 0.004), Submerge) // and then there is nothing left to see
	return vec4(out, 1.0)
}
`

var deathShader = mustCompileShader(deathShaderSrc)
