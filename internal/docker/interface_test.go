package docker

import (
	"testing"
)

func TestValidateDockerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "mycontainer", false},
		{"valid with hyphen", "my-container", false},
		{"valid with underscore", "my_container", false},
		{"valid with period", "my.container", false},
		{"valid with tag", "myimage:latest", false},
		{"valid alphanumeric start", "1container", false},
		{"empty string", "", true},
		{"starts with hyphen", "-container", true},
		{"starts with period", ".container", true},
		{"starts with underscore", "_container", true},
		{"contains space", "my container", true},
		{"contains slash", "my/container", true},
		{"contains colon only", ":", true},
		{"too long", repeatStr("a", 129), true},
		{"exactly max length", repeatStr("a", 128), false},
		{"tag stripped for validation", "image:v1.0", false},
		{"valid complex", "my-image.v2_test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDockerName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDockerName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		fieldName string
		wantErr   bool
	}{
		{"valid absolute path", "/home/user/project", "workspace", false},
		{"empty path", "", "workspace", true},
		{"relative path", "relative/path", "workspace", true},
		// Note: filepath.Clean resolves ".." in absolute paths, so "/a/../b" becomes "/b".
		// The ".." check only catches cases Clean can't resolve (none for absolute paths).
		// Relative paths are caught by the IsAbs check before reaching the ".." check.
		{"cleaned traversal passes", "/home/user/../etc/passwd", "workspace", false},
		{"root path", "/", "workspace", false},
		{"deep path", "/a/b/c/d/e/f", "workspace", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q, %q) error = %v, wantErr %v", tt.path, tt.fieldName, err, tt.wantErr)
			}
		})
	}
}

func TestContainerConfig_Validate(t *testing.T) {
	validConfig := ContainerConfig{
		ImageName:        "claude-capsule:latest",
		ContainerName:    "claude-a1b2c3d4",
		VolumeMountPoint: "/Volumes/Capsule-abc123",
		WorkspacePath:    "/Users/test/project",
	}

	t.Run("valid config", func(t *testing.T) {
		if err := validConfig.Validate(); err != nil {
			t.Errorf("Validate() unexpected error: %v", err)
		}
	})

	t.Run("invalid image name", func(t *testing.T) {
		cfg := validConfig
		cfg.ImageName = ""
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() expected error for empty image name")
		}
	})

	t.Run("invalid container name", func(t *testing.T) {
		cfg := validConfig
		cfg.ContainerName = "-bad"
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() expected error for invalid container name")
		}
	})

	t.Run("relative mount point", func(t *testing.T) {
		cfg := validConfig
		cfg.VolumeMountPoint = "relative/path"
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() expected error for relative mount point")
		}
	})

	t.Run("relative workspace path", func(t *testing.T) {
		cfg := validConfig
		cfg.WorkspacePath = "relative/path"
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() expected error for relative workspace path")
		}
	})
}

func repeatStr(s string, n int) string {
	result := make([]byte, 0, n*len(s))
	for range n {
		result = append(result, s...)
	}
	return string(result)
}
