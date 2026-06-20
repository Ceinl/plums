package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestCommandsOwnSkillsEntries(t *testing.T) {
	commands := (&Plugin{}).Commands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Do == nil {
			t.Fatalf("command %q has nil Do", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"/skills", "Skills list"}
	if len(names) != len(want) {
		t.Fatalf("commands = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("commands = %v, want %v", names, want)
		}
	}
}

func TestExpandSkillMarkers(t *testing.T) {
	got := (&Plugin{}).Expand("/skill demo-skill\ndo the work", []capabilities.Skill{
		{Name: "demo-skill", Content: "## Instructions\nUse demo behavior."},
	})
	for _, want := range []string{
		"Use the `demo-skill` skill",
		"<skill_content name=\"demo-skill\">",
		"Use demo behavior.",
		"User request:\ndo the work",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expanded prompt missing %q:\n%s", want, got)
		}
	}
}

func TestSkillsFindsOpenCodeCompatibleSkill(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	cwd := filepath.Join(dir, "nested", "pkg")
	skillDir := filepath.Join(dir, ".agents", "skills", "demo-skill")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cwd: %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: demo-skill\ndescription: Demonstrate skill loading\n---\n## Body\nFollow these instructions.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	skills, err := (&Plugin{}).Skills(context.Background(), cwd)
	if err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %#v", skills)
	}
	if skills[0].Name != "demo-skill" || skills[0].Description != "Demonstrate skill loading" || !strings.Contains(skills[0].Content, "Follow these instructions") {
		t.Fatalf("unexpected skill: %#v", skills[0])
	}
}
