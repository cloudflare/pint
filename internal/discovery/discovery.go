package discovery

import (
	"os"

	"github.com/cloudflare/pint/internal/parser"

	"github.com/rs/zerolog/log"
)

type RuleFinder interface {
	Find() ([]Entry, error)
}

type Entry struct {
	Path          string
	PathError     error
	ModifiedLines []int
	Rule          parser.Rule
}

func readFile(path string) (entries []Entry, err error) {
	p := parser.NewParser()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	content, err := parser.ReadContent(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	rules, err := p.Parse(content)
	if err != nil {
		log.Error().Str("path", path).Err(err).Msg("Failed to parse file content")
		entries = append(entries, Entry{
			Path:      path,
			PathError: err,
		})
		return entries, nil
	}

	for _, rule := range rules {
		entries = append(entries, Entry{
			Path: path,
			Rule: rule,
		})
	}

	log.Info().Str("path", path).Int("rules", len(entries)).Msg("File parsed")
	return entries, nil
}
