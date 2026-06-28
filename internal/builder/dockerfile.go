package builder

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"forgeos/internal/buildpacks"
)

// osStat is a tiny alias so fileExists reads cleanly.
var osStat = os.Stat

// Generate produces a Dockerfile string for the detected project. For
// LangDocker (repo already has a Dockerfile) it returns "" — the caller uses
// the repo's Dockerfile as-is. The port is the fallback listen port (the app's
// configured port); the template sets it as ENV PORT.
func Generate(det *Detection, dir string, port int) (string, error) {
	if det.Language == LangDocker {
		return "", nil // repo's own Dockerfile is used as-is
	}

	data := map[string]any{
		"Port":  port,
		"Entry": det.PythonEntry,
	}

	switch det.Language {
	case LangNode:
		data["IsTS"] = det.NodeIsTS
		data["StartCmd"] = nodeStartCmd(det)
		data["HasPackageLock"] = fileExists(dir, "package-lock.json")
		data["UseYarn"] = fileExists(dir, "yarn.lock")
	case LangPython:
		data["HasRequirements"] = fileExists(dir, "requirements.txt")
		data["HasPyproject"] = fileExists(dir, "pyproject.toml")
	case LangGo:
		data["BuildTarget"] = "." // build the module at repo root
	}

	name := templateName(det.Language)
	tmplBytes, err := buildpacksFS(name)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", name, err)
	}

	t, err := template.New(name).Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return strings.TrimSpace(out.String()) + "\n", nil
}

// templateName maps a language to its embedded template path.
func templateName(lang Language) string {
	switch lang {
	case LangNode:
		return "nodejs/Dockerfile.tmpl"
	case LangPython:
		return "python/Dockerfile.tmpl"
	case LangGo:
		return "go/Dockerfile.tmpl"
	case LangStatic:
		return "static/Dockerfile.tmpl"
	default:
		return "static/Dockerfile.tmpl"
	}
}

// buildpacksFS reads a template from the embedded FS. The //go:embed directive
// lives in the buildpacks package, so paths within the FS are relative to that
// directory (e.g. "nodejs/Dockerfile.tmpl").
func buildpacksFS(name string) ([]byte, error) {
	return fs.ReadFile(buildpacks.Templates, name)
}

// nodeStartCmd formats the start command as a JSON-array CMD for the template.
// If detection found a custom start script it is split on spaces; otherwise we
// fall back to "npm start".
func nodeStartCmd(det *Detection) string {
	cmd := det.NodeStartScript
	if cmd == "" {
		cmd = "npm start"
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		parts = []string{"npm", "start"}
	}
	// Emit as JSON-array shell fragment, e.g. ["npm","start"]
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = `"` + p + `"`
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// fileExists reports whether name exists in dir.
func fileExists(dir, name string) bool {
	_, err := osStat(filepath.Join(dir, name))
	return err == nil
}
