package reporter

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

	"github.com/cloudflare/pint/internal/checks"
)

func NewCheckStyleReporter(output io.Writer) CheckStyleReporter {
	return CheckStyleReporter{
		output: output,
	}
}

type CheckStyleReporter struct {
	output io.Writer
}

type checkstyleReport struct {
	reportsPerPath [][]Report
}

func (d checkstyleReport) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	_ = e.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "checkstyle"},
		Attr: []xml.Attr{
			{
				Name:  xml.Name{Local: "version"},
				Value: "4.3",
			},
		},
	})
	for _, reports := range d.reportsPerPath {
		if len(reports) == 0 {
			continue
		}
		_ = e.EncodeToken(
			xml.StartElement{
				Name: xml.Name{Local: "file"},
				Attr: []xml.Attr{
					{
						Name:  xml.Name{Local: "name"},
						Value: reports[0].Path.Name,
					},
				},
			},
		)
		for _, report := range reports {
			_ = e.Encode(report)
		}
		_ = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "file"}})
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "checkstyle"}})
}

func (r Report) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	msg := r.Problem.Summary
	if r.Problem.Details != "" {
		msg += "\n" + r.Problem.Details
	}

	var checkstyleSeverity string
	switch r.Problem.Severity {
	case checks.Information:
		checkstyleSeverity = "info"
	case checks.Warning:
		checkstyleSeverity = "warning"
	case checks.Bug:
		checkstyleSeverity = "error"
	case checks.Fatal:
		checkstyleSeverity = "error"
	}

	_ = e.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "error"},
		Attr: []xml.Attr{
			{
				Name:  xml.Name{Local: "line"},
				Value: strconv.Itoa(r.Problem.Lines.First),
			},
			{
				Name:  xml.Name{Local: "severity"},
				Value: checkstyleSeverity,
			},
			{
				Name:  xml.Name{Local: "message"},
				Value: msg,
			},
			{
				Name:  xml.Name{Local: "source"},
				Value: r.Problem.Reporter,
			},
		},
	})
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "error"}})
}

func (cs CheckStyleReporter) Submit(_ context.Context, summary Summary) error {
	report := checkstyleReport{reportsPerPath: summary.ReportsPerPath()}
	xmlString, _ := xml.MarshalIndent(report, "", "  ")
	_, err := fmt.Fprint(cs.output, string(xml.Header)+string(xmlString)+"\n")
	return err
}
