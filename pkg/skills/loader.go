package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
	"gopkg.in/yaml.v3"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/logger"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

type SkillMetadata struct {
	Name          string   `json:"name" yaml:"name"`
	Description   string   `json:"description" yaml:"description"`
	Arguments     []string `json:"arguments,omitempty" yaml:"arguments"`
	ArgumentHint  string   `json:"argument-hint,omitempty" yaml:"argument-hint"`
	Context       string   `json:"context,omitempty" yaml:"context"`
	AgentType     string   `json:"agent_type,omitempty" yaml:"agent_type"`
	AllowedTools  []string `json:"allowed_tools,omitempty" yaml:"allowed_tools"`
	UserInvocable bool     `json:"user-invocable" yaml:"user-invocable"`
}

type SkillInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Source        string `json:"source"`
	Description   string `json:"description"`
	ArgumentHint  string `json:"argument-hint,omitempty"`
	Context       string `json:"context,omitempty"`
	UserInvocable bool   `json:"user-invocable"`
}

func (info SkillInfo) validate() error {
	var errs error
	if info.Name == "" {
		errs = errors.Join(errs, errors.New("name is required"))
	} else {
		if len(info.Name) > MaxNameLength {
			errs = errors.Join(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(info.Name) {
			errs = errors.Join(errs, errors.New("name must be alphanumeric with hyphens"))
		}
	}

	if info.Description == "" {
		errs = errors.Join(errs, errors.New("description is required"))
	} else if len(info.Description) > MaxDescriptionLength {
		errs = errors.Join(errs, fmt.Errorf("description exceeds %d character", MaxDescriptionLength))
	}
	return errs
}

type SkillsLoader struct {
	workspace       string
	workspaceSkills string // workspace skills (project-level)
	globalSkills    string // global skills (~/.spawnbot/skills)
	builtinSkills   string // builtin skills
}

// SkillRoots returns all unique skill root directories used by this loader.
// The order follows resolution priority: workspace > global > builtin.
func (sl *SkillsLoader) SkillRoots() []string {
	roots := []string{sl.workspaceSkills, sl.globalSkills, sl.builtinSkills}
	seen := make(map[string]struct{}, len(roots))
	out := make([]string, 0, len(roots))

	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		clean := filepath.Clean(trimmed)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}

	return out
}

func NewSkillsLoader(workspace string, globalSkills string, builtinSkills string) *SkillsLoader {
	return &SkillsLoader{
		workspace:       workspace,
		workspaceSkills: filepath.Join(workspace, "skills"),
		globalSkills:    globalSkills, // ~/.spawnbot/skills
		builtinSkills:   builtinSkills,
	}
}

func (sl *SkillsLoader) ListSkills() []SkillInfo {
	skills := make([]SkillInfo, 0)
	seen := make(map[string]bool)

	addSkills := func(dir, source string) {
		if dir == "" {
			return
		}
		dirs, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			skillFile := filepath.Join(dir, d.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			info := SkillInfo{
				Name:   d.Name(),
				Path:   skillFile,
				Source: source,
			}
			metadata := sl.getSkillMetadata(skillFile)
			if metadata != nil {
				info.Description = metadata.Description
				info.Name = metadata.Name
				info.ArgumentHint = metadata.ArgumentHint
				info.Context = metadata.Context
				info.UserInvocable = metadata.UserInvocable
			}
			if err := info.validate(); err != nil {
				slog.Warn("invalid skill from "+source, "name", info.Name, "error", err)
				continue
			}
			if seen[info.Name] {
				continue
			}
			seen[info.Name] = true
			skills = append(skills, info)
		}
	}

	// Priority: workspace > global > builtin
	addSkills(sl.workspaceSkills, "workspace")
	addSkills(sl.globalSkills, "global")
	addSkills(sl.builtinSkills, "builtin")

	return skills
}

func (sl *SkillsLoader) LoadSkill(name string) (string, bool) {
	// 1. load from workspace skills first (project-level)
	if sl.workspaceSkills != "" {
		skillFile := filepath.Join(sl.workspaceSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 2. then load from global skills (~/.spawnbot/skills)
	if sl.globalSkills != "" {
		skillFile := filepath.Join(sl.globalSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	// 3. finally load from builtin skills
	if sl.builtinSkills != "" {
		skillFile := filepath.Join(sl.builtinSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	return "", false
}

func (sl *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range skillNames {
		content, ok := sl.LoadSkill(name)
		if ok {
			parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

func (sl *SkillsLoader) BuildSkillsSummary() string {
	allSkills := sl.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "<skills>")
	for _, s := range allSkills {
		escapedName := escapeXML(s.Name)
		escapedDesc := escapeXML(s.Description)

		// Use workspace-relative paths so the LLM passes them
		// directly to read_file (which resolves relative to workspace).
		displayPath := s.Path
		if sl.workspace != "" {
			if rel, err := filepath.Rel(sl.workspace, s.Path); err == nil {
				displayPath = rel
			}
		}
		escapedPath := escapeXML(displayPath)

		lines = append(lines, "  <skill>")
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapedName))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapedDesc))
		if s.ArgumentHint != "" {
			lines = append(lines, fmt.Sprintf("    <usage>/%s %s</usage>", escapeXML(s.Name), escapeXML(s.ArgumentHint)))
		}
		lines = append(lines, fmt.Sprintf("    <location>%s</location>", escapedPath))
		lines = append(lines, fmt.Sprintf("    <source>%s</source>", s.Source))
		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n")
}

// rawSkillMeta is an intermediate struct used during parsing.
// UserInvocable is a pointer so we can detect when the field is absent (nil means not set).
type rawSkillMeta struct {
	Name          string   `json:"name" yaml:"name"`
	Description   string   `json:"description" yaml:"description"`
	Arguments     []string `json:"arguments,omitempty" yaml:"arguments"`
	ArgumentHint  string   `json:"argument-hint,omitempty" yaml:"argument-hint"`
	Context       string   `json:"context,omitempty" yaml:"context"`
	AgentType     string   `json:"agent_type,omitempty" yaml:"agent_type"`
	AllowedTools  []string `json:"allowed_tools,omitempty" yaml:"allowed_tools"`
	UserInvocable *bool    `json:"user-invocable,omitempty" yaml:"user-invocable"`
}

func applyDefaults(meta *SkillMetadata, raw *rawSkillMeta) {
	if raw.Name != "" {
		meta.Name = raw.Name
	}
	if raw.Description != "" {
		meta.Description = raw.Description
	}
	meta.Arguments = raw.Arguments
	meta.ArgumentHint = raw.ArgumentHint
	meta.AgentType = raw.AgentType
	meta.AllowedTools = raw.AllowedTools

	// Context: default to "inline", validate against allowed values.
	switch raw.Context {
	case "inline", "fork", "spawn":
		meta.Context = raw.Context
	case "":
		meta.Context = "inline"
	default:
		meta.Context = "inline"
	}

	// UserInvocable: default to true when the field is absent from the source.
	if raw.UserInvocable == nil {
		meta.UserInvocable = true
	} else {
		meta.UserInvocable = *raw.UserInvocable
	}
}

func (sl *SkillsLoader) getSkillMetadata(skillPath string) *SkillMetadata {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		logger.WarnCF("skills", "Failed to read skill metadata",
			map[string]any{
				"skill_path": skillPath,
				"error":      err.Error(),
			})
		return nil
	}

	frontmatter, bodyContent := splitFrontmatter(string(content))
	dirName := filepath.Base(filepath.Dir(skillPath))
	title, bodyDescription := extractMarkdownMetadata(bodyContent)

	metadata := &SkillMetadata{
		Name:        dirName,
		Description: bodyDescription,
	}
	if title != "" && namePattern.MatchString(title) && len(title) <= MaxNameLength {
		metadata.Name = title
	}

	if frontmatter == "" {
		// Apply defaults even when there is no frontmatter.
		applyDefaults(metadata, &rawSkillMeta{})
		return metadata
	}

	// Try JSON first (for backward compatibility).
	var jsonRaw rawSkillMeta
	if err := json.Unmarshal([]byte(frontmatter), &jsonRaw); err == nil {
		applyDefaults(metadata, &jsonRaw)
		return metadata
	}

	// Fall back to YAML parsing.
	var yamlRaw rawSkillMeta
	if err := yaml.Unmarshal([]byte(frontmatter), &yamlRaw); err == nil {
		applyDefaults(metadata, &yamlRaw)
	}
	return metadata
}

// GetSkillMetadata returns the full metadata for a skill by name.
func (sl *SkillsLoader) GetSkillMetadata(name string) SkillMetadata {
	for _, root := range sl.SkillRoots() {
		skillDir := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
			m := sl.getSkillMetadata(filepath.Join(skillDir, "SKILL.md"))
			if m != nil {
				return *m
			}
		}
	}
	return SkillMetadata{Context: "inline", UserInvocable: true}
}

// GetSkillDir returns the absolute path to a skill's directory.
func (sl *SkillsLoader) GetSkillDir(name string) string {
	for _, root := range sl.SkillRoots() {
		skillDir := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
			return skillDir
		}
	}
	return ""
}

func extractMarkdownMetadata(content string) (title, description string) {
	p := parser.NewWithExtensions(parser.CommonExtensions)
	doc := markdown.Parse([]byte(content), p)
	if doc == nil {
		return "", ""
	}

	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext
		}

		switch n := node.(type) {
		case *ast.Heading:
			if title == "" && n.Level == 1 {
				title = nodeText(n)
				if title != "" && description != "" {
					return ast.Terminate
				}
			}
		case *ast.Paragraph:
			if description == "" {
				description = nodeText(n)
				if title != "" && description != "" {
					return ast.Terminate
				}
			}
		}
		return ast.GoToNext
	})

	return title, description
}

func nodeText(n ast.Node) string {
	var b strings.Builder
	ast.WalkFunc(n, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext
		}

		switch t := node.(type) {
		case *ast.Text:
			b.Write(t.Literal)
		case *ast.Code:
			b.Write(t.Literal)
		case *ast.Softbreak, *ast.Hardbreak, *ast.NonBlockingSpace:
			b.WriteByte(' ')
		}
		return ast.GoToNext
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// parseSimpleYAML parses YAML frontmatter and extracts known metadata fields.
func (sl *SkillsLoader) parseSimpleYAML(content string) map[string]string {
	result := make(map[string]string)

	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(content), &meta); err != nil {
		return result
	}
	if meta.Name != "" {
		result["name"] = meta.Name
	}
	if meta.Description != "" {
		result["description"] = meta.Description
	}

	return result
}

func (sl *SkillsLoader) extractFrontmatter(content string) string {
	frontmatter, _ := splitFrontmatter(content)
	return frontmatter
}

func (sl *SkillsLoader) stripFrontmatter(content string) string {
	_, body := splitFrontmatter(content)
	return body
}

func splitFrontmatter(content string) (frontmatter, body string) {
	normalized := string(parser.NormalizeNewlines([]byte(content)))
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return "", content
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", content
	}

	frontmatter = strings.Join(lines[1:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	body = strings.TrimLeft(body, "\n")
	return frontmatter, body
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
