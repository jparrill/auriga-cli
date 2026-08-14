package exec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
)

type DownloadOpts struct {
	Resume bool
}

func DownloadFile(ctx context.Context, url, dest, label string, opts DownloadOpts) error {
	if config.DryRun {
		fmt.Println(ui.MutedStyle.Render("[dry-run]"), "download", label)
		return nil
	}

	ui.Logger.Debug("download", "url", url, "dest", dest)

	var existingSize int64
	if opts.Resume {
		if info, err := os.Stat(dest); err == nil {
			existingSize = info.Size()
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	var totalSize int64
	switch resp.StatusCode {
	case http.StatusPartialContent:
		totalSize = existingSize + resp.ContentLength
	case http.StatusOK:
		totalSize = resp.ContentLength
		existingSize = 0
	case http.StatusRequestedRangeNotSatisfiable:
		return nil
	default:
		return fmt.Errorf("download HTTP %d for %s", resp.StatusCode, label)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if existingSize > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dest, flags, 0644)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", dest, err)
	}
	defer f.Close()

	isTTY := isTTYOutput()
	downloaded := existingSize
	startTime := time.Now()
	lastRender := time.Time{}
	buf := make([]byte, 64*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				clearLine(isTTY)
				return fmt.Errorf("write failed: %w", err)
			}
			downloaded += int64(n)

			if isTTY && time.Since(lastRender) >= 150*time.Millisecond {
				elapsed := time.Since(startTime).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(downloaded-existingSize) / elapsed
				}
				renderDownloadProgress(label, downloaded, totalSize, speed)
				lastRender = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			clearLine(isTTY)
			return fmt.Errorf("download interrupted: %w", readErr)
		}
	}

	clearLine(isTTY)
	return nil
}

const progressBarWidth = 20

func renderDownloadProgress(label string, downloaded, total int64, speed float64) {
	width := getTermWidth()

	sizeStr := FormatSize(downloaded)
	speedStr := FormatSpeed(speed)

	if total > 0 {
		pct := float64(downloaded) / float64(total) * 100
		filled := int(pct / 100 * progressBarWidth)
		if filled > progressBarWidth {
			filled = progressBarWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)
		totalStr := FormatSize(total)

		// visible chars: "  ↓ " + label + "  " + bar(20) + "  XXX%" + "  size/total" + "  speed"
		fixedWidth := 4 + 2 + progressBarWidth + 6 + 2 + len(sizeStr) + 1 + len(totalStr) + 2 + len(speedStr)
		truncLabel := truncateStr(label, width-fixedWidth)

		fmt.Printf("\r\033[K  %s %s  %s  %3.0f%%  %s/%s  %s",
			ui.InfoStyle.Render("↓"),
			truncLabel,
			ui.AccentStyle.Render(bar),
			pct,
			sizeStr,
			totalStr,
			ui.MutedStyle.Render(speedStr),
		)
	} else {
		fixedWidth := 4 + 2 + len(sizeStr) + 2 + len(speedStr)
		truncLabel := truncateStr(label, width-fixedWidth)

		fmt.Printf("\r\033[K  %s %s  %s  %s",
			ui.InfoStyle.Render("↓"),
			truncLabel,
			sizeStr,
			ui.MutedStyle.Render(speedStr),
		)
	}
}

func getTermWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func truncateStr(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func clearLine(isTTY bool) {
	if isTTY {
		fmt.Print("\r\033[K")
	}
}

func isTTYOutput() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func FormatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func FormatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1<<30:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/(1<<30))
	case bytesPerSec >= 1<<20:
		return fmt.Sprintf("%.0f MB/s", bytesPerSec/(1<<20))
	case bytesPerSec >= 1<<10:
		return fmt.Sprintf("%.0f KB/s", bytesPerSec/(1<<10))
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}
