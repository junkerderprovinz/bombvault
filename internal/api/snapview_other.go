//go:build !linux

package api

import "errors"

// enterView refuses outside Linux, where no container runs and no snapshot
// view exists.
func enterView(string) error {
	return errors.New("a snapshot view needs Linux")
}
