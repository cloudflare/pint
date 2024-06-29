package config

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOwnersValidate(t *testing.T) {
	owners := Owners{}
	err := owners.validate()
	assert.NoError(t, err)

	owners = Owners{
		Allowed: []string{"^[a-zA-Z]+$", "\\d+"},
	}
	err = owners.validate()
	assert.NoError(t, err)

	owners = Owners{
		Allowed: []string{"["},
	}
	err = owners.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error parsing regexp")

	owners = Owners{
		Allowed: []string{""},
	}
	err = owners.validate()
	assert.NoError(t, err)
}

func TestCompileAllowed(t *testing.T) {
	o1 := Owners{Allowed: []string{}}
	expected1 := []*regexp.Regexp{regexp.MustCompile(".*")}
	result1 := o1.CompileAllowed()
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("Expected %+v but got %+v", expected1, result1)
	}

	o2 := Owners{Allowed: []string{"pattern1", "pattern2"}}
	expected2 := []*regexp.Regexp{
		regexp.MustCompile("^pattern1$"),
		regexp.MustCompile("^pattern2$"),
	}
	result2 := o2.CompileAllowed()
	if !reflect.DeepEqual(result2, expected2) {
		t.Errorf("Expected %+v but got %+v", expected2, result2)
	}
}
