package embedded

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/jeanhaley32/claude-capsule/internal/constants"
)

//go:embed Dockerfile
var Dockerfile []byte

// BuildImage builds the Docker image from the embedded Dockerfile.
// Returns nil if successful, error otherwise.
func BuildImage(imageName string) error {
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

	// Build the image, passing host UID/GID so the in-container claude user
	// matches the host user. This ensures bind-mounted directories (/workspace,
	// /claude-env) are writable without UID remapping on Linux.
	//
	// On macOS, Docker Desktop's VirtioFS transparently maps the host user to
	// UID/GID 1000 inside the container regardless of the actual macOS UID
	// (typically 501). Baking the real macOS UID into the image would cause a
	// mismatch and break bind-mount writes. On Linux, raw kernel bind mounts
	// use real UIDs, so HOST_UID/GID must match the actual host user.
	hostUID := "1000"
	hostGID := "1000"
	if runtime.GOOS == "linux" {
		hostUID = strconv.Itoa(os.Getuid())
		hostGID = strconv.Itoa(os.Getgid())
	}
	cmd := exec.Command("docker", "build",
		"-t", imageName,
		"--build-arg", "HOST_UID="+hostUID,
		"--build-arg", "HOST_GID="+hostGID,
		tempDir,
	)
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
