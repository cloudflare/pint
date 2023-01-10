package parser

import (
	"strings"
	"time"
)

type Snooze struct {
	Until time.Time
	Text  string
}

func ParseSnooze(comment string) *Snooze {
	parts := strings.SplitN(comment, " ", 2)
	if len(parts) != 2 {
		return nil
	}

	s := Snooze{Text: parts[1]}

	var err error

	s.Until, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		s.Until, err = time.Parse("2006-01-02", parts[0])
	}
	if err != nil {
		return nil
	}

	if !s.Until.After(time.Now()) {
		return nil
	}

	return &s
}
