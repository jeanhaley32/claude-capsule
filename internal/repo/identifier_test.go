package repo

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "my-repo",
			expected: "my-repo",
		},
		{
			name:     "path separators become hyphens",
			input:    "github.com/user/repo",
			expected: "github.com-user-repo",
		},
		{
			name:     "colons and backslashes become hyphens",
			input:    `host:path\to\repo`,
			expected: "host-path-to-repo",
		},
		{
			name:     "whitespace becomes hyphens",
			input:    "my repo name",
			expected: "my-repo-name",
		},
		{
			name:     "unsafe chars removed",
			input:    "repo!@#$%name",
			expected: "repo-name",
		},
		{
			name:     "multiple hyphens collapsed",
			input:    "repo---name",
			expected: "repo-name",
		},
		{
			name:     "leading and trailing hyphens trimmed",
			input:    "-repo-name-",
			expected: "repo-name",
		},
		{
			name:     "empty string returns default",
			input:    "",
			expected: "unknown-repo",
		},
		{
			name:     "only unsafe chars returns default",
			input:    "!!!###",
			expected: "unknown-repo",
		},
		{
			name:     "only separators returns default",
			input:    "///:::@@@",
			expected: "unknown-repo",
		},
		{
			name:     "periods preserved",
			input:    "github.com",
			expected: "github.com",
		},
		{
			name:     "underscores preserved",
			input:    "my_repo_name",
			expected: "my_repo_name",
		},
		{
			name:     "all unsafe chars returns default",
			input:    string(make([]byte, 200)),
			expected: "unknown-repo", // 200 null bytes → all removed → empty → default
		},
		{
			name:     "long valid name truncated to 100",
			input:    repeatString("a", 50) + "-" + repeatString("b", 60),
			expected: repeatString("a", 50) + "-" + repeatString("b", 49),
		},
		{
			name:     "unicode removed",
			input:    "répo-名前",
			expected: "rpo",
		},
		{
			name:     "mixed safe and unsafe",
			input:    "user@host:project/sub.git",
			expected: "user-host-project-sub.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeName_MaxLength(t *testing.T) {
	// A name exactly at max length should not be truncated
	name := repeatString("a", maxIdentifierLength)
	got := sanitizeName(name)
	if len(got) != maxIdentifierLength {
		t.Errorf("sanitizeName(%d chars) length = %d, want %d", maxIdentifierLength, len(got), maxIdentifierLength)
	}

	// A name over max length should be truncated
	name = repeatString("a", maxIdentifierLength+10)
	got = sanitizeName(name)
	if len(got) != maxIdentifierLength {
		t.Errorf("sanitizeName(%d chars) length = %d, want %d", maxIdentifierLength+10, len(got), maxIdentifierLength)
	}
}

func TestSanitizeName_TruncationNoTrailingHyphen(t *testing.T) {
	// Create a name where truncation would leave a trailing hyphen
	name := repeatString("a", maxIdentifierLength-1) + "-bbb"
	got := sanitizeName(name)
	if got[len(got)-1] == '-' {
		t.Errorf("sanitizeName() result ends with hyphen: %q", got)
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "HTTPS URL",
			url:      "https://github.com/user/repo.git",
			expected: "github.com-user-repo",
		},
		{
			name:     "HTTP URL",
			url:      "http://github.com/user/repo.git",
			expected: "github.com-user-repo",
		},
		{
			name:     "SSH URL",
			url:      "git@github.com:user/repo.git",
			expected: "github.com-user-repo",
		},
		{
			name:     "git protocol URL",
			url:      "git://github.com/user/repo.git",
			expected: "github.com-user-repo",
		},
		{
			name:     "no .git suffix",
			url:      "https://github.com/user/repo",
			expected: "github.com-user-repo",
		},
		{
			name:     "nested path",
			url:      "https://gitlab.com/group/subgroup/repo.git",
			expected: "gitlab.com-group-subgroup-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRemoteURL(tt.url)
			if got != tt.expected {
				t.Errorf("normalizeRemoteURL(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

// Helper to repeat a string n times
func repeatString(s string, n int) string {
	result := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		result = append(result, s...)
	}
	return string(result)
}

