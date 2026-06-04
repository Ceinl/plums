package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type CommandCondition func(CommandContext) bool

type CommandHandler func(*State, CommandContext) bool

type Command struct {
	Name            string
	Trigger         string
	Conditions      []CommandCondition
	HandlerFunction CommandHandler
}

type CommandContext struct {
	Input        string
	CursorIndex  int
	TriggerStart int
	TriggerEnd   int
	Query        string
}

func (s *State) registeredCommands() []Command {
	return []Command{
		{
			Name:       "SlashCommand",
			Trigger:    "/",
			Conditions: []CommandCondition{CommandAtFirstChar},
			HandlerFunction: func(state *State, ctx CommandContext) bool {
				return state.runSlashCommand(ctx.Query)
			},
		},
		{
			Name:            "FilePathCommand",
			Trigger:         "@",
			Conditions:      []CommandCondition{CommandAfterWhitespace},
			HandlerFunction: nil,
		},
	}
}

func CommandAtFirstChar(ctx CommandContext) bool {
	return ctx.TriggerStart == 0
}

func CommandAfterWhitespace(ctx CommandContext) bool {
	if ctx.TriggerStart <= 0 {
		return false
	}
	previous := []rune(ctx.Input[:ctx.TriggerStart])
	return len(previous) > 0 && unicode.IsSpace(previous[len(previous)-1])
}

func (s *State) commandContext(command Command) (CommandContext, bool) {
	input := s.Editor.GetContent()
	cursorIndex := s.editorCursorIndex()
	if cursorIndex < 0 || cursorIndex > len(input) {
		return CommandContext{}, false
	}
	beforeCursor := input[:cursorIndex]
	triggerStart := strings.LastIndex(beforeCursor, command.Trigger)
	if triggerStart < 0 {
		return CommandContext{}, false
	}
	query := beforeCursor[triggerStart+len(command.Trigger):]
	if strings.ContainsAny(query, "\n\t ") {
		return CommandContext{}, false
	}
	ctx := CommandContext{Input: input, CursorIndex: cursorIndex, TriggerStart: triggerStart, TriggerEnd: triggerStart + len(command.Trigger), Query: query}
	for _, condition := range command.Conditions {
		if !condition(ctx) {
			return CommandContext{}, false
		}
	}
	return ctx, true
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

func (s *State) activeCommand(trigger string) (Command, CommandContext, bool) {
	for _, command := range s.registeredCommands() {
		if command.Trigger != trigger {
			continue
		}
		ctx, ok := s.commandContext(command)
		return command, ctx, ok
	}
	return Command{}, CommandContext{}, false
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
