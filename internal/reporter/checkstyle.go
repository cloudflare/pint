package reporter

import (
	"fmt"
	"io"
	"strings"
)

func NewCheckStyleReporter(output io.Writer) CheckStyleReporter {
	return CheckStyleReporter{
		output: output,
	}
}

type CheckStyleReporter struct {
	output io.Writer
}

type Dirs map[string][]Report

func (d Dirs) SetDefault(key string, val []Report) (result []Report) {
	if v, ok := d[key]; ok {
		return v
	} else {
		d[key] = val
		return val
	}
}

func sortByFile(summary Summary) Dirs {
	x := make(Dirs)
	for _, report := range summary.reports {
		dir := x.SetDefault(report.Path.Name, make([]Report, 1))
		dir = append(dir, report)
		x[report.Path.Name] = dir
	}
	return x
}

func (cs CheckStyleReporter) Submit(summary Summary) error {

	dirs := sortByFile(summary)

	var buf strings.Builder
	// for _, report := range summary.reports {
	// 	buf.WriteString(fmt.Sprintf("______%s\n", report.Problem.Text))
	// }
	buf.WriteString("<?xml version='1.0' encoding='UTF-8'?>\n")
	buf.WriteString("<checkstyle version='4.3'>\n")

	for dir, reports := range dirs {
		buf.WriteString(fmt.Sprintf("<file name\"%s\" >\n", dir))
		for _, report := range reports {
			line := fmt.Sprintf("<error line=\"%d\" severity=\"%s\" message=\"%s\" source=\"%s\" />\n",
				report.Problem.Lines.First,
				report.Problem.Severity.String(),
				report.Problem.Text,
				report.Problem.Reporter,
			)
			buf.WriteString(line)
		}
		buf.WriteString("</file>\n")
	}
	buf.WriteString("</checkstyle>\n")
	fmt.Fprint(cs.output, buf.String())
	buf.Reset()
	return nil
}
