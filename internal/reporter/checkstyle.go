package reporter

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
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

func (d checkstyleReport) MarshalXML(e *xml.Encoder, _ xml.StartElement) (err error) {
	err = e.EncodeToken(xml.StartElement{
		Name: xml.Name{Local: "checkstyle"},
		Attr: []xml.Attr{
			{
				Name:  xml.Name{Local: "version"},
				Value: "4.3",
			},
		},
	})
	if err != nil {
		return err
	}
	for _, reports := range d.reportsPerPath {
		if len(reports) == 0 {
			continue
		}
		if err = e.EncodeToken(
			xml.StartElement{
				Name: xml.Name{Local: "file"},
				Attr: []xml.Attr{
					{
						Name:  xml.Name{Local: "name"},
						Value: reports[0].Path.Name,
					},
				},
			}); err != nil {
			return err
		}
		for _, report := range reports {
			if err = e.Encode(report); err != nil {
				return err
			}
		}
		if err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "file"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "checkstyle"}})
}

func (r Report) MarshalXML(e *xml.Encoder, _ xml.StartElement) (err error) {
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

	startel := xml.StartElement{
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
	}
	if err = e.EncodeToken(startel); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "error"}})
}

func (cs CheckStyleReporter) Submit(ctx context.Context, summary Summary) error {
	report := checkstyleReport{reportsPerPath: summary.ReportsPerPath()}
	xmlString, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "Failed to marshal checkstyle report", slog.Any("err", err))
		return err
	}
	_, err = fmt.Fprint(cs.output, string(xml.Header)+string(xmlString)+"\n")
	return err
}
