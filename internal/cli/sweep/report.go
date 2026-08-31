package sweep

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jparrill/auriga-cli/internal/config"
)

type ReportWriter interface {
	Extension() string
	Write(report *SweepReport, path string) error
}

type JSONReportWriter struct{}

func (w *JSONReportWriter) Extension() string { return ".json" }

func (w *JSONReportWriter) Write(report *SweepReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

type CSVReportWriter struct{}

func (w *CSVReportWriter) Extension() string { return ".csv" }

func (w *CSVReportWriter) Write(report *SweepReport, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)

	overrideKeys := collectOverrideKeys(report.Results)
	sort.Strings(overrideKeys)

	header := []string{
		"index", "ttft_ms", "prompt_tok_s",
		"nothink_median", "nothink_min", "nothink_max",
		"think_median", "think_min", "think_max",
		"duration_ms", "error",
	}
	header = append(header, overrideKeys...)
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, r := range report.Results {
		row := []string{
			fmt.Sprintf("%d", r.Index),
			fmt.Sprintf("%d", r.TTFT),
			fmt.Sprintf("%.2f", r.Prompt),
			fmt.Sprintf("%.2f", r.NoThink.Median),
			fmt.Sprintf("%.2f", r.NoThink.Min),
			fmt.Sprintf("%.2f", r.NoThink.Max),
			fmt.Sprintf("%.2f", r.Think.Median),
			fmt.Sprintf("%.2f", r.Think.Min),
			fmt.Sprintf("%.2f", r.Think.Max),
			fmt.Sprintf("%d", r.DurationMs),
			r.Error,
		}
		for _, k := range overrideKeys {
			if v, ok := r.Overrides[k]; ok {
				row = append(row, v)
			} else {
				row = append(row, "")
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

var reportWriters = map[string]ReportWriter{
	"json": &JSONReportWriter{},
	"csv":  &CSVReportWriter{},
}

func GetReportWriter(format string) (ReportWriter, error) {
	w, ok := reportWriters[format]
	if !ok {
		return nil, fmt.Errorf("unsupported format %q: valid formats are 'json', 'csv'", format)
	}
	return w, nil
}

func saveSweepReport(report *SweepReport, format string, startTime time.Time) (string, error) {
	writer, err := GetReportWriter(format)
	if err != nil {
		return "", err
	}

	dir := config.ExpandHome("~/.config/auriga/sweep-results")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	timestamp := startTime.Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s-%s%s", report.Profile, timestamp, writer.Extension())
	path := filepath.Join(dir, filename)

	if err := writer.Write(report, path); err != nil {
		return "", err
	}
	return path, nil
}
