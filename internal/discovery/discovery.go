package discovery

type File struct {
	Path    string
	Commits []string
}

type FileFindResults interface {
	Results() []File
	Paths() []string
	Commits() []string
	HasCommit(commit string) bool
}

type FileFinder interface {
	Find(...string) (FileFindResults, error)
}

type Line struct {
	Path     string
	Position int
	Commit   string
}

type LineFindResults interface {
	Results() []Line
	HasLines(lines []int) bool
}

type LineFinder interface {
	Find(string) (LineFindResults, error)
}
