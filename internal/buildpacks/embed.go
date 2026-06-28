// Package buildpacks embeds the per-language Dockerfile templates so the
// compiled ForgeOS binary is self-contained (no external template files).
package buildpacks

import "embed"

// Templates holds all Dockerfile templates under this directory tree.
//
//go:embed */Dockerfile.tmpl */*.dockerfile
var Templates embed.FS
