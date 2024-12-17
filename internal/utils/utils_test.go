package utils_test

import (
	"testing"
	"zeta/internal/utils"
)

func TestReference2Path(t *testing.T) {
	tests := []struct {
		ref      string
		base     string
		expected string
	}{
		{
			ref:      "@example.com/file",
			base:     "/home/user",
			expected: "/home/user/example/com/file.typ", // Test where reference has multiple dots
		},
		{
			ref:      "@user.profile.data",
			base:     "/data",
			expected: "/data/user/profile/data.typ", // Test where reference has multiple dots
		},
		{
			ref:      "@a.b.c",
			base:     "/base",
			expected: "/base/a/b/c.typ", // Test with simple dots in the reference
		},
		{
			ref:      "@singleword",
			base:     "/files",
			expected: "/files/singleword.typ", // Test with a single word after "@" (no dots)
		},
		{
			ref:      "@another.example.path",
			base:     "/test",
			expected: "/test/another/example/path.typ", // Test with multiple dots
		},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got, err := utils.Reference2Path(tt.ref, tt.base)
			if err != nil {
				t.Fatalf("Reference2Path() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("Reference2Path() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPath2Target tests the Path2Target function
func TestPath2Target(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		base     string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple case",
			path:     "/home/user/project/file1.typ",
			base:     "/home/user/project",
			expected: "file1",
			wantErr:  false,
		},
		{
			name:     "nested case",
			path:     "/home/user/project/subdir/file2.typ",
			base:     "/home/user/project",
			expected: "subdir.file2",
			wantErr:  false,
		},
		{
			name:     "path without .typ suffix",
			path:     "/home/user/project/file4",
			base:     "/home/user/project",
			expected: "file4",
			wantErr:  false,
		},
		{
			name:     "empty path",
			path:     "",
			base:     "/home/user/project",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.Path2Target(tt.path, tt.base)
			if (err != nil) != tt.wantErr {
				t.Errorf("Path2Target() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("Path2Target() = %v, want %v", result, tt.expected)
			}
		})
	}
}
