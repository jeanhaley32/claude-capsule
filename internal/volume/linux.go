package volume

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeanhaley32/claude-capsule/internal/constants"
	"github.com/jeanhaley32/claude-capsule/internal/terminal"
)

// LinuxMountPointPrefix is the prefix for mount points under /tmp.
// Exported so the state detector can use it instead of hardcoding paths.
const LinuxMountPointPrefix = "/tmp/capsule-"

// linuxVolumeOperationTimeout is the timeout for LUKS operations (cryptsetup can
// take a few seconds on format, especially on slower disks).
const linuxVolumeOperationTimeout = 5 * time.Minute

// LinuxVolumeManager implements VolumeManager using LUKS/cryptsetup for Linux.
type LinuxVolumeManager struct{}

// NewLinuxVolumeManager creates a new Linux volume manager.
func NewLinuxVolumeManager() *LinuxVolumeManager {
	return &LinuxVolumeManager{}
}

func (m *LinuxVolumeManager) Bootstrap(cfg BootstrapConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid bootstrap config: %w", err)
	}

	volumePath := cfg.VolumePath

	if m.Exists(volumePath) {
		return fmt.Errorf("volume already exists at %s", volumePath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(volumePath)
	if err := os.MkdirAll(parentDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// 1. Create sparse file using truncate (no root required)
	sizeStr := fmt.Sprintf("%dG", cfg.SizeGB)
	if err := runCmd(linuxVolumeOperationTimeout, nil, "truncate", "-s", sizeStr, volumePath); err != nil {
		return fmt.Errorf("failed to create volume file: %w", err)
	}

	// 2. Format as LUKS2 (reads passphrase from stdin)
	if err := runCmd(linuxVolumeOperationTimeout, cfg.Password.Reader(),
		"sudo", "cryptsetup", "luksFormat",
		"--batch-mode", "--type", "luks2",
		"--key-file", "-",
		volumePath,
	); err != nil {
		os.Remove(volumePath)
		return fmt.Errorf("failed to format LUKS volume: %w", err)
	}

	// 3. Open the LUKS device
	mapperName := m.generateMapperName(volumePath)
	if err := runCmd(linuxVolumeOperationTimeout, cfg.Password.Reader(),
		"sudo", "cryptsetup", "luksOpen",
		"--key-file", "-",
		volumePath, mapperName,
	); err != nil {
		os.Remove(volumePath)
		return fmt.Errorf("failed to open LUKS volume: %w", err)
	}

	mapperDev := "/dev/mapper/" + mapperName

	// 4. Format the device as ext4
	if err := runCmd(linuxVolumeOperationTimeout, nil, "sudo", "mkfs.ext4", "-q", mapperDev); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(volumePath)
		return fmt.Errorf("failed to format ext4 filesystem: %w", err)
	}

	// 5. Create and mount the mount point
	mountPoint := m.generateMountPoint(volumePath)
	if err := os.MkdirAll(mountPoint, constants.DirPermissions); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(volumePath)
		return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
	}

	if err := runCmd(linuxVolumeOperationTimeout, nil, "sudo", "mount", mapperDev, mountPoint); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(volumePath)
		os.Remove(mountPoint)
		return fmt.Errorf("failed to mount volume: %w", err)
	}

	// 6a. Chown to the host user so createVolumeDirectories can write files.
	// The freshly-formatted ext4 root is owned by root; the host process runs
	// as an unprivileged user and cannot write there without this step.
	hostOwner := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if err := runCmd(30*time.Second, nil, "sudo", "chown", "-R", hostOwner, mountPoint); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "umount", mountPoint)
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(mountPoint)
		os.Remove(volumePath)
		return fmt.Errorf("failed to set volume ownership for bootstrap: %w", err)
	}

	// 7. Create directory structure and config files (shared with macOS)
	if err := createVolumeDirectories(mountPoint, cfg); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "umount", mountPoint)
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(mountPoint)
		os.Remove(volumePath)
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// 6b. Now re-chown to the container's claude user (UID/GID 1000). The
	// Dockerfile creates claude with useradd (no -u flag), which assigns UID
	// 1000 on any fresh Debian-based image. This must happen after directory
	// creation so the container user can write to /claude-env at runtime.
	if err := runCmd(30*time.Second, nil, "sudo", "chown", "-R", "1000:1000", mountPoint); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to set container ownership: %v\n", err)
	}

	// 8. Unmount and close
	if err := m.unmountAndClose(mountPoint, mapperName); err != nil {
		return fmt.Errorf("failed to unmount volume after setup: %w", err)
	}

	return nil
}

func (m *LinuxVolumeManager) Mount(volumePath string, password *terminal.SecurePassword) (string, error) {
	// Reuse existing mount if already open
	if mp := m.GetMountPoint(volumePath); mp != "" {
		return mp, nil
	}

	mapperName := m.generateMapperName(volumePath)
	mapperDev := "/dev/mapper/" + mapperName
	mountPoint := m.generateMountPoint(volumePath)

	// Open the LUKS device (reads passphrase from stdin)
	if err := runCmd(linuxVolumeOperationTimeout, password.Reader(),
		"sudo", "cryptsetup", "luksOpen",
		"--key-file", "-",
		volumePath, mapperName,
	); err != nil {
		return "", fmt.Errorf("failed to open LUKS volume: %w", err)
	}

	// Create mount point directory
	if err := os.MkdirAll(mountPoint, constants.DirPermissions); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		return "", fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
	}

	// Mount the device
	if err := runCmd(linuxVolumeOperationTimeout, nil, "sudo", "mount", mapperDev, mountPoint); err != nil {
		_ = runCmd(30*time.Second, nil, "sudo", "cryptsetup", "luksClose", mapperName)
		os.Remove(mountPoint)
		return "", fmt.Errorf("failed to mount volume: %w", err)
	}

	return mountPoint, nil
}

func (m *LinuxVolumeManager) Unmount(mountPoint string) error {
	if mountPoint == "" {
		mountPoint = m.findAnyMountedVolume()
		if mountPoint == "" {
			return nil // Nothing mounted
		}
	}

	// Derive the mapper name from the mount point suffix (last 12 hex chars after prefix)
	mapperName := ""
	if strings.HasPrefix(mountPoint, LinuxMountPointPrefix) {
		shortHash := strings.TrimPrefix(mountPoint, LinuxMountPointPrefix)
		mapperName = "capsule-" + shortHash
	}

	return m.unmountAndClose(mountPoint, mapperName)
}

func (m *LinuxVolumeManager) Exists(volumePath string) bool {
	_, err := os.Stat(volumePath)
	return err == nil
}

// GetMountPoint returns the active mount point for the given volume path, or ""
// if the volume is not currently mounted.
func (m *LinuxVolumeManager) GetMountPoint(volumePath string) string {
	if volumePath == "" {
		return ""
	}

	mapperName := m.generateMapperName(volumePath) // normalizes path internally

	// Check whether the device mapper entry exists
	if _, err := os.Stat("/dev/mapper/" + mapperName); os.IsNotExist(err) {
		return ""
	}

	// Parse /proc/mounts to find where /dev/mapper/<name> is mounted
	return findMountPointInProcMounts("/dev/mapper/" + mapperName)
}

// volumePathHash returns a 12-character hex string derived from the absolute
// path of the volume file. Resolving to an absolute path first ensures that
// Bootstrap, Mount, and GetMountPoint all produce the same hash regardless of
// how the path was provided.
func volumePathHash(volumePath string) string {
	abs, err := filepath.Abs(volumePath)
	if err != nil {
		abs = volumePath // fall back to the raw path
	}
	hash := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(hash[:])[:12]
}

// generateMapperName returns the device-mapper name for a given volume path.
func (m *LinuxVolumeManager) generateMapperName(volumePath string) string {
	return "capsule-" + volumePathHash(volumePath)
}

// generateMountPoint returns the expected mount point path for a volume.
func (m *LinuxVolumeManager) generateMountPoint(volumePath string) string {
	return LinuxMountPointPrefix + volumePathHash(volumePath)
}

// unmountAndClose runs umount followed by cryptsetup luksClose.
func (m *LinuxVolumeManager) unmountAndClose(mountPoint, mapperName string) error {
	unmountTimeout := 30 * time.Second

	// Flush pending writes
	_ = exec.Command("sync").Run()

	if err := runCmd(unmountTimeout, nil, "sudo", "umount", mountPoint); err != nil {
		return fmt.Errorf("failed to unmount volume at %s: %w", mountPoint, err)
	}

	// Remove the mount point directory we created
	if strings.HasPrefix(mountPoint, LinuxMountPointPrefix) {
		os.Remove(mountPoint)
	}

	if mapperName != "" {
		if err := runCmd(unmountTimeout, nil, "sudo", "cryptsetup", "luksClose", mapperName); err != nil {
			return fmt.Errorf("failed to close LUKS device %s: %w", mapperName, err)
		}
	}

	return nil
}

// findAnyMountedVolume scans /dev/mapper for capsule-* entries and returns the
// first one that has an active mount, as a fallback when no specific volume path
// is known.
func (m *LinuxVolumeManager) findAnyMountedVolume() string {
	entries, err := os.ReadDir("/dev/mapper")
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "capsule-") {
			mp := findMountPointInProcMounts("/dev/mapper/" + entry.Name())
			if mp != "" {
				return mp
			}
		}
	}

	return ""
}

// findMountPointInProcMounts parses /proc/mounts to find the mount point for
// the given device path. Returns "" if not found.
func findMountPointInProcMounts(device string) string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == device {
			return fields[1]
		}
	}

	return ""
}

// runCmd runs a command with a timeout, optionally feeding data to its stdin.
func runCmd(timeout time.Duration, stdin interface{ Read([]byte) (int, error) }, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("command %q timed out after %v", name, timeout)
		}
		return err
	}
	return nil
}
