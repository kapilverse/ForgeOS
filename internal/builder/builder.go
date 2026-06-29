package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
)

type Builder struct {
	docker *client.Client
}

func New(docker *client.Client) *Builder {
	return &Builder{
		docker: docker,
	}
}

type BuildRequest struct {
	AppSlug  string
	Version  int
	RepoURL  string
	Branch   string
	Port     int
	LogSink  LogSink
}

func (b *Builder) Build(ctx context.Context, req BuildRequest) (string, error) {
	// 1. Create a temp directory
	tempDir, err := os.MkdirTemp("", "forgeos-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if req.LogSink != nil {
		req.LogSink(fmt.Sprintf("==> Cloning %s (branch: %s)...\n", req.RepoURL, req.Branch))
	}

	// 2. Clone the repo
	if err := Clone(ctx, req.RepoURL, req.Branch, tempDir); err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}

	if req.LogSink != nil {
		req.LogSink("==> Detecting project type...\n")
	}

	// 3. Detect language
	det, err := Detect(tempDir)
	if err != nil {
		return "", err
	}

	if req.LogSink != nil {
		req.LogSink(fmt.Sprintf("==> Detected language: %s\n", det.Language))
	}

	// 4. Generate Dockerfile (if not LangDocker)
	if det.Language != LangDocker {
		port := det.Port
		if port == 0 {
			port = req.Port
		}
		dfContent, err := Generate(det, tempDir, port)
		if err != nil {
			return "", fmt.Errorf("generate Dockerfile: %w", err)
		}
		
		if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dfContent), 0644); err != nil {
			return "", fmt.Errorf("write Dockerfile: %w", err)
		}
		if req.LogSink != nil {
			req.LogSink("==> Generated Dockerfile automatically\n")
		}
	} else {
		if req.LogSink != nil {
			req.LogSink("==> Using repository's existing Dockerfile\n")
		}
	}

	// 5. Build image
	imageTag := fmt.Sprintf("forgeos.local/%s:v%d", req.AppSlug, req.Version)
	if req.LogSink != nil {
		req.LogSink(fmt.Sprintf("==> Building Docker image %s...\n", imageTag))
	}

	if err := DockerBuild(ctx, b.docker, tempDir, imageTag, req.LogSink); err != nil {
		return "", err
	}

	if req.LogSink != nil {
		req.LogSink("==> Build successful!\n")
	}

	return imageTag, nil
}
