package reporter

import (
	"bytes"
	"encoding/xml"
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

func sortByFile(summary Summary) Dirs {
	x := make(Dirs)
	for _, report := range summary.reports {
		x[report.Path.Name] = append(x[report.Path.Name], report)
	}
	return x
}

func (cs CheckStyleReporter) Submit(summary Summary) error {
	dirs := sortByFile(summary)
	var buf strings.Builder
	buf.WriteString("<?xml version='1.0' encoding='UTF-8'?>\n")
	buf.WriteString("<checkstyle version='4.3'>\n")

	for dir, reports := range dirs {
		buf.WriteString(fmt.Sprintf("<file name=\"%s\" >\n", dir))
		for _, report := range reports {
			// xml excape message
			xmlMessageBuf := bytes.Buffer{}
			textDetails := fmt.Sprintf("Text:%s\n Details:%s", report.Problem.Text, report.Problem.Details)
			xml.EscapeText(&xmlMessageBuf, []byte(textDetails))
			// xml escape reporter
			xmlReporterBuf := bytes.Buffer{}
			xml.EscapeText(&xmlReporterBuf, []byte(report.Problem.Reporter))
			line := fmt.Sprintf("<error line=\"%d\" severity=\"%s\" message=\"%s\" source=\"%s\" />\n",
				report.Problem.Lines.First,
				report.Problem.Severity.String(),
				xmlMessageBuf.String(),
				xmlReporterBuf.String(),
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
