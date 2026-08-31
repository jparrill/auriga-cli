package show

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"encoding/json"
	"io"

	"github.com/jparrill/auriga-cli/internal/llamaserver"
	"github.com/jparrill/auriga-cli/internal/perf"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newShowPerfCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "perf [profile-name]",
		Short: "Quick performance test for running llama-server instances",
		Long: `Send test prompts to each running llama-server instance and report:
  - TTFT: Time to first token (prompt processing latency)
  - Prompt: Prompt processing speed (tok/s)
  - Gen: Token generation speed (tok/s), median of 5 runs with min-max range

A warmup request runs before measuring to avoid cold-cache penalties.
If a profile name is given, only test that profile's port.

Examples:
  auriga show perf              # Test all running instances
  auriga show perf qwen3.8-27b  # Test specific profile`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runPerfForProfile(args[0])
			}
			return runPerfAll()
		},
	}
}

type perfResult struct {
	Port      int
	Profile   string
	ModelType string
	SpecType  string
	Model     string
	Binary    string
	TTFT      time.Duration
	NoThink   perf.BenchResult
	Think     perf.BenchResult
	Error     string
}

func runPerfAll() error {
	ports := collectActivePorts()
	if len(ports) == 0 {
		ui.Warn("No running llama-server instances found")
		return nil
	}

	var results []perfResult
	for _, p := range ports {
		r := benchPort(p.port, p.profile)
		results = append(results, r)
	}

	printPerfResults(results)
	return nil
}

func runPerfForProfile(name string) error {
	profileKey := fmt.Sprintf("profiles.%s", name)
	model := viper.GetString(profileKey + ".model")
	if model == "" {
		return fmt.Errorf("profile %q not found", name)
	}

	port := resolveProfilePort(name)
	r := benchPort(port, name)
	printPerfResults([]perfResult{r})
	return nil
}

type portInfo struct {
	port    int
	profile string
}

func collectActivePorts() []portInfo {
	var active []portInfo
	seen := map[int]bool{}

	for _, port := range []int{llamaserver.DensePort(), llamaserver.MoePort()} {
		if seen[port] {
			continue
		}
		seen[port] = true
		if isPortHealthy(port) {
			profile := resolveProfileForPort(port)
			active = append(active, portInfo{port: port, profile: profile})
		}
	}

	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		port := resolveProfilePort(name)
		if seen[port] {
			continue
		}
		seen[port] = true
		if isPortHealthy(port) {
			active = append(active, portInfo{port: port, profile: name})
		}
	}

	return active
}

func resolveProfilePort(name string) int {
	profileKey := fmt.Sprintf("profiles.%s", name)
	if p := viper.GetInt(profileKey + ".port"); p > 0 {
		return p
	}
	t := viper.GetString(profileKey + ".type")
	if t == "" {
		model := viper.GetString(profileKey + ".model")
		t = detectModelTypeShow(model)
	}
	if t == "moe" {
		return llamaserver.MoePort()
	}
	return llamaserver.DensePort()
}

func resolveProfileForPort(port int) string {
	runningModel := filepath.Base(getRunningModel(port))
	profiles := viper.GetStringMap("profiles")
	for name := range profiles {
		if resolveProfilePort(name) == port {
			if runningModel == "" {
				return name
			}
			if viper.GetString(fmt.Sprintf("profiles.%s.model", name)) == runningModel {
				return name
			}
		}
	}
	return fmt.Sprintf("port-%d", port)
}

func getRunningModel(port int) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/v1/models", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) == nil && len(result.Data) > 0 {
		return result.Data[0].ID
	}
	return ""
}

func isPortHealthy(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func benchPort(port int, profile string) perfResult {
	profileKey := fmt.Sprintf("profiles.%s", profile)
	pType := viper.GetString(profileKey + ".type")
	if pType == "" {
		pType = detectModelTypeShow(viper.GetString(profileKey + ".model"))
	}
	specType := resolveSpecType(profile)
	bin := filepath.Base(llamaserver.BinForProfile(profile))
	result := perfResult{Port: port, Profile: profile, ModelType: pType, SpecType: specType, Binary: bin}

	ui.Info(fmt.Sprintf("Testing %s (port %d)...", profile, port))

	ui.Info("  warmup...")
	perf.RunSingleBench(port, false)

	ttft, model := perf.MeasureTTFT(port)
	result.TTFT = ttft
	result.Model = filepath.Base(model)

	if result.ModelType == "dense" && model != "" {
		result.ModelType = detectModelTypeShow(filepath.Base(model))
	}

	ui.Info(fmt.Sprintf("  no-think (%d runs)...", perf.Iterations))
	result.NoThink = perf.RunBench(port, false)

	ui.Info(fmt.Sprintf("  think (%d runs)...", perf.Iterations))
	result.Think = perf.RunBench(port, true)

	return result
}

func printPerfResults(results []perfResult) {
	fmt.Println()
	tbl := ui.NewTable("Performance",
		"PROFILE", "TYPE", "PORT", "BINARY", "SPEC", "MODEL",
		"TTFT", "PROMPT",
		"NO-THINK GEN", "THINK GEN",
	)

	for _, r := range results {
		spec := r.SpecType
		if spec != "-" {
			spec = ui.SuccessStyle.Render(spec)
		}
		if r.Error != "" {
			tbl.AddRow(r.Profile, "-", fmt.Sprintf("%d", r.Port), r.Binary, spec, "-", "-", "-", "-", "-")
			continue
		}

		ttft := fmt.Sprintf("%dms", r.TTFT.Milliseconds())
		prompt := fmtTokS(r.NoThink.PromptTokPerSec, r.NoThink.Error)
		noThinkGen := fmtTokSRange(r.NoThink)
		thinkGen := fmtTokSRange(r.Think)

		tbl.AddRow(
			r.Profile,
			r.ModelType,
			fmt.Sprintf("%d", r.Port),
			r.Binary,
			spec,
			r.Model,
			ttft,
			prompt,
			noThinkGen,
			thinkGen,
		)
	}
	tbl.Print()
}

func resolveSpecType(profile string) string {
	profileKey := fmt.Sprintf("profiles.%s", profile)
	if viper.GetString(profileKey+".dflash") != "" {
		return "dflash"
	}
	flags := viper.GetStringSlice(profileKey + ".flags")
	for _, f := range flags {
		if f == "draft-mtp" {
			return "mtp"
		}
	}
	if viper.GetString(profileKey+".mtp_drafter") != "" {
		return "mtp"
	}
	return "-"
}

func fmtTokS(tokPerSec float64, err string) string {
	if err != "" {
		return "error"
	}
	return fmt.Sprintf("%.1f tok/s", tokPerSec)
}

func fmtTokSRange(b perf.BenchResult) string {
	if b.Error != "" {
		return "error"
	}
	if b.GenMin == b.GenMax {
		return fmt.Sprintf("%.1f tok/s", b.GenerationTokPerSec)
	}
	return fmt.Sprintf("%.1f (%.1f-%.1f)", b.GenerationTokPerSec, b.GenMin, b.GenMax)
}
