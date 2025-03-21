package parser

import (
	"bufio"
	"io"

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

func NewContentReader(r io.Reader) *ContentReader {
	return &ContentReader{
		src:         bufio.NewReader(r),
		buf:         nil,
		comments:    nil,
		diagnostics: nil,
		lineno:      0,
		skipAll:     false,
		skipNext:    false,
		autoReset:   false,
		inBegin:     false,
	}
}

type ContentReader struct {
	src         *bufio.Reader
	buf         []byte
	comments    []comments.Comment
	diagnostics []diags.Diagnostic
	lineno      int

	skipAll   bool
	skipNext  bool
	autoReset bool
	inBegin   bool
}

func (r *ContentReader) Comments() []comments.Comment {
	return r.comments
}

func (r *ContentReader) Diagnostics() []diags.Diagnostic {
	return r.diagnostics
}

func (r *ContentReader) Ignored() bool {
	return r.skipAll
}

func (r *ContentReader) TotalLines() int {
	return r.lineno
}

func (r *ContentReader) Read(b []byte) (got int, err error) {
	for {
		if len(r.buf) == 0 {
			err = r.readNextLine()
		}
		n := copy(b[got:], r.buf)
		r.buf = r.buf[n:]
		got += n
		if got >= cap(b) || err != nil {
			break
		}
	}
	return got, err
}

func (r *ContentReader) readNextLine() (err error) {
	r.buf, err = r.src.ReadBytes('\n')
	if len(r.buf) == 0 {
		return err
	}

	r.lineno++
	r.parseComments()
	return err
}

func (r *ContentReader) parseComments() {
	lineComments := comments.Parse(r.lineno, string(r.buf))

	if r.skipAll {
		r.emptyCurrentLine(lineComments)
		return
	}

	var found bool
	var skip skipMode
	for _, comment := range lineComments {
		// nolint:exhaustive
		switch comment.Type {
		case comments.IgnoreFileType:
			skip = skipFile
			found = true
			r.diagnostics = append(r.diagnostics, diags.Diagnostic{
				Message: "This file was excluded from pint checks.",
				Pos: diags.PositionRanges{
					{
						Line:        r.lineno,
						FirstColumn: comment.Offset + 1,
						LastColumn:  len(r.buf) - 1,
					},
				},
				FirstColumn: comment.Offset + 1,
				LastColumn:  len(r.buf) - 1,
			})
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
			r.comments = append(r.comments, comment)
		case comments.RuleOwnerType:
			// pass
		case comments.FileDisableType:
			r.comments = append(r.comments, comment)
		case comments.DisableType:
			// pass
		case comments.FileSnoozeType:
			r.comments = append(r.comments, comment)
		case comments.SnoozeType:
			// pass
		case comments.RuleSetType:
			// pass
		case comments.InvalidComment:
			r.comments = append(r.comments, comment)
		}
	}
	switch {
	case found:
		switch skip { // nolint: exhaustive
		case skipFile:
			r.emptyCurrentLine(lineComments)
			r.skipNext = true
			r.autoReset = false
			r.skipAll = true
		case skipCurrentLine:
			r.emptyCurrentLine(lineComments)
			if !r.inBegin {
				r.skipNext = false
				r.autoReset = true
			}
		case skipNextLine:
			r.skipNext = true
			r.autoReset = true
		case skipBegin:
			r.skipNext = true
			r.autoReset = false
			r.inBegin = true
		case skipEnd:
			r.skipNext = false
			r.autoReset = true
			r.inBegin = false
		}
	case r.skipNext:
		r.emptyCurrentLine(lineComments)
		if r.autoReset {
			r.skipNext = false
		}
	}
}

func (r *ContentReader) emptyCurrentLine(comments []comments.Comment) {
	offset := len(r.buf)
	for _, c := range comments {
		offset = c.Offset
		break
	}

	buf := make([]byte, 0, len(r.buf))
	for i, c := range r.buf {
		switch {
		case c == '\n':
			buf = append(buf, c)
		case i < offset || r.inBegin:
			buf = append(buf, ' ')
		default:
			buf = append(buf, c)
		}
	}
	r.buf = buf
}
