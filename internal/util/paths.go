// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"os"
	"path/filepath"

	"v.io/x/devtools/internal/tool"
)

const (
	rootEnv          = "V23_ROOT"
	metadataDirName  = ".v23"
	metadataFileName = "metadata.v2"
)

// AliasesFilePath returns the path to the aliases file.
func AliasesFilePath(ctx *tool.Context) (string, error) {
	dataDir, err := DataDirPath(ctx, tool.Name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "aliases.v1.xml"), nil
}

// OncallRotationPath returns the path to the oncall rotation file.
func OncallRotationPath(ctx *tool.Context) (string, error) {
	dataDir, err := DataDirPath(ctx, tool.Name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "oncall.v1.xml"), nil
}

// ConfigFilePath returns the path to the tools configuration file.
func ConfigFilePath(ctx *tool.Context) (string, error) {
	dataDir, err := DataDirPath(ctx, tool.Name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "config.v1.xml"), nil
}

// DataDirPath returns the path to the data directory of the given tool.
func DataDirPath(ctx *tool.Context, toolName string) (string, error) {
	projects, tools, err := readManifest(ctx, false)
	if err != nil {
		return "", err
	}
	if toolName == "" {
		// If the tool name is not set, use "v23" as the default. As a
		// consequence, any manifest is assumed to specify a "v23" tool.
		toolName = "v23"
	}
	tool, ok := tools[toolName]
	if !ok {
		return "", fmt.Errorf("tool %q not found in the manifest", tool.Name)
	}
	projectName := tool.Project
	project, ok := projects[projectName]
	if !ok {
		return "", fmt.Errorf("project %q not found in the manifest", projectName)
	}
	return filepath.Join(project.Path, tool.Data), nil
}

// LocalManifestFile returns the path to the local manifest.
func LocalManifestFile() (string, error) {
	root, err := V23Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".local_manifest"), nil
}

// LocalSnapshotDir returns the path to the local snapshot directory.
func LocalSnapshotDir() (string, error) {
	root, err := V23Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".snapshot"), nil
}

// ManifestDir returns the path to the manifest directory.
func ManifestDir() (string, error) {
	root, err := V23Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".manifest", "v2"), nil
}

// ManifestFile returns the path to the manifest file with the given
// relative path.
func ManifestFile(name string) (string, error) {
	dir, err := ManifestDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// RemoteSnapshotDir returns the path to the remote snapshot directory.
func RemoteSnapshotDir() (string, error) {
	manifestDir, err := ManifestDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(manifestDir, "snapshot"), nil
}

// ResolveManifestPath resolves the given manifest name to an absolute
// path in the local filesystem.
func ResolveManifestPath(name string) (string, error) {
	if name != "" {
		if filepath.IsAbs(name) {
			return name, nil
		}
		return ManifestFile(name)
	}
	path, err := LocalManifestFile()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ResolveManifestPath("default")
		}
		return "", fmt.Errorf("Stat(%v) failed: %v", path, err)
	}
	return path, nil
}

// VanadiumGitRepoHost returns the URL that hosts Vanadium git
// repositories.
func VanadiumGitRepoHost() string {
	return "https://vanadium.googlesource.com/"
}

// V23Root returns the root of the Vanadium universe.
func V23Root() (string, error) {
	root := os.Getenv(rootEnv)
	if root == "" {
		return "", fmt.Errorf("%v is not set", rootEnv)
	}
	result, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("EvalSymlinks(%v) failed: %v", root, err)
	}
	return result, nil
}