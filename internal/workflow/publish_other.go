//go:build !linux || !amd64

package workflow

import "errors"

func publishDirectory(source, target string) error {
	return errors.New("atomic snapshot publication requires the supported Linux amd64 profile")
}
