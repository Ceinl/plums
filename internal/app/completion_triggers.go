package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// triggerContext locates a completion trigger ("/" or "@") relative to the
// editor cursor and the text typed after it. It is the activation gate the
// editor's "/" and "@" dropdowns share (the completion candidates themselves
// come from the registered Completion sources). It is NOT the command-execution
// path — slash commands run through runSlashCommand against the registered
// command set.
type triggerContext struct {
	Input        string
	CursorIndex  int
	TriggerStart int
	TriggerEnd   int
	Query        string
}

// triggerCondition gates whether a trigger at a given position is active (e.g.
// "/" only at the first char; "@" only after whitespace or line start).
type triggerCondition func(triggerContext) bool

func triggerAtFirstChar(ctx triggerContext) bool {
	return ctx.TriggerStart == 0
}

func triggerAfterWhitespaceOrLineStart(ctx triggerContext) bool {
	if ctx.TriggerStart == 0 {
		return true
	}
	if ctx.TriggerStart < 0 {
		return false
	}
	previous := []rune(ctx.Input[:ctx.TriggerStart])
	return len(previous) > 0 && unicode.IsSpace(previous[len(previous)-1])
}

// triggerConditionFor returns the activation condition for a trigger string.
func triggerConditionFor(trigger string) triggerCondition {
	switch trigger {
	case "/":
		return triggerAtFirstChar
	case "@":
		return triggerAfterWhitespaceOrLineStart
	default:
		return nil
	}
}

func (s *State) triggerContext(trigger string) (triggerContext, bool) {
	condition := triggerConditionFor(trigger)
	if condition == nil {
		return triggerContext{}, false
	}
	input := s.Editor.GetContent()
	cursorIndex := s.editorCursorIndex()
	if cursorIndex < 0 || cursorIndex > len(input) {
		return triggerContext{}, false
	}
	beforeCursor := input[:cursorIndex]
	triggerStart := strings.LastIndex(beforeCursor, trigger)
	if triggerStart < 0 {
		return triggerContext{}, false
	}
	query := beforeCursor[triggerStart+len(trigger):]
	if strings.ContainsAny(query, "\n\t ") {
		return triggerContext{}, false
	}
	ctx := triggerContext{Input: input, CursorIndex: cursorIndex, TriggerStart: triggerStart, TriggerEnd: triggerStart + len(trigger), Query: query}
	if !condition(ctx) {
		return triggerContext{}, false
	}
	return ctx, true
}

// activeCommand returns the active trigger context for the given trigger, if the
// cursor is currently inside one. It powers the editor dropdowns' activation.
func (s *State) activeCommand(trigger string) (triggerContext, bool) {
	return s.triggerContext(trigger)
}

func (s *State) editorCursorIndex() int {
	idx := 0
	for row := 0; row < s.Editor.Cursor.Pos.Row && row < len(s.Editor.Content); row++ {
		idx += len(string(s.Editor.Content[row])) + 1
	}
	if s.Editor.Cursor.Pos.Row >= len(s.Editor.Content) {
		return idx
	}
	col := s.Editor.Cursor.Pos.Col
	if col > len(s.Editor.Content[s.Editor.Cursor.Pos.Row]) {
		col = len(s.Editor.Content[s.Editor.Cursor.Pos.Row])
	}
	idx += len(string(s.Editor.Content[s.Editor.Cursor.Pos.Row][:col]))
	return idx
}

func projectFilePaths(root string, limit int) []string {
	if root == "" {
		root = "."
	}
	ignoredDirs := map[string]bool{".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true, ".cache": true}
	paths := make([]string, 0, limit)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		name := d.Name()
		if d.IsDir() && ignoredDirs[name] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if len(paths) >= limit {
			return filepath.SkipAll
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(paths)
	return paths
}
