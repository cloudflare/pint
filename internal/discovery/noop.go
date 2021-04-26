package discovery

type AlwaysPresentLines struct{}

func (apl AlwaysPresentLines) Results() []Line {
	return nil
}

func (apl AlwaysPresentLines) HasLines(lines []int) bool {
	return true
}

type NoopLineFinder struct{}

func (nlf NoopLineFinder) Find(string) (LineFindResults, error) {
	return AlwaysPresentLines{}, nil
}
