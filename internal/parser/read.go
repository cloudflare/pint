package parser

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"github.com/cloudflare/pint/internal/comments"
)

type skipMode uint8

const (
	skipNone skipMode = iota
	skipNextLine
	skipBegin
	skipEnd
	skipCurrentLine
	skipFile
)

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

	return emptied
}

type Content struct {
	Body    []byte
	Ignored bool
}

func ReadContent(r io.Reader) (out Content, fileComments []comments.Comment, err error) {
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
			out.Body = append(out.Body, []byte(emptyLine(line))...)
		} else {
			skip = skipNone
			found = false
			for _, comment := range comments.Parse(line) {
				// nolint:exhaustive
				switch comment.Type {
				case comments.IgnoreFileType:
					skip = skipFile
					found = true
				case comments.IgnoreLineType:
					skip = skipCurrentLine
					found = true
				case comments.IgnoreBeginType:
					skip = skipBegin
					found = true
				case comments.IgnoreEndType:
					skip = skipEnd
					found = true
				case comments.IgnoreNextLineType:
					skip = skipNextLine
					found = true
				case comments.FileOwnerType:
					fileComments = append(fileComments, comment)
				case comments.RuleOwnerType:
					// pass
				case comments.FileDisableType:
					fileComments = append(fileComments, comment)
				case comments.DisableType:
					// pass
				case comments.FileSnoozeType:
					fileComments = append(fileComments, comment)
				case comments.SnoozeType:
					// pass
				case comments.RuleSetType:
					// pass
				case comments.InvalidComment:
					fileComments = append(fileComments, comment)
				}
			}
			switch {
			case found:
				switch skip {
				case skipNone:
					// no-op
				case skipFile:
					out.Ignored = true
					out.Body = append(out.Body, []byte(emptyLine(line))...)
					skipNext = true
					autoReset = false
					skipAll = true
				case skipCurrentLine:
					out.Body = append(out.Body, []byte(emptyLine(line))...)
					if !inBegin {
						skipNext = false
						autoReset = true
					}
				case skipNextLine:
					out.Body = append(out.Body, []byte(line)...)
					skipNext = true
					autoReset = true
				case skipBegin:
					out.Body = append(out.Body, []byte(line)...)
					skipNext = true
					autoReset = false
					inBegin = true
				case skipEnd:
					out.Body = append(out.Body, []byte(line)...)
					skipNext = false
					autoReset = true
					inBegin = false
				}
			case skipNext:
				out.Body = append(out.Body, []byte(emptyLine(line))...)
				if autoReset {
					skipNext = false
				}
			default:
				out.Body = append(out.Body, []byte(line)...)
			}
		}

		if err != nil {
			break
		}
	}

	if !errors.Is(err, io.EOF) {
		return out, fileComments, err
	}

	return out, fileComments, nil
}
