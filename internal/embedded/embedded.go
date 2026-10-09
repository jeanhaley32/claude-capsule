package embedded

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jeanhaley32/claude-capsule/internal/constants"
)

//go:embed Dockerfile
var Dockerfile []byte

// rodinFiles are copied into the build context next to the Dockerfile so the
// rodin stage can COPY them.
//
//go:embed rodin
var rodinFiles embed.FS

// BuildImage builds the named stage of the embedded Dockerfile and tags it imageName.
func BuildImage(imageName, target string) error {
	// Create temp directory for build context
	tempDir, err := os.MkdirTemp("", "capsule-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write Dockerfile to temp directory
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, Dockerfile, constants.FilePermissions); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	if err := writeBuildContext(tempDir); err != nil {
		return err
	}

	// Build the image
	cmd := exec.Command("docker", "build", "--target", target, "-t", imageName, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	return nil
}

func writeBuildContext(dir string) error {
	return fs.WalkDir(rodinFiles, "rodin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, path)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := rodinFiles.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, constants.FilePermissions)
	})
}

// ImageExists checks if a Docker image exists locally.
func ImageExists(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}
