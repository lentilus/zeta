package parser_test

// Test data constants
const (
	simpleContent  = `Some text with a @simple:reference in it`
	complexContent = `Multiple references:
@first:reference
Some text in between
@second:reference
@nested:reference:with:colons`

	updatedContent = `Updated @new:reference content`
)

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
