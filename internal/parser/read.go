package parser

import (
	"bufio"
	"errors"
	"io"
	"strings"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
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

func emptyLine(line string, comments []comments.Comment, stripComments bool) string {
	offset := len(line)
	for _, c := range comments {
		offset = c.Offset
		break
	}

	var buf strings.Builder
	for i, r := range line {
		switch {
		case r == '\n':
			buf.WriteRune(r)
		case i < offset || stripComments:
			buf.WriteRune(' ')
		default:
			buf.WriteRune(r)
		}
	}

	return buf.String()
}

type Content struct {
	Ignored      *diags.Diagnostic
	Body         []byte
	FileComments []comments.Comment
	TotalLines   int
}

func ReadContent(r io.Reader) (out Content, err error) {
	reader := bufio.NewReader(r)
	var (
		lineno       int
		line         string
		lineComments []comments.Comment
		found        bool
		skip         skipMode

		skipNext  bool
		autoReset bool
		skipAll   bool
		inBegin   bool
	)

	for {
		lineno++
		line, err = reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			break
		}
		if line != "" {
			out.TotalLines++
		}

		lineComments = comments.Parse(lineno, line)

		if skipAll {
			out.Body = append(out.Body, []byte(emptyLine(line, lineComments, inBegin))...)
		} else {
			skip = skipNone
			found = false
			for _, comment := range lineComments {
				// nolint:exhaustive
				switch comment.Type {
				case comments.IgnoreFileType:
					skip = skipFile
					found = true
					out.Ignored = &diags.Diagnostic{
						Message: "This file was excluded from pint checks.",
						Pos: diags.PositionRanges{
							{
								Line:        lineno,
								FirstColumn: comment.Offset + 1,
								LastColumn:  len(line) - 1,
							},
						},
						FirstColumn: comment.Offset + 1,
						LastColumn:  len(line) - 1,
					}
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
					out.FileComments = append(out.FileComments, comment)
				case comments.RuleOwnerType:
					// pass
				case comments.FileDisableType:
					out.FileComments = append(out.FileComments, comment)
				case comments.DisableType:
					// pass
				case comments.FileSnoozeType:
					out.FileComments = append(out.FileComments, comment)
				case comments.SnoozeType:
					// pass
				case comments.RuleSetType:
					// pass
				case comments.InvalidComment:
					out.FileComments = append(out.FileComments, comment)
				}
			}
			switch {
			case found:
				switch skip {
				case skipNone:
					// no-op
				case skipFile:
					out.Body = append(out.Body, []byte(emptyLine(line, lineComments, inBegin))...)
					skipNext = true
					autoReset = false
					skipAll = true
				case skipCurrentLine:
					out.Body = append(out.Body, []byte(emptyLine(line, lineComments, inBegin))...)
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
				out.Body = append(out.Body, []byte(emptyLine(line, lineComments, inBegin))...)
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
		return out, err
	}

	return out, nil
}
