package android

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Artifact is an installable APK found under a project directory.
type Artifact struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// skipArtifactDirs are directories that never contain a project's build output.
var skipArtifactDirs = map[string]bool{
	".git": true, ".gradle": true, ".idea": true, ".kotlin": true,
	"node_modules": true, ".cxx": true, "__pycache__": true,
}

// maxArtifactDepth bounds the walk relative to the starting directory, so a
// deep checkout cannot stall the UI.
const maxArtifactDepth = 12

// FindAPKs walks dir for *.apk files, newest first, so the UI can offer them to
// `android install` and `android run`. Gradle's build/outputs/apk tree sorts
// ahead of stray APKs elsewhere, dependency and VCS caches are skipped, and the
// result is capped at limit. Paths come back exactly as walked, so a relative
// dir yields relative paths the CLI resolves against the same working directory.
func FindAPKs(dir string, limit int) []Artifact {
	if limit <= 0 {
		return nil
	}
	var out []Artifact
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path == dir {
				return nil
			}
			name := d.Name()
			if skipArtifactDirs[name] || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if rel, err := filepath.Rel(dir, path); err != nil ||
				strings.Count(rel, string(filepath.Separator)) >= maxArtifactDepth {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".apk") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, Artifact{Path: path, Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})

	sort.SliceStable(out, func(i, j int) bool {
		bi, bj := isBuildOutput(out[i].Path), isBuildOutput(out[j].Path)
		if bi != bj {
			return bi
		}
		if !out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ModTime.After(out[j].ModTime)
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func isBuildOutput(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/build/outputs/apk/")
}
