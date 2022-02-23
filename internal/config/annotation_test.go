package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnotationSettings(t *testing.T) {
	type testCaseT struct {
		conf AnnotationSettings
		err  error
	}

	testCases := []testCaseT{
		{
			conf: AnnotationSettings{
				Key: "summary",
			},
		},
		{
			conf: AnnotationSettings{},
			err:  errors.New("annotation key cannot be empty"),
		},
		{
			conf: AnnotationSettings{
				Key: ".++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: AnnotationSettings{
				Key:   ".+",
				Value: ".++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: AnnotationSettings{
				Key: "{{nil}}",
			},
			err: errors.New(`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`),
		},
		{
			conf: AnnotationSettings{
				Key:   ".+",
				Value: "{{nil}}",
			},
			err: errors.New(`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`),
		},
		{
			conf: AnnotationSettings{
				Key:      ".+",
				Severity: "foo",
			},
			err: errors.New("unknown severity: foo"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			assert := assert.New(t)
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				assert.Equal(err, tc.err)
			} else {
				assert.EqualError(err, tc.err.Error())
			}
		})
	}
}
