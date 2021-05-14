// Package tools implements the sub-commands for the artifact CLI.
package tools

import "go.viam.com/core/artifact"

// Pull ensures all artifacts in the global cache tree are present locally.
func Pull() error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	_, err = cache.Ensure("/")
	return err
}