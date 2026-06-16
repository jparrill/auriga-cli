package ui

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
)

var Logger *log.Logger

func InitLogger(verbose bool) {
	Logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
		Prefix:          "auriga",
	})
	if verbose {
		Logger.SetLevel(log.DebugLevel)
	} else {
		Logger.SetLevel(log.InfoLevel)
	}
}
