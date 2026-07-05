//go:build !windows

package textfile

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
