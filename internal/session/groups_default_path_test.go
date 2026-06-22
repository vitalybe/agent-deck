package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHasUsableDefaultPath covers the git-free existence check used on the hot
// path of opening the Quick Session dialog. It must mirror DefaultPathForGroup's
// notion of "is there a usable folder?" (explicit default first, then the most
// recent session path) without shelling out to git.
func TestHasUsableDefaultPath(t *testing.T) {
	tmp := t.TempDir()

	tree := NewGroupTree(nil)
	tree.Groups["g"] = &Group{Name: "g", Path: "g"}

	if tree.HasUsableDefaultPath("missing-group") {
		t.Error("a group that does not exist must report no usable default path")
	}
	if tree.HasUsableDefaultPath("g") {
		t.Error("a group with no default path and no sessions must report false")
	}

	// Explicit default path pointing at an existing directory.
	tree.Groups["g"].DefaultPath = tmp
	if !tree.HasUsableDefaultPath("g") {
		t.Error("an existing default directory must report true")
	}

	// Explicit default path that does not exist.
	tree.Groups["g"].DefaultPath = filepath.Join(tmp, "does-not-exist")
	if tree.HasUsableDefaultPath("g") {
		t.Error("a non-existent default path must report false")
	}

	// Explicit default path that is a file, not a directory.
	file := filepath.Join(tmp, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree.Groups["g"].DefaultPath = file
	if tree.HasUsableDefaultPath("g") {
		t.Error("a default path that is a file must report false")
	}

	// No explicit default: fall back to the most recent session's path.
	tree.Groups["g"].DefaultPath = ""
	tree.Groups["g"].Sessions = []*Instance{{
		ProjectPath:    tmp,
		LastAccessedAt: time.Now(),
	}}
	if !tree.HasUsableDefaultPath("g") {
		t.Error("a recent session in an existing directory must report true")
	}
}
