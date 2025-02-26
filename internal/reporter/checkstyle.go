package reporter

import (
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"strconv"
)

func NewCheckStyleReporter(output io.Writer) CheckStyleReporter {
	return CheckStyleReporter{
		output: output,
	}
}

type CheckStyleReporter struct {
	output io.Writer
}

type checkstyleReport map[string][]Report

func createCheckstyleReport(summary Summary) checkstyleReport {
	x := make(checkstyleReport)
	for _, report := range summary.reports {
		x[report.Path.Name] = append(x[report.Path.Name], report)
	}
	return x
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
	for dir, reports := range d {
		if err = e.EncodeToken(
			xml.StartElement{
				Name: xml.Name{Local: "file"},
				Attr: []xml.Attr{
					{
						Name:  xml.Name{Local: "name"},
						Value: dir,
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
	startel := xml.StartElement{
		Name: xml.Name{Local: "error"},
		Attr: []xml.Attr{
			{
				Name:  xml.Name{Local: "line"},
				Value: strconv.Itoa(r.Problem.Lines.First),
			},
			{
				Name:  xml.Name{Local: "severity"},
				Value: r.Problem.Severity.String(),
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

func (cs CheckStyleReporter) Submit(summary Summary) error {
	checkstyleReport := createCheckstyleReport(summary)
	xmlString, err := xml.MarshalIndent(checkstyleReport, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal checkstyle report", slog.Any("err", err))
		return err
	}
	_, err = fmt.Fprint(cs.output, string(xml.Header)+string(xmlString)+"\n")
	return err
}
