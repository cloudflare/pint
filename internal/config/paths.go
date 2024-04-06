package config

import (
	"regexp"
)

func ValidatePaths(paths []string) error {
	for _, pattern := range paths {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	return nil
}
