package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/client"
)

// LogSink receives a single line of build output. Used by the builder to
// stream Docker build log lines into the database.
type LogSink func(line string)

// DockerBuild invokes the Docker daemon's ImageBuild API on a directory context.
// imageTag is the tag to apply (e.g. "forgeos/my-app:v3").
// sink receives each build-log line as it streams in.
//
// The build context is a tar archive piped via the Context field, but Docker
// also supports a local directory path via RemoteContext — we use the context-dir
// form by setting RemoteContext to the absolute path of the build directory.
// This is simpler and avoids having to tar the dir ourselves.
func DockerBuild(ctx context.Context, cli *client.Client, contextDir, imageTag string, sink LogSink) error {
	opts := build.ImageBuildOptions{
		Tags:          []string{imageTag},
		Remove:        true,
		ForceRemove:   true,
		RemoteContext: contextDir,
	}

	resp, err := cli.ImageBuild(ctx, nil, opts)
	if err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	defer resp.Body.Close()

	// The build stream is a JSON-lines stream; each line is either a build step
	// progress entry or an error. Decode each line and emit a human-readable
	// line to the sink.
	dec := json.NewDecoder(resp.Body)
	for {
		var entry struct {
			Stream string `json:"stream"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := dec.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read build stream: %w", err)
		}

		if entry.Error.Message != "" {
			return fmt.Errorf("docker build error: %s", strings.TrimRight(entry.Error.Message, "\n"))
		}

		line := strings.TrimRight(entry.Stream, "\n")
		if line != "" && sink != nil {
			sink(line + "\n")
		}
	}
	return nil
}
