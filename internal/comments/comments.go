package comments

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/cloudflare/pint/internal/diags"
)

type Type uint8

const (
	UnknownType Type = iota
	InvalidComment
	IgnoreFileType     // ignore/file
	IgnoreLineType     // ignore/line
	IgnoreBeginType    // ignore/begin
	IgnoreEndType      // ignore/end
	IgnoreNextLineType // ignore/next-line
	FileOwnerType      // file/owner
	RuleOwnerType      // rule/owner
	FileDisableType    // file/disable
	DisableType        // disable
	FileSnoozeType     // file/snooze
	SnoozeType         // snooze
	RuleSetType        // rule/set
)

var (
	Prefix = "pint"

	IgnoreFileComment     = "ignore/file"
	IgnoreLineComment     = "ignore/line"
	IgnoreBeginComment    = "ignore/begin"
	IgnoreEndComment      = "ignore/end"
	IgnoreNextLineComment = "ignore/next-line"
	FileOwnerComment      = "file/owner"
	RuleOwnerComment      = "rule/owner"
	FileDisableComment    = "file/disable"
	DisableComment        = "disable"
	FileSnoozeComment     = "file/snooze"
	SnoozeComment         = "snooze"
	RuleSetComment        = "rule/set"
)

type Value interface {
	String() string
}

type Comment struct {
	Value  Value
	Type   Type
	Offset int
}

func parseType(s string) Type {
	switch s {
	case IgnoreFileComment:
		return IgnoreFileType
	case IgnoreLineComment:
		return IgnoreLineType
	case IgnoreBeginComment:
		return IgnoreBeginType
	case IgnoreEndComment:
		return IgnoreEndType
	case IgnoreNextLineComment:
		return IgnoreNextLineType
	case FileOwnerComment:
		return FileOwnerType
	case RuleOwnerComment:
		return RuleOwnerType
	case FileDisableComment:
		return FileDisableType
	case DisableComment:
		return DisableType
	case FileSnoozeComment:
		return FileSnoozeType
	case SnoozeComment:
		return SnoozeType
	case RuleSetComment:
		return RuleSetType
	default:
		return UnknownType
	}
}

type Error struct {
	Diagnostic diags.Diagnostic
}

func (ce Error) Error() string {
	return ce.Diagnostic.Message
}

type OwnerError struct {
	Diagnostic diags.Diagnostic
}

func (oe OwnerError) Error() string {
	return oe.Diagnostic.Message
}

type Invalid struct {
	Err Error
}

func (i Invalid) String() string {
	return i.Err.Error()
}

type Position struct {
	Pos    diags.PositionRanges
	Offset int
}

type Owner struct {
	Name string
	Position
}

func (o Owner) String() string {
	return o.Name
}

type Disable struct {
	Match string
	Position
}

func (d Disable) String() string {
	return d.Match
}

type Snooze struct {
	Until time.Time
	Match string
	Position
}

func (s Snooze) String() string {
	return s.Until.Format(time.RFC3339) + " " + s.Match
}

type RuleSet struct {
	Value string
	Position
}

func (r RuleSet) String() string {
	return r.Value
}

func parseSnooze(s string, pos diags.PositionRanges, offset int) (snz Snooze, err error) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return Snooze{}, fmt.Errorf("invalid snooze comment, expected '$TIME $MATCH' got %q", s)
	}

	snz.Match = parts[1]
	snz.Position = Position{Pos: pos, Offset: offset + len(parts[0]) + 1}
	snz.Until, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		snz.Until, err = time.Parse("2006-01-02", parts[0])
	}
	if err != nil {
		return snz, fmt.Errorf("invalid snooze timestamp: %w", err)
	}
	return snz, nil
}

func parseValue(commentType Type, s string, pos diags.PositionRanges, offset int) (Value, error) {
	// nolint:exhaustive
	switch commentType {
	case IgnoreFileType, IgnoreLineType, IgnoreBeginType, IgnoreEndType, IgnoreNextLineType:
		if s != "" {
			return nil, fmt.Errorf("unexpected comment suffix: %q", s)
		}
		return nil, nil
	case FileOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileOwnerComment)
		}
		return Owner{
			Name:     s,
			Position: Position{Pos: pos, Offset: offset},
		}, nil
	case RuleOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", RuleOwnerComment)
		}
		return Owner{
			Name:     s,
			Position: Position{Pos: pos, Offset: offset},
		}, nil
	case FileDisableType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileDisableComment)
		}
		return Disable{
			Match:    s,
			Position: Position{Pos: pos, Offset: offset},
		}, nil
	case DisableType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", DisableComment)
		}
		return Disable{
			Match:    s,
			Position: Position{Pos: pos, Offset: offset},
		}, nil
	case FileSnoozeType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileSnoozeComment)
		}
		return parseSnooze(s, pos, offset)
	case SnoozeType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", SnoozeComment)
		}
		return parseSnooze(s, pos, offset)
	case RuleSetType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", RuleSetComment)
		}
		return RuleSet{
			Value:    s,
			Position: Position{Pos: pos, Offset: offset},
		}, nil
	case UnknownType, InvalidComment:
		// these are never passed here
		return nil, nil
	default:
		// this is not reachable
		return nil, nil
	}
}

const (
	needsHash uint8 = iota
	needsPrefix
	readsPrefix
	needsType
	readsType
	needsValue
	readsValue
)

func parseComment(s string, line, columnOffset int) (parsed []Comment) {
	var (
		err         error
		buf         strings.Builder
		c           Comment
		valueOffset int
		state       = needsHash
	)

	for i, r := range s + "\n" {
	READRUNE:
		switch state {
		case needsHash:
			if r != '#' {
				goto NEXT
			}
			state = needsPrefix
			buf.Reset()
			c.Type = UnknownType
			c.Value = nil
			c.Offset = i
		case needsPrefix:
			if unicode.IsSpace(r) {
				goto NEXT
			}
			state = readsPrefix
			goto READRUNE
		case readsPrefix:
			if unicode.IsLetter(r) {
				_, _ = buf.WriteRune(r)
				goto NEXT
			}
			if unicode.IsSpace(r) {
				// Invalid comment prefix, ignore this comment.
				if buf.String() != Prefix {
					buf.Reset()
					state = needsHash
					goto NEXT
				}
				buf.Reset()
				state = needsType
				goto NEXT
			}
			// Invalid character in the prefix, ignore this comment.
			state = needsHash
		case needsType:
			if r == '#' {
				state = needsHash
				goto READRUNE
			}
			if unicode.IsSpace(r) {
				goto NEXT
			}
			state = readsType
			goto READRUNE
		case readsType:
			if unicode.IsLetter(r) || r == '/' || r == '-' {
				_, _ = buf.WriteRune(r)
				goto NEXT
			}
			if unicode.IsSpace(r) || r == '\n' {
				c.Type = parseType(buf.String())
				buf.Reset()
				if c.Type == UnknownType {
					state = needsHash
				} else {
					state = needsValue
				}

			}
		case needsValue:
			if unicode.IsSpace(r) {
				goto NEXT
			}
			valueOffset = i
			state = readsValue
			goto READRUNE
		case readsValue:
			if r == '\n' {
				goto NEXT
			}
			_, _ = buf.WriteRune(r)
		}
	NEXT:
	}

	if c.Type != UnknownType {
		pos := diags.PositionRanges{
			{
				Line:        line,
				FirstColumn: columnOffset + c.Offset + 1,
				LastColumn:  columnOffset + len(s),
			},
		}
		c.Value, err = parseValue(c.Type, strings.TrimSpace(buf.String()), pos, valueOffset)
		if err != nil {
			c.Type = InvalidComment
			c.Value = Invalid{
				Err: Error{
					Diagnostic: diags.Diagnostic{
						Message:     "This comment is not a valid pint control comment: " + err.Error(),
						Pos:         pos,
						Expr:        nil,
						FirstColumn: 1,
						LastColumn:  len(s),
						Kind:        diags.Issue,
					},
				},
			}
		}
		parsed = append(parsed, c)
	}

	return parsed
}

func Parse(lineno int, text string, columnOffset int) (comments []Comment) {
	var index int
	for line := range strings.SplitSeq(text, "\n") {
		comments = append(comments, parseComment(line, lineno+index, columnOffset)...)
		index++
	}
	return comments
}

func Only[T any](src []Comment, commentType Type) []T {
	dst := make([]T, 0, len(src))
	for _, c := range src {
		if c.Type != commentType {
			continue
		}
		dst = append(dst, c.Value.(T))
	}
	return dst
}

func IsRuleComment(commentType Type) bool {
	// nolint:exhaustive
	switch commentType {
	case RuleOwnerType, DisableType, SnoozeType, RuleSetType:
		return true
	}
	return false
}
