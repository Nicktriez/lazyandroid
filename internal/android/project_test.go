package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeAPK(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFindAPKs(t *testing.T) {
	dir := t.TempDir()
	writeAPK(t, filepath.Join(dir, "app", "build", "outputs", "apk", "debug", "app-debug.apk"), 10)
	writeAPK(t, filepath.Join(dir, "app", "build", "outputs", "apk", "release", "app-release.apk"), 20)
	writeAPK(t, filepath.Join(dir, "scratch", "copy.apk"), 30)
	writeAPK(t, filepath.Join(dir, "node_modules", "dep", "junk.apk"), 40)
	writeAPK(t, filepath.Join(dir, ".git", "objects", "blob.apk"), 50)

	got := FindAPKs(dir, 40)
	if len(got) != 3 {
		t.Fatalf("found %d APKs, want 3 (caches skipped): %+v", len(got), got)
	}
	for _, a := range got {
		if strings.Contains(a.Path, "node_modules") || strings.Contains(a.Path, ".git") {
			t.Errorf("walk descended into a skipped directory: %s", a.Path)
		}
	}
	// Gradle's build outputs sort ahead of stray APKs.
	if !isBuildOutput(got[0].Path) || !isBuildOutput(got[1].Path) {
		t.Errorf("build outputs must sort first, got %+v", got)
	}
	if got[0].Size == 0 {
		t.Errorf("size not recorded: %+v", got[0])
	}
}

func TestFindAPKsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "app", "build", "outputs", "apk", "debug", "old.apk")
	newer := filepath.Join(dir, "app", "build", "outputs", "apk", "debug", "new.apk")
	writeAPK(t, old, 1)
	writeAPK(t, newer, 1)
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	got := FindAPKs(dir, 40)
	if len(got) != 2 || got[0].Path != newer {
		t.Errorf("want newest first, got %+v", got)
	}
}

func TestFindAPKsLimitAndEmpty(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		writeAPK(t, filepath.Join(dir, "build", "outputs", "apk", string(rune('a'+i)), "x.apk"), 1)
	}
	if got := FindAPKs(dir, 2); len(got) != 2 {
		t.Errorf("limit ignored: got %d, want 2", len(got))
	}
	if got := FindAPKs(t.TempDir(), 10); len(got) != 0 {
		t.Errorf("empty dir returned %d APKs", len(got))
	}
	if got := FindAPKs(dir, 0); got != nil {
		t.Errorf("limit 0 returned %v, want nil", got)
	}
}

func TestFindAPKsSkipsDeepTrees(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "app", "build", "outputs", "apk", "debug")
	writeAPK(t, filepath.Join(deep, "ok.apk"), 1)
	// One directory per level, well past maxArtifactDepth.
	nested := dir
	for range maxArtifactDepth + 4 {
		nested = filepath.Join(nested, "d")
	}
	writeAPK(t, filepath.Join(nested, "toodeep.apk"), 1)

	got := FindAPKs(dir, 40)
	if len(got) != 1 || got[0].Path != filepath.Join(deep, "ok.apk") {
		t.Errorf("depth limit not applied, got %+v", got)
	}
}
