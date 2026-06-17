package components

// Many components cache resolved ANSI colour strings (theme.Color.Fg()/.Bg())
// in package-level vars for speed. When the active theme palette is swapped
// (e.g. switching to the zen layout's grey palette), those caches must be
// recomputed. Each component registers a refresher; RefreshColors re-runs them.

var colorRefreshers []func()

// registerColorRefresher records f and runs it once to populate initial values.
func registerColorRefresher(f func()) {
	colorRefreshers = append(colorRefreshers, f)
	f()
}

// RefreshColors recomputes every component's cached colours from the current
// theme palette. Call it after theme.Apply.
func RefreshColors() {
	for _, f := range colorRefreshers {
		f()
	}
}
