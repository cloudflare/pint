package output

import "fmt"

type ColorFn func(format string, a ...any) string

func MaybeColor(fn ColorFn, disabled bool, format string, a ...any) string {
	if disabled {
		return fmt.Sprintf(format, a...)
	}
	return fn(format, a...)
}
