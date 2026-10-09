package embedded

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jeanhaley32/claude-capsule/internal/constants"
)

//go:embed Dockerfile
var Dockerfile []byte

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

	// Build the image
	cmd := exec.Command("docker", "build", "--target", target, "-t", imageName, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	return nil
}

// ImageExists checks if a Docker image exists locally.
func ImageExists(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}
