package console

import (
    "github.com/gdamore/tcell"
    "log"
    "rpg/game"
    "rpg/geometry"
)

func drawText(s tcell.Screen, x1, y1, x2, y2 int, style tcell.Style, text string) {
	row := y1
	col := x1
	for _, r := range []rune(text) {
		s.SetContent(col, row, r, nil, style)
		col++
		if col >= x2 {
			row++
			col = x1
		}
		if row > y2 {
			break
		}
	}
}

type Console struct {
	game       *game.GameInstance
	shouldQuit bool
	lightStyle tcell.Style
	darkStyle  tcell.Style
	screen     tcell.Screen
}

func (c Console) HandleInput() (quit bool) {
	ev := c.screen.PollEvent()
	if ev == nil {
		return true
	}
	switch typedEvent := ev.(type) {
	case *tcell.EventResize:
		c.screen.Sync()
	case *tcell.EventKey:
		typedRune := typedEvent.Rune()
		//mod, key, ch := typedEvent.Modifiers(), typedEvent.Key(), typedRune
		//println(fmt.Sprintf("EventKey Modifiers: %d Key: %d Rune: %d", mod, key, ch))
		if typedRune == 'w' {
			c.game.ManualMovePlayer(geometry.Point{X: 0, Y: -1})
		} else if typedRune == 's' {
			c.game.ManualMovePlayer(geometry.Point{X: 0, Y: 1})
		} else if typedRune == 'a' {
			c.game.ManualMovePlayer(geometry.Point{X: -1, Y: 0})
		} else if typedRune == 'd' {
			c.game.ManualMovePlayer(geometry.Point{X: 1, Y: 0})
		} else if typedRune == 'q' {
			c.shouldQuit = true
			c.screen.Fini()
		}
	}
	return c.shouldQuit
}

func (c Console) putRune(x, y int, style tcell.Style, r rune) {
	c.screen.SetContent(x, y, r, nil, style)
}

func (c Console) Update(dt float64) {
	// Poll event
	gameErr := c.game.Update()
	if gameErr != nil {
		c.shouldQuit = true
	}
}

func (c Console) Draw() {
	screen := c.screen
	player := c.game.GetWorldState().GetPlayer()
	sWidth, sHeight := screen.Size()
	currentMap := c.game.GetWorldState().GetMap()
	startX := 0
	startY := 0
	cellCountX := min(sWidth, currentMap.GetWidth())
	cellCountY := min(sHeight, currentMap.GetHeight())
	if sWidth < currentMap.GetWidth() {
		startX = max(0, player.MapPosition().X - sWidth/2)
	}
	if sHeight < currentMap.GetHeight() {
		startY = max(0, player.MapPosition().Y - sHeight/2)
	}

	screen.Clear()

	for y := 0; y < cellCountY; y++ {
		for x := 0; x < cellCountX; x++ {
			//drawInfos := c.game.StaticMapLookup(x, y, c.game.GetTicks())
			//lastInfo := drawInfos[len(drawInfos)-1]
			//lastInfo.

			mapPos := geometry.Point{X: startX + x, Y: startY + y}
			if !currentMap.Contains(mapPos) || !currentMap.IsExplored(mapPos) {
				continue
			}

			drawStyle := c.darkStyle
			if player.CanSee(mapPos) {
				drawStyle = c.lightStyle
			}

			if mapPos == player.MapPosition() {
				c.putRune(x, y, drawStyle, '@')
			} else if currentMap.IsActorAt(mapPos) {
				actorAt := currentMap.ActorAt(mapPos)
				c.putRune(x, y, drawStyle, rune(actorAt.Name()[0]))
			} else if currentMap.IsTileWalkable(mapPos) {
				c.putRune(x, y, drawStyle, '.')
			} else {
				c.putRune(x, y, drawStyle, '#')
			}
		}
	}

	screen.Show()
}

func NewConsole(game *game.GameInstance) *Console {
	// Initialize screen
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("%+v", err)
	}
	lightStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	darkStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorGray)
	s.SetStyle(lightStyle)
	//s.EnableMouse()
	s.Clear()


	game.SetOnMapChange(func() {
		s.Clear()
		s.Sync()
	})

	return &Console{
		game:       game,
		screen:     s,
		lightStyle: lightStyle,
		darkStyle:  darkStyle,
	}
}
