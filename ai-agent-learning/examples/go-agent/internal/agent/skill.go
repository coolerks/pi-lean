package agent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Path        string
}

// DiscoverSkills loads metadata only. Full SKILL.md content is fetched later,
// which is the progressive-disclosure boundary.
func DiscoverSkills(root string) ([]Skill, error) {
	var skills []Skill
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, description, err := parseSkillFrontmatter(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		skills = append(skills, Skill{Name: name, Description: description, Path: path})
		return nil
	})
	return skills, err
}

func ReadSkill(skill Skill) (string, error) {
	content, err := os.ReadFile(skill.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func parseSkillFrontmatter(content string) (string, string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", "", fmt.Errorf("missing frontmatter")
	}
	values := map[string]string{}
	closed := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			closed = true
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	if !closed {
		return "", "", fmt.Errorf("unterminated frontmatter")
	}
	if values["name"] == "" || values["description"] == "" {
		return "", "", fmt.Errorf("frontmatter requires name and description")
	}
	return values["name"], values["description"], nil
}
