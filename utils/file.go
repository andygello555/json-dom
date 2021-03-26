package utils

import (
	"os"
)

// Checks if the given path exists and whether the path points to a file
// Returns true if the path exists and the path points to a file, otherwise false
func CheckFileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
