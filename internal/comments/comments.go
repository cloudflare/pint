package comments

import (
	"bufio"
	"fmt"
	"strings"
	"time"
	"unicode"
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

	EmptyComment Comment
)

type CommentValue interface {
	String() string
}

type Comment struct {
	Value CommentValue
	Type  Type
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

type Invalid struct {
	Err error
}

func (i Invalid) String() string {
	return i.Err.Error()
}

type Owner struct {
	Name string
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

	snz = Snooze{Match: parts[1]}
	snz.Until, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		snz.Until, err = time.Parse("2006-01-02", parts[0])
	}
	if err != nil {
		return snz, fmt.Errorf("invalid snooze timestamp: %w", err)
	}
	return snz, nil
}

func parseValue(typ Type, s string) (CommentValue, error) {
	switch typ {
	case IgnoreFileType, IgnoreLineType, IgnoreBeginType, IgnoreEndType, IgnoreNextLineType:
		if s != "" {
			return nil, fmt.Errorf("unexpected comment suffix: %q", s)
		}
	case FileOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", FileOwnerComment)
		}
		return Owner{Name: s}, nil
	case RuleOwnerType:
		if s == "" {
			return nil, fmt.Errorf("missing %s value", RuleOwnerComment)
		}
		return Owner{Name: s}, nil
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
	needsPrefix int = iota
	readsPrefix
	needsType
	readsType
	needsValue
	readsValue
)

func parseComment(s string) (c Comment, err error) {
	var buf strings.Builder

	state := needsPrefix
	for _, r := range s + "\n" {
	READRUNE:
		switch state {
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
					return EmptyComment, nil
				}
				buf.Reset()
				state = needsType
				goto NEXT
			}
			// Invalid character in the prefix, ignore this comment.
			return EmptyComment, nil
		case needsType:
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
				state = needsValue
				goto NEXT
			}
			// Invalid character in the type, ignore this comment.
			return EmptyComment, nil
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

	c.Value, err = parseValue(c.Type, strings.TrimSpace(buf.String()))
	return c, err
}

func Parse(text string) (comments []Comment) {
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		elems := strings.SplitN(sc.Text(), "# ", 2)
		if len(elems) != 2 {
			continue
		}
		c, err := parseComment(elems[1])
		switch {
		case err != nil:
			comments = append(comments, Comment{
				Type:  InvalidComment,
				Value: Invalid{Err: err},
			})
		case c == EmptyComment:
			// pass
		default:
			comments = append(comments, c)
		}
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
