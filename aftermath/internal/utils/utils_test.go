package utils_test

import (
	"aftermath/internal/utils"
	"testing"
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
