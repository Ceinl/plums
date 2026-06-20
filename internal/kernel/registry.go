package kernel

import (
	"fmt"
	"sort"

	"github.com/Ceinl/plums/capabilities"
)

type Entry struct {
	Kind  capabilities.RegistryKind
	Name  string
	Owner string
	Value any
}

type Shadow struct {
	Kind          capabilities.RegistryKind
	Name          string
	PreviousOwner string
	NewOwner      string
}

type Registry struct {
	entries map[capabilities.RegistryKind]map[string]Entry
	order   []capabilities.RegistryKey
	shadows []Shadow
	logf    func(string, ...any)
}

func NewRegistry(logf func(string, ...any)) *Registry {
	return &Registry{
		entries: make(map[capabilities.RegistryKind]map[string]Entry),
		logf:    logf,
	}
}

func (r *Registry) Register(kind capabilities.RegistryKind, name string, value any, owner string) error {
	if kind == "" {
		return fmt.Errorf("registry: empty kind")
	}
	if name == "" {
		return fmt.Errorf("registry: empty name for %s", kind)
	}
	if owner == "" {
		owner = "unknown"
	}
	byName := r.entries[kind]
	if byName == nil {
		byName = make(map[string]Entry)
		r.entries[kind] = byName
	}
	if previous, ok := byName[name]; ok {
		shadow := Shadow{Kind: kind, Name: name, PreviousOwner: previous.Owner, NewOwner: owner}
		r.shadows = append(r.shadows, shadow)
		if r.logf != nil {
			r.logf("registry: %s.%s from %s overridden by %s", kind, name, previous.Owner, owner)
		}
	} else {
		r.order = append(r.order, capabilities.RegistryKey{Kind: kind, Name: name})
	}
	byName[name] = Entry{Kind: kind, Name: name, Owner: owner, Value: value}
	return nil
}

func (r *Registry) Disable(keys []capabilities.RegistryKey) {
	for _, key := range keys {
		if key.Kind == "" || key.Name == "" {
			continue
		}
		if byName := r.entries[key.Kind]; byName != nil {
			delete(byName, key.Name)
		}
	}
}

func (r *Registry) Entry(kind capabilities.RegistryKind, name string) (Entry, bool) {
	byName := r.entries[kind]
	if byName == nil {
		return Entry{}, false
	}
	entry, ok := byName[name]
	return entry, ok
}

func (r *Registry) Entries(kind capabilities.RegistryKind) []Entry {
	byName := r.entries[kind]
	if byName == nil {
		return nil
	}
	entries := make([]Entry, 0, len(byName))
	for _, entry := range byName {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func (r *Registry) EntriesInRegistrationOrder(kind capabilities.RegistryKind) []Entry {
	entries := make([]Entry, 0, len(r.entries[kind]))
	seen := make(map[string]bool)
	for _, key := range r.order {
		if key.Kind != kind || seen[key.Name] {
			continue
		}
		entry, ok := r.Entry(key.Kind, key.Name)
		if !ok {
			continue
		}
		entries = append(entries, entry)
		seen[key.Name] = true
	}
	return entries
}

func (r *Registry) Shadows() []Shadow {
	return append([]Shadow(nil), r.shadows...)
}
