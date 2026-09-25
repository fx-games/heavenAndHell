package app

import "syscall/js"

// webBridge hands web/index.html's top bar to the game: window.hh.zoom(±1), hh.sound(on), hh.music(on).
// The game decides what they do (RVIP W0); the page only forwards clicks. Then tells the page the game runs.
func webBridge(host *modHost) {
	fn := func(f func(js.Value)) js.Func {
		return js.FuncOf(func(_ js.Value, a []js.Value) any { f(a[0]); return nil })
	}
	js.Global().Set("hh", map[string]any{
		"zoom":  fn(func(v js.Value) { host.UIController.ChangeMapZoom(v.Int()) }),
		"sound": fn(func(v js.Value) { host.UIController.Audio().MuteSound(!v.Bool()) }),
		"music": fn(func(v js.Value) { host.UIController.Audio().MuteMusic(!v.Bool()) }),
	})
	if p := js.Global().Get("parent"); p.Get("hhLoaded").Type() == js.TypeFunction {
		p.Call("hhLoaded")
	}
}
