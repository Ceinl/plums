package config

import "sync"

// A compiled user build registers its Config() through Use so the generated
// main can discover it without an explicit import cycle. The external authoring
// shape is `func Config() config.Config`; the generated main calls Use(Config).
var (
	registeredMu sync.RWMutex
	registered   func() Config
)

// Use registers the config-producing function selected by a compiled user
// build. The generated cmd/plums/main.go calls config.Use(userconfig.Config).
func Use(fn func() Config) {
	if fn == nil {
		panic("config: nil Config function")
	}
	registeredMu.Lock()
	defer registeredMu.Unlock()
	registered = fn
}

// Registered returns the config-producing function previously registered by Use,
// and whether one was registered.
func Registered() (func() Config, bool) {
	registeredMu.RLock()
	defer registeredMu.RUnlock()
	return registered, registered != nil
}
