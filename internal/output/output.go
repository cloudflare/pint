package output

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

var (
	redColor     = color.New(color.FgRed)
	blueColor    = color.New(color.FgCyan)
	whiteColor   = color.New(color.FgWhite)
	yellowColor  = color.New(color.FgYellow)
	magnetaColor = color.New(color.FgMagenta)
	grayColor    = color.New(color.FgHiBlack)
)

func Info(output io.Writer, msg string) {
	fmt.Fprintln(output, msg)
}

func Error(output io.Writer, err error) {
	redColor.Fprintln(output, err.Error())
}

func MakeRed(format string, a ...interface{}) string {
	return redColor.Sprintf(format, a...)
}

func MakeBlue(format string, a ...interface{}) string {
	return blueColor.Sprintf(format, a...)
}

func MakeWhite(format string, a ...interface{}) string {
	return whiteColor.Sprintf(format, a...)
}

func MakeYellow(format string, a ...interface{}) string {
	return yellowColor.Sprintf(format, a...)
}

func MakeMagneta(format string, a ...interface{}) string {
	return magnetaColor.Sprintf(format, a...)
}

func MakeGray(format string, a ...interface{}) string {
	return grayColor.Sprintf(format, a...)
}
