// Package workspace embeds the default workspace files and provides a Deploy
// function that copies them to a target directory with template rendering.
// Both the CLI onboard command and the web backend use this package to create
// identical workspace layouts.
package workspace

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:generate cp -r ../../workspace files
//go:embed files
var embeddedFiles embed.FS

// TemplateData is passed to workspace templates for rendering.
type TemplateData struct {
	UserName string
}

// Deploy copies the embedded workspace files to targetDir, rendering Go
// templates in .md files that contain {{ syntax. Existing files are
// overwritten — this is intentional so that onboard always produces a
// consistent workspace.
func Deploy(targetDir string, data TemplateData) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	return fs.WalkDir(embeddedFiles, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}

		relPath, err := filepath.Rel("files", path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}

		targetPath := filepath.Join(targetDir, relPath)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(targetPath), err)
		}

		// Render Go templates in markdown files
		if strings.HasSuffix(path, ".md") && bytes.Contains(content, []byte("{{")) {
			rendered, tErr := renderTemplate(string(content), data)
			if tErr != nil {
				return fmt.Errorf("render template %s: %w", path, tErr)
			}
			content = []byte(rendered)
		}

		return os.WriteFile(targetPath, content, 0o644)
	})
}

func renderTemplate(text string, data TemplateData) (string, error) {
	tmpl, err := template.New("workspace").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
