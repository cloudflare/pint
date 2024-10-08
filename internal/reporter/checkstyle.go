package reporter

import (
	"encoding/xml"
	"fmt"
	"io"
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

func (d checkstyleReport) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	err := e.EncodeToken(xml.StartElement{
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
		errEnc := e.EncodeToken(xml.StartElement{
			Name: xml.Name{Local: "file"},
			Attr: []xml.Attr{
				{
					Name:  xml.Name{Local: "name"},
					Value: dir,
				},
			},
		})
		if errEnc != nil {
			return errEnc
		}
		for _, report := range reports {
			errEnc2 := e.Encode(report)
			if errEnc2 != nil {
				return errEnc2
			}
		}
		errEnc = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "file"}})
		if errEnc != nil {
			return errEnc
		}
	}
	err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "checkstyle"}})
	return err
}

func (r Report) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
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
				Value: fmt.Sprintf("Text:%s\n Details:%s", r.Problem.Text, r.Problem.Details),
			},
			{
				Name:  xml.Name{Local: "source"},
				Value: r.Problem.Reporter,
			},
		},
	}
	err := e.EncodeToken(startel)
	if err != nil {
		return err
	}
	err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "error"}})

	return err
}

func (cs CheckStyleReporter) Submit(summary Summary) error {
	checkstyleReport := createCheckstyleReport(summary)
	xmlString, err := xml.MarshalIndent(checkstyleReport, "", "  ")
	if err != nil {
		fmt.Printf("%v", err)
	}
	fmt.Fprint(cs.output, string(xml.Header)+string(xmlString)+"\n")
	return nil
}
