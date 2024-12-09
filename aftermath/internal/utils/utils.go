package utils

import (
	"crypto/md5"
	"path/filepath"
	"regexp"
	"strings"
)

// ComputeMD5Checksum takes a byte slice and returns the raw MD5 checksum as a byte slice
func ComputeChecksum(content []byte) []byte {
	hash := md5.New()
	hash.Write(content)
	return hash.Sum(nil)
}

func Reference2Path(ref string, base string) (string, error) {
	re := regexp.MustCompile(`^@(.*)`)       // Capture everything after "@" (.*)
	result := re.ReplaceAllString(ref, `$1`) // `$1` refers to the first captured group

	// Replace ":" with "/"
	result = strings.ReplaceAll(result, ":", "/")

	// Add file extension and parent directory
	return filepath.Join(base, result+".typ"), nil
}

func Path2Target(path string, base string) (string, error) {
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return "", err
	}

	// Replace "/" with "."
	target := strings.ReplaceAll(relative, "/", ":")
	re := regexp.MustCompile(`^(.*)\.typ$`)
	result := re.ReplaceAllString(target, `$1`) // `$1` refers to the first captured group
	return result, nil
}
