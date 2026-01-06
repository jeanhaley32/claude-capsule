package constants

import "os"

// Volume-related constants
const (
	// MacOSVolumeFile is the filename for the encrypted volume on macOS.
	MacOSVolumeFile = "claude-env.sparseimage"

	// MacOSMountPoint is the default mount point for the encrypted volume on macOS.
	MacOSMountPoint = "/Volumes/ClaudeEnv"

	// MacOSVolumeName is the volume label used when creating the encrypted volume.
	MacOSVolumeName = "ClaudeEnv"

	// LinuxVolumeFile is the filename for the encrypted volume on Linux.
	LinuxVolumeFile = "claude-env.img"

	// LinuxMountPoint is the default mount point for the encrypted volume on Linux.
	LinuxMountPoint = "/tmp/claude-env-mount"
)

// Shadow documentation constants
const (
	// DocsSymlinkName is the name of the shadow documentation directory.
	DocsSymlinkName = "_docs"
)

// Volume size limits
const (
	// MinVolumeSizeGB is the minimum volume size in gigabytes.
	MinVolumeSizeGB = 1
	// MaxVolumeSizeGB is the maximum volume size in gigabytes.
	MaxVolumeSizeGB = 100
)

// File permissions
const (
	// DirPermissions is the default permission mode for directories.
	DirPermissions os.FileMode = 0755

	// FilePermissions is the default permission mode for sensitive files.
	FilePermissions os.FileMode = 0600
)
