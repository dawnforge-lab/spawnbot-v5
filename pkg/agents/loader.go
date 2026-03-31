package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type agentFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Model         string   `yaml:"model"`
	ToolsAllow    []string `yaml:"tools_allow"`
	ToolsDeny     []string `yaml:"tools_deny"`
	MaxIterations int      `yaml:"max_iterations"`
	Timeout       string   `yaml:"timeout"`
}

func ParseAgentMD(content, source, baseDir string) (*AgentDefinition, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var meta agentFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	var timeout time.Duration
	if meta.Timeout != "" {
		timeout, err = time.ParseDuration(meta.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", meta.Timeout, err)
		}
	}

	def := &AgentDefinition{
		Name:          meta.Name,
		Description:   meta.Description,
		SystemPrompt:  strings.TrimSpace(body),
		Model:         meta.Model,
		ToolsAllow:    meta.ToolsAllow,
		ToolsDeny:     meta.ToolsDeny,
		MaxIterations: meta.MaxIterations,
		Timeout:       timeout,
		Source:        source,
		BaseDir:       baseDir,
	}

	def.ApplyDefaults()

	if err := def.Validate(); err != nil {
		return nil, err
	}

	return def, nil
}

func LoadFromDir(dir string) ([]*AgentDefinition, []string) {
	var agents []*AgentDefinition
	var warnings []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentFile := filepath.Join(dir, entry.Name(), "AGENT.md")
		content, err := os.ReadFile(agentFile)
		if err != nil {
			continue
		}

		baseDir := filepath.Join(dir, entry.Name())
		def, err := ParseAgentMD(string(content), "workspace", baseDir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("agents/%s/AGENT.md failed to load: %s", entry.Name(), err))
			continue
		}

		agents = append(agents, def)
	}

	return agents, warnings
}

func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("missing YAML frontmatter (file must start with ---)")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	fm := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimLeft(body, "\n")

	return fm, body, nil
}
