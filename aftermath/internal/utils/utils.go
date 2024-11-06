package utils

import (
	"crypto/md5"
)

// ComputeMD5Checksum takes a byte slice and returns the raw MD5 checksum as a byte slice
func ComputeChecksum(content []byte) []byte {
	hash := md5.New()
	hash.Write(content)
	return hash.Sum(nil)
}
