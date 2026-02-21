package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanhaley32/claude-capsule/internal/volume"
)

func TestCheckVolumeMounted_UsesCorrectPrefix(t *testing.T) {
	// Verify the detector scans the correct directory (derived from volume.MountPointPrefix)
	mountDir := filepath.Dir(volume.MountPointPrefix)
	prefix := filepath.Base(volume.MountPointPrefix)

	if mountDir != "/Volumes" {
		t.Errorf("mount directory = %q, want /Volumes", mountDir)
	}
	if prefix != "Capsule-" {
		t.Errorf("mount prefix = %q, want Capsule-", prefix)
	}
}

func TestCheckVolumeMounted_NoMountPoints(t *testing.T) {
	// Create a detector with arbitrary values
	d := NewDetector("/tmp/test.sparseimage", "test-container", "/tmp/workspace")

	// On a test system without capsule volumes, should return empty
	mountPoint, mounted := d.checkVolumeMounted()

	// We can't guarantee /Volumes exists on all test systems (Linux CI),
	// so we just verify it doesn't panic and returns consistent results
	if mounted && mountPoint == "" {
		t.Error("checkVolumeMounted() returned mounted=true but empty mountPoint")
	}
	if !mounted && mountPoint != "" {
		t.Error("checkVolumeMounted() returned mounted=false but non-empty mountPoint")
	}
}

func TestCheckSymlink_NotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDetector("", "", tmpDir)
	exists, broken := d.checkSymlink()

	if exists {
		t.Error("checkSymlink() exists = true for non-existent symlink")
	}
	if broken {
		t.Error("checkSymlink() broken = true for non-existent symlink")
	}
}

func TestCheckSymlink_ValidSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a target directory and symlink
	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmpDir, "_docs")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	d := NewDetector("", "", tmpDir)
	exists, broken := d.checkSymlink()

	if !exists {
		t.Error("checkSymlink() exists = false for valid symlink")
	}
	if broken {
		t.Error("checkSymlink() broken = true for valid symlink")
	}
}

func TestCheckSymlink_BrokenSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a symlink pointing to a non-existent target
	link := filepath.Join(tmpDir, "_docs")
	if err := os.Symlink("/nonexistent/target", link); err != nil {
		t.Fatal(err)
	}

	d := NewDetector("", "", tmpDir)
	exists, broken := d.checkSymlink()

	if !exists {
		t.Error("checkSymlink() exists = false for broken symlink")
	}
	if !broken {
		t.Error("checkSymlink() broken = false for broken symlink")
	}
}

func TestCheckSymlink_RegularFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular file named _docs (not a symlink)
	docsPath := filepath.Join(tmpDir, "_docs")
	if err := os.WriteFile(docsPath, []byte("not a symlink"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector("", "", tmpDir)
	exists, broken := d.checkSymlink()

	if exists {
		t.Error("checkSymlink() exists = true for regular file")
	}
	if broken {
		t.Error("checkSymlink() broken = true for regular file")
	}
}

func TestDetect_Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	volumePath := filepath.Join(tmpDir, "nonexistent.sparseimage")
	d := NewDetector(
		volumePath,
		"nonexistent-container-xyz987",
		tmpDir,
	)
	state := d.Detect()

	if state.VolumeExists {
		t.Error("Detect() VolumeExists = true for nonexistent volume")
	}
	// Note: VolumeMounted may be true if the developer has a real capsule
	// volume mounted — the detector scans /Volumes/Capsule-* globally.
	// We only assert the fields we fully control.
	if state.ContainerExists {
		t.Error("Detect() ContainerExists = true for nonexistent container")
	}
	if state.SymlinkExists {
		t.Error("Detect() SymlinkExists = true for nonexistent symlink")
	}
	if state.VolumePath != volumePath {
		t.Errorf("Detect() VolumePath = %q, want %q", state.VolumePath, volumePath)
	}
}
