// Package builder implements the ForgeOS build pipeline: clone a git repo,
// detect the language/framework, generate a Dockerfile, and build the image
// with the Docker daemon. The detector is a pure function over a directory so
// it is unit-testable without Docker.
package builder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Language is the detected runtime family for a repo.
type Language string

const (
	LangUnknown Language = "unknown"
	LangDocker  Language = "docker" // repo ships its own Dockerfile
	LangNode    Language = "node"
	LangPython  Language = "python"
	LangGo      Language = "go"
	LangStatic  Language = "static" // plain static site (index.html, no JS)
)

// Detection is the result of inspecting a repo root.
type Detection struct {
	Language Language

	// Node: detected start command ("npm start" / "yarn start") and whether
	// the project is a TypeScript build (tsconfig.json present).
	NodeStartScript string
	NodeIsTS        bool

	// Python: detected entry module for gunicorn, e.g. "main:app" or "app:app".
	PythonEntry string

	// Go: detected binary name (defaults to repo dir name).
	GoBinary string

	// Port is the best-guess listen port from the source (Dockerfile EXPOSE,
	// etc.). 0 means "unknown" → caller falls back to the app's configured port.
	Port int
}

// ErrDetectionFailed is returned when no supported language is detected.
var ErrDetectionFailed = errors.New("could not detect project language; add a Dockerfile")

// Detect inspects a repo root and reports the language + any framework hints.
// Detection precedence (highest first):
//  1. Dockerfile present  → docker (use as-is)
//  2. package.json        → node
//  3. requirements.txt / pyproject.toml → python
//  4. go.mod              → go
//  5. index.html (no package.json) → static
func Detect(dir string) (*Detection, error) {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	readFile := func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, name))
	}

	// 1. Custom Dockerfile wins; we just run it as-is.
	if exists("Dockerfile") {
		return &Detection{Language: LangDocker, Port: parseDockerfileExpose(dir)}, nil
	}

	// 2. Node.js
	if exists("package.json") {
		d := &Detection{Language: LangNode, NodeIsTS: exists("tsconfig.json")}
		if start, err := detectNodeStart(readFile); err == nil {
			d.NodeStartScript = start
		}
		return d, nil
	}

	// 3. Python
	if exists("requirements.txt") || exists("pyproject.toml") {
		d := &Detection{Language: LangPython}
		if entry, err := detectPythonEntry(dir, exists); err == nil && entry != "" {
			d.PythonEntry = entry
		}
		if d.PythonEntry == "" {
			d.PythonEntry = "main:app" // conventional default
		}
		return d, nil
	}

	// 4. Go
	if exists("go.mod") {
		d := &Detection{Language: LangGo}
		d.GoBinary = filepath.Base(filepath.Clean(dir))
		if d.GoBinary == "" || d.GoBinary == "." || d.GoBinary == "/" {
			d.GoBinary = "app"
		}
		return d, nil
	}

	// 5. Static site (index.html with no build tooling)
	if exists("index.html") {
		return &Detection{Language: LangStatic}, nil
	}

	return nil, fmt.Errorf("%w: inspected %s", ErrDetectionFailed, dir)
}

// detectNodeStart parses package.json for a "start" script, returning the
// command to run (e.g. "node server.js"). Falls back to "npm start".
func detectNodeStart(readFile func(string) ([]byte, error)) (string, error) {
	data, err := readFile("package.json")
	if err != nil {
		return "", err
	}
	scripts := extractJSONObjectField(string(data), "scripts")
	if start := extractJSONObjectField(scripts, "start"); start != "" {
		return start, nil
	}
	return "npm start", nil
}

// detectPythonEntry looks for common WSGI/ASGI entry points (app.py, main.py,
// wsgi.py) and returns "module:app" if found.
func detectPythonEntry(dir string, exists func(string) bool) (string, error) {
	// Prefer main.py then app.py then wsgi.py.
	for _, cand := range []struct{ file, module string }{
		{"main.py", "main"},
		{"app.py", "app"},
		{"wsgi.py", "wsgi"},
	} {
		if exists(cand.file) {
			return cand.module + ":app", nil
		}
	}
	return "", nil
}

// parseDockerfileExpose scans a Dockerfile for an EXPOSE directive so the
// caller can record the right port. Returns 0 if none found.
func parseDockerfileExpose(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return 0
	}
	tokens := tokenizeLines(string(data))
	for i, t := range tokens {
		if equalFold(t, "EXPOSE") && i+1 < len(tokens) {
			if port := atoiSafe(tokens[i+1]); port > 0 {
				return port
			}
		}
	}
	return 0
}
