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

type CommentValue interface {
	String() string
}

type Comment struct {
	Value  CommentValue
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

type CommentError struct {
	Diagnostic diags.Diagnostic
}

func (ce CommentError) Error() string {
	return ce.Diagnostic.Message
}

type OwnerError struct {
	Diagnostic diags.Diagnostic
}

func (oe OwnerError) Error() string {
	return oe.Diagnostic.Message
}

type Invalid struct {
	Err CommentError
}

func (i Invalid) String() string {
	return i.Err.Error()
}

type Owner struct {
	Name string
	Line int
}

func (o Owner) String() string {
	return o.Name
}

type Disable struct {
	Match string
}

func (d Disable) String() string {
	return d.Match
}

type Snooze struct {
	Until time.Time
	Match string
}

func (s Snooze) String() string {
	return fmt.Sprintf("%s %s", s.Until.Format(time.RFC3339), s.Match)
}

type RuleSet struct {
	Value string
}

func (r RuleSet) String() string {
	return r.Value
}

func parseSnooze(s string) (snz Snooze, err error) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return Snooze{}, fmt.Errorf("invalid snooze comment, expected '$TIME $MATCH' got %q", s)
	}

	snz.Match = parts[1]
	snz.Until, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		snz.Until, err = time.Parse("2006-01-02", parts[0])
	}
	if err != nil {
		return snz, fmt.Errorf("invalid snooze timestamp: %w", err)
	}
	return snz, nil
}

func parseValue(typ Type, s string, line int) (CommentValue, error) {
	switch typ {
	case IgnoreFileType, IgnoreLineType, IgnoreBeginType, IgnoreEndType, IgnoreNextLineType:
		if s != "" {
			return nil, fmt.Errorf("unexpected comment suffix: %q", s)
		}
	case FileOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileOwnerComment)
		}
		return Owner{Name: s, Line: line}, nil
	case RuleOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", RuleOwnerComment)
		}
		return Owner{Name: s, Line: 0}, nil // comment attached to the rule, line numbers are unreliable
	case FileDisableType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileDisableComment)
		}
		return Disable{Match: s}, nil
	case DisableType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", DisableComment)
		}
		return Disable{Match: s}, nil
	case FileSnoozeType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileSnoozeComment)
		}
		return parseSnooze(s)
	case SnoozeType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", SnoozeComment)
		}
		return parseSnooze(s)
	case RuleSetType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", RuleSetComment)
		}
		return RuleSet{Value: s}, nil
	case UnknownType, InvalidComment:
		// pass
	}
	return nil, nil
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

func parseComment(s string, line int) (parsed []Comment) {
	var err error
	var buf strings.Builder
	var c Comment

	state := needsHash
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
		c.Value, err = parseValue(c.Type, strings.TrimSpace(buf.String()), line)
		if err != nil {
			c.Type = InvalidComment
			c.Value = Invalid{
				Err: CommentError{
					Diagnostic: diags.Diagnostic{
						Message: "This comment is not a valid pint control comment: " + err.Error(),
						Pos: diags.PositionRanges{
							{
								Line:        line,
								FirstColumn: c.Offset + 1,
								LastColumn:  len(s),
							},
						},
						FirstColumn: 1,
						LastColumn:  len(s),
					},
				},
			}
		}
		parsed = append(parsed, c)
	}

	return parsed
}

func Parse(lineno int, text string) (comments []Comment) {
	var index int
	for _, line := range strings.Split(text, "\n") {
		comments = append(comments, parseComment(line, lineno+index)...)
		index++
	}
	return comments
}

func Only[T any](src []Comment, typ Type) []T {
	dst := make([]T, 0, len(src))
	for _, c := range src {
		if c.Type != typ {
			continue
		}
		dst = append(dst, c.Value.(T))
	}
	return dst
}

func IsRuleComment(typ Type) bool {
	// nolint:exhaustive
	switch typ {
	case RuleOwnerType, DisableType, SnoozeType, RuleSetType:
		return true
	}
	return false
}
