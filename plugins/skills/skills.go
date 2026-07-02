// Package skills is the default skills plugin. It discovers opencode/Claude/
// agent-style skills and expands /skill directives before prompts are sent.
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Ceinl/plums/capabilities"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var skillDirectivePattern = regexp.MustCompile(`(?m)^\s*/skill\s+([a-z0-9]+(?:-[a-z0-9]+)*)\s*$`)

type Plugin struct{}

func (*Plugin) Name() string                      { return "skills" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Commands() []capabilities.Command {
	return []capabilities.Command{
		slash("/skills", "Load an opencode skill", func(_ context.Context, ctx capabilities.Ctx) error {
			ctx.View().OpenSkills()
			return nil
		}),
		{
			Name:   "Skills list",
			Detail: "Load an opencode skill for the next prompt",
			Do:     func(_ context.Context, ctx capabilities.Ctx) error { ctx.View().OpenSkills(); return nil },
		},
	}
}

func (*Plugin) Skills(_ context.Context, cwd string) ([]capabilities.Skill, error) {
	return discover(cwd)
}

func (*Plugin) Expand(input string, items []capabilities.Skill) string {
	return expand(input, items)
}

func slash(name, detail string, do func(context.Context, capabilities.Ctx) error) capabilities.Command {
	return capabilities.Command{
		Name:   name,
		Detail: detail,
		Do:     do,
		Title: func(capabilities.CommandState) capabilities.PaletteLabel {
			return capabilities.PaletteLabel{Title: name, Detail: detail, Disabled: true}
		},
	}
}

func discover(cwd string) ([]capabilities.Skill, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	items := []capabilities.Skill{}
	for _, dir := range projectSkillDirs(abs) {
		discovered, err := readSkillsDir(dir, seen)
		if err != nil {
			return nil, err
		}
		items = append(items, discovered...)
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{
			filepath.Join(home, ".config", "opencode", "skills"),
			filepath.Join(home, ".claude", "skills"),
			filepath.Join(home, ".agents", "skills"),
		} {
			discovered, err := readSkillsDir(dir, seen)
			if err != nil {
				return nil, err
			}
			items = append(items, discovered...)
		}
	}
	return items, nil
}

func projectSkillDirs(cwd string) []string {
	dirs := []string{}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		dirs = append(dirs,
			filepath.Join(dir, ".opencode", "skills"),
			filepath.Join(dir, ".claude", "skills"),
			filepath.Join(dir, ".agents", "skills"),
		)
		if hasGitDir(dir) || filepath.Dir(dir) == dir {
			break
		}
	}
	return dirs
}

func hasGitDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func readSkillsDir(dir string, seen map[string]bool) ([]capabilities.Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	items := []capabilities.Skill{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "SKILL.md")
		item, ok, err := readSkillFile(path, entry.Name())
		if err != nil {
			return nil, err
		}
		if !ok || seen[item.Name] {
			continue
		}
		seen[item.Name] = true
		items = append(items, item)
	}
	return items, nil
}

func readSkillFile(path, dirName string) (capabilities.Skill, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return capabilities.Skill{}, false, nil
		}
		return capabilities.Skill{}, false, err
	}
	frontmatter, body, ok := splitFrontmatter(string(data))
	if !ok {
		return capabilities.Skill{}, false, nil
	}
	fields := parseFrontmatterFields(frontmatter)
	name := fields["name"]
	description := fields["description"]
	if !validSkillMetadata(name, description, dirName) {
		return capabilities.Skill{}, false, nil
	}
	return capabilities.Skill{Name: name, Description: description, Path: path, Content: strings.TrimSpace(body)}, true, nil
}

func splitFrontmatter(content string) (frontmatter, body string, ok bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", false
	}
	body = strings.TrimPrefix(rest[end+len("\n---"):], "\n")
	return rest[:end], body, true
}

func parseFrontmatterFields(frontmatter string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(frontmatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" {
			fields[key] = value
		}
	}
	return fields
}

func validSkillMetadata(name, description, dirName string) bool {
	return name == dirName && len(name) >= 1 && len(name) <= 64 && skillNamePattern.MatchString(name) && len(description) >= 1 && len(description) <= 1024
}

func expand(input string, skills []capabilities.Skill) string {
	matches := skillDirectivePattern.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}

	byName := make(map[string]capabilities.Skill, len(skills))
	for _, skill := range skills {
		byName[skill.Name] = skill
	}
	seen := map[string]bool{}
	expanded := []capabilities.Skill{}
	for _, match := range matches {
		name := match[1]
		skill, ok := byName[name]
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		expanded = append(expanded, skill)
	}
	if len(expanded) == 0 {
		return input
	}

	request := strings.TrimSpace(skillDirectivePattern.ReplaceAllString(input, ""))
	var b strings.Builder
	for _, skill := range expanded {
		fmt.Fprintf(&b, "Use the `%s` skill for this request.\n\n<skill_content name=\"%s\">\n%s\n</skill_content>\n\n", skill.Name, skill.Name, skill.Content)
	}
	b.WriteString("User request:\n")
	b.WriteString(request)
	return b.String()
}
