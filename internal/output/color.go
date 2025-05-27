package output

import "fmt"

type Color uint8

const (
	None    Color = 0
	Bold    Color = 1
	Dim     Color = 2
	Black   Color = 90
	Red     Color = 91
	Yellow  Color = 93
	Blue    Color = 94
	Magenta Color = 95
	Cyan    Color = 96
	White   Color = 97
)

func MaybeColor(color Color, disabled bool, s string) string {
	if disabled {
		return s
	}
	return fmt.Sprintf("\033[%dm%s\033[0m", color, s)
}
