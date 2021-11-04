package parser

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

type skipMode int

const (
	skipNone skipMode = iota
	skipNextLine
	skipBegin
	skipEnd
	skipCurrentLine
	skipFile
)

func removeRedundantSpaces(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

func emptyLine(line string) (emptied string) {
	preComment := strings.TrimSuffix(line, "\n")
	var comment string
	if commentStart := strings.IndexRune(line, '#'); commentStart >= 0 {
		comment = preComment[commentStart:]
		preComment = preComment[:commentStart]
	}

	emptied = strings.Repeat(" ", len(preComment)) + comment

	if strings.HasSuffix(line, "\n") {
		emptied += "\n"
	}

	return
}

func hasComment(line, comment string) bool {
	if !strings.Contains(line, "#") {
		return false
	}

	elems := strings.Split(strings.TrimSuffix(line, "\n"), "#")
	lastComment := elems[len(elems)-1]
	parts := strings.SplitN(removeRedundantSpaces(lastComment), " ", 2)
	if len(parts) < 2 {
		return false
	}

	return parts[0] == "pint" && parts[1] == comment
}

func parseSkipComment(line string) (skipMode, bool) {
	if hasComment(line, "ignore/file") {
		return skipFile, true
	} else if hasComment(line, "ignore/line") {
		return skipCurrentLine, true
	} else if hasComment(line, "ignore/next-line") {
		return skipNextLine, true
	} else if hasComment(line, "ignore/begin") {
		return skipBegin, true
	} else if hasComment(line, "ignore/end") {
		return skipEnd, true
	}
	return skipNone, false
}

func ReadContent(r io.Reader) (out []byte, err error) {
	reader := bufio.NewReader(r)
	var line string
	var found bool
	var skip skipMode

	var skipNext bool
	var autoReset bool
	var skipAll bool
	var inBegin bool
	for {
		line, err = reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			break
		}

		if skipAll {
			out = append(out, []byte(emptyLine(line))...)
		} else {
			skip, found = parseSkipComment(line)
			if found {
				switch skip {
				case skipFile:
					out = append(out, []byte(emptyLine(line))...)
					skipNext = true
					autoReset = false
					skipAll = true
				case skipCurrentLine:
					out = append(out, []byte(emptyLine(line))...)
					if !inBegin {
						skipNext = false
						autoReset = true
					}
				case skipNextLine:
					out = append(out, []byte(line)...)
					skipNext = true
					autoReset = true
				case skipBegin:
					out = append(out, []byte(line)...)
					skipNext = true
					autoReset = false
					inBegin = true
				case skipEnd:
					out = append(out, []byte(line)...)
					skipNext = false
					autoReset = true
					inBegin = false
				}
			} else if skipNext {
				out = append(out, []byte(emptyLine(line))...)
				if autoReset {
					skipNext = false
				}
			} else {
				out = append(out, []byte(line)...)
			}
		}

		if err != nil {
			break
		}
	}

	if !errors.Is(err, io.EOF) {
		return nil, err
	}

	return out, nil
}
