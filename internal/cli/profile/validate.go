package profile

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ggufMeta struct {
	Architecture string
	CtxTrain     int
	Layers       int
	KVHeads      int
	HeadCount    int
	EmbdSize     int
	HasMTP       bool
}

type profileValidation struct {
	Name      string
	Type      string
	CtxSize   int
	CtxMax    int
	SpecType  string
	ModelSize int64
	KVEst     int64
	TotalEst  int64
	Warnings  []string
	Errors    []string
}

func newProfileValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate profile configs against model capabilities and memory",
		Long: `Check each profile's ctx_size against model maximum, estimate memory usage,
and verify dual-instance (dense + MoE) fits within available GPU memory (GTT).

Detects: ctx_size exceeding model max, mtp_drafter/dflash without --model-draft,
dual-instance combinations that exceed GTT.

GTT source: llama_server.gtt_bytes config > /sys/class/drm/card*/device/mem_info_gtt_total.

Examples:
  auriga profile validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileValidate()
		},
	}
}

type configCheck struct {
	Key      string
	Status   string // "ok", "missing", "recommended"
	Detail   string
}

var requiredGlobalKeys = []struct {
	key     string
	reason  string
}{
	{"llama_server.bin", "llama-server binary path"},
	{"llama_server.gguf_dir", "GGUF model directory"},
}

var recommendedGlobalKeys = []struct {
	key     string
	reason  string
}{
	{"llama_server.mmproj_dir", "multimodal projector directory"},
	{"llama_server.dense_port", "dense model port"},
	{"llama_server.moe_port", "MoE model port"},
	{"llama_server.ctx_size", "global default context size"},
}

var requiredProfileKeys = []string{"model", "repo"}
var recommendedProfileKeys = []string{"type", "ctx_size"}

func runProfileValidate() error {
	ggufDir := config.ExpandHome(viper.GetString("llama_server.gguf_dir"))
	profiles := viper.GetStringMap("profiles")
	hasErrors := false

	// --- Config schema ---
	schemaChecks := validateConfigSchema(profiles)
	schemaTbl := ui.NewTable("Config Schema", "STATUS", "KEY", "DETAIL")
	for _, c := range schemaChecks {
		status := ui.SuccessStyle.Render("✓")
		if c.Status == "missing" {
			status = ui.ErrorStyle.Render("✗")
			hasErrors = true
		} else if c.Status == "recommended" {
			status = ui.WarningStyle.Render("⚠")
		}
		schemaTbl.AddRow(status, c.Key, c.Detail)
	}
	schemaTbl.Print()

	if len(profiles) == 0 {
		ui.Warn("No profiles configured")
		printLegend()
		return nil
	}

	// --- Profiles ---
	gtt := readGTTTotal()

	var names []string
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	var validations []profileValidation
	var denseProfiles, moeProfiles []profileValidation

	for _, name := range names {
		v := validateProfile(name, ggufDir)
		validations = append(validations, v)
		if v.Type == "dense" {
			denseProfiles = append(denseProfiles, v)
		} else {
			moeProfiles = append(moeProfiles, v)
		}
		if len(v.Errors) > 0 {
			hasErrors = true
		}
	}

	profileTbl := ui.NewTable("Profiles", "STATUS", "PROFILE", "TYPE", "CTX", "SPEC", "MEMORY")
	for _, v := range validations {
		status := ui.SuccessStyle.Render("✓")
		if len(v.Errors) > 0 {
			status = ui.ErrorStyle.Render("✗")
		} else if len(v.Warnings) > 0 {
			status = ui.WarningStyle.Render("⚠")
		}

		ctxCol := "-"
		if v.CtxMax > 0 {
			ctxCol = fmt.Sprintf("%dk/%dk", v.CtxSize/1024, v.CtxMax/1024)
		}

		memCol := "-"
		if v.TotalEst > 0 {
			memCol = fmt.Sprintf("%.1f GB", float64(v.TotalEst)/1e9)
		}

		specCol := "-"
		if v.SpecType != "" {
			specCol = ui.SuccessStyle.Render(v.SpecType)
		}

		profileTbl.AddRow(status, v.Name, v.Type, ctxCol, specCol, memCol)
	}
	profileTbl.Print()

	// --- Dual-instance ---
	if gtt > 0 && len(denseProfiles) > 0 && len(moeProfiles) > 0 {
		gttLabel := fmt.Sprintf("Dual-Instance Fit (GTT: %.1f GB)", float64(gtt)/1e9)
		dualTbl := ui.NewTable(gttLabel, "STATUS", "DENSE", "MOE", "COMBINED", "USAGE")
		for _, d := range denseProfiles {
			for _, m := range moeProfiles {
				if d.TotalEst == 0 || m.TotalEst == 0 {
					continue
				}
				combined := d.TotalEst + m.TotalEst
				pct := float64(combined) / float64(gtt) * 100
				status := ui.SuccessStyle.Render("✓")
				if combined > gtt {
					status = ui.ErrorStyle.Render("✗")
					hasErrors = true
				} else if pct > 85 {
					status = ui.WarningStyle.Render("⚠")
				}
				dualTbl.AddRow(
					status,
					d.Name,
					m.Name,
					fmt.Sprintf("%.1f GB", float64(combined)/1e9),
					fmt.Sprintf("%.0f%%", pct),
				)
			}
		}
		dualTbl.Print()
	} else if gtt == 0 {
		ui.Warn("GTT not available — skipping dual-instance checks")
		fmt.Println()
	}

	// --- Issues summary ---
	var hasIssues bool
	for _, v := range validations {
		if len(v.Errors) > 0 || len(v.Warnings) > 0 {
			hasIssues = true
			break
		}
	}
	if hasIssues {
		fmt.Printf("  %s\n", ui.BoldStyle.Render("Issues"))
		for _, v := range validations {
			for _, e := range v.Errors {
				fmt.Printf("  %s %s: %s\n", ui.ErrorStyle.Render("✗"), v.Name, e)
			}
			for _, w := range v.Warnings {
				fmt.Printf("  %s %s: %s\n", ui.WarningStyle.Render("⚠"), v.Name, w)
			}
		}
		fmt.Println()
	}

	printLegend()

	if hasErrors {
		return fmt.Errorf("validation found errors")
	}
	return nil
}

func validateConfigSchema(profiles map[string]any) []configCheck {
	var checks []configCheck

	for _, req := range requiredGlobalKeys {
		if viper.GetString(req.key) != "" {
			checks = append(checks, configCheck{Key: req.key, Status: "ok", Detail: req.reason})
		} else {
			checks = append(checks, configCheck{Key: req.key, Status: "missing", Detail: req.reason + " (required)"})
		}
	}

	for _, rec := range recommendedGlobalKeys {
		if viper.Get(rec.key) != nil {
			checks = append(checks, configCheck{Key: rec.key, Status: "ok", Detail: rec.reason})
		} else {
			checks = append(checks, configCheck{Key: rec.key, Status: "recommended", Detail: rec.reason + " (recommended)"})
		}
	}

	if viper.GetInt64("llama_server.gtt_bytes") > 0 {
		checks = append(checks, configCheck{Key: "llama_server.gtt_bytes", Status: "ok", Detail: "GPU memory override"})
	} else if runtime.GOOS != "linux" {
		checks = append(checks, configCheck{Key: "llama_server.gtt_bytes", Status: "recommended", Detail: "no sysfs on " + runtime.GOOS + ", set manually for memory validation (recommended)"})
	}

	for name := range profiles {
		prefix := fmt.Sprintf("profiles.%s", name)
		for _, key := range requiredProfileKeys {
			full := prefix + "." + key
			if viper.GetString(full) != "" {
				continue
			}
			checks = append(checks, configCheck{Key: full, Status: "missing", Detail: "required"})
		}
		for _, key := range recommendedProfileKeys {
			full := prefix + "." + key
			if viper.Get(full) != nil {
				continue
			}
			checks = append(checks, configCheck{Key: full, Status: "recommended", Detail: "recommended for explicit control"})
		}
	}

	return checks
}

func printLegend() {
	fmt.Printf("  %s = ok   %s = warning   %s = error\n\n",
		ui.SuccessStyle.Render("✓"),
		ui.WarningStyle.Render("⚠"),
		ui.ErrorStyle.Render("✗"))
}

func validateProfile(name, ggufDir string) profileValidation {
	profileKey := fmt.Sprintf("profiles.%s", name)
	modelFile := viper.GetString(profileKey + ".model")

	v := profileValidation{
		Name:    name,
		Type:    profileType(name),
		CtxSize: profileCtxSize(name),
	}

	if modelFile == "" {
		v.Errors = append(v.Errors, "no model configured")
		return v
	}

	modelPath := filepath.Join(ggufDir, modelFile)
	fi, err := os.Stat(modelPath)
	if err != nil {
		v.Warnings = append(v.Warnings, fmt.Sprintf("model not on disk: %s", modelFile))
		return v
	}
	v.ModelSize = fi.Size()

	meta, err := readGGUFMeta(modelPath)
	if err != nil {
		v.Warnings = append(v.Warnings, fmt.Sprintf("GGUF metadata unreadable: %v", err))
		v.TotalEst = v.ModelSize
		return v
	}

	v.CtxMax = meta.CtxTrain

	if meta.CtxTrain > 0 && v.CtxSize > meta.CtxTrain {
		v.Errors = append(v.Errors, fmt.Sprintf("ctx_size %d > model max %d", v.CtxSize, meta.CtxTrain))
	}

	if meta.KVHeads > 0 && meta.Layers > 0 && meta.HeadCount > 0 && meta.EmbdSize > 0 {
		headDim := meta.EmbdSize / meta.HeadCount
		v.KVEst = estimateKVCache(meta.KVHeads, headDim, meta.Layers, v.CtxSize)
	}
	v.TotalEst = v.ModelSize + v.KVEst

	mmprojFile := viper.GetString(profileKey + ".mmproj")
	mmprojDir := config.ExpandHome(viper.GetString("llama_server.mmproj_dir"))
	if mmprojFile != "" {
		if _, err := os.Stat(filepath.Join(mmprojDir, mmprojFile)); err != nil {
			v.Warnings = append(v.Warnings, fmt.Sprintf("mmproj not on disk: %s", mmprojFile))
		}
	}

	flags := viper.GetStringSlice(profileKey + ".flags")
	mtpDrafter := viper.GetString(profileKey + ".mtp_drafter")
	dflash := viper.GetString(profileKey + ".dflash")

	for _, drafter := range []struct{ field, file string }{
		{"mtp_drafter", mtpDrafter},
		{"dflash", dflash},
	} {
		if drafter.file == "" {
			continue
		}
		drafterPath := filepath.Join(ggufDir, drafter.file)
		if fi, err := os.Stat(drafterPath); err != nil {
			v.Warnings = append(v.Warnings, fmt.Sprintf("%s not on disk: %s", drafter.field, drafter.file))
		} else {
			v.TotalEst += fi.Size()
		}
		if !containsFlag(flags, "--model-draft") {
			v.Warnings = append(v.Warnings, fmt.Sprintf("%s set but --model-draft not in flags", drafter.field))
		}
	}

	if dflash != "" {
		v.SpecType = "dflash"
		dflashRepo := viper.GetString(profileKey + ".dflash_repo")
		repo := viper.GetString(profileKey + ".repo")
		if dflashRepo == "" && repo == "" {
			v.Warnings = append(v.Warnings, "dflash set but no dflash_repo or repo for sync")
		}
	}
	if meta.HasMTP || mtpDrafter != "" || containsFlag(flags, "draft-mtp") {
		v.SpecType = "mtp"
	}
	if mtpDrafter != "" {
		mtpDrafterRepo := viper.GetString(profileKey + ".mtp_drafter_repo")
		repo := viper.GetString(profileKey + ".repo")
		if mtpDrafterRepo == "" && repo == "" {
			v.Warnings = append(v.Warnings, "mtp_drafter set but no mtp_drafter_repo or repo for sync")
		}
	}

	if containsFlag(flags, "draft-mtp") && !meta.HasMTP && mtpDrafter == "" {
		v.Warnings = append(v.Warnings, "--spec-type draft-mtp but model has no MTP heads and no external drafter")
	}

	return v
}

func estimateKVCache(kvHeads, headDim, layers, ctxSize int) int64 {
	perToken := int64(2) * int64(kvHeads) * int64(headDim) * int64(layers)
	return perToken * int64(ctxSize)
}

func readGTTTotal() int64 {
	if gtt := viper.GetInt64("llama_server.gtt_bytes"); gtt > 0 {
		return gtt
	}
	matches, err := filepath.Glob("/sys/class/drm/card*/device/mem_info_gtt_total")
	if err != nil {
		return 0
	}
	var maxGTT int64
	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			continue
		}
		if val > maxGTT {
			maxGTT = val
		}
	}
	return maxGTT
}

func readGGUFMeta(path string) (ggufMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return ggufMeta{}, err
	}
	defer f.Close()

	var magic [4]byte
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil {
		return ggufMeta{}, err
	}
	if string(magic[:]) != "GGUF" {
		return ggufMeta{}, fmt.Errorf("not a GGUF file")
	}

	var version uint32
	if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		return ggufMeta{}, err
	}

	var nTensors, nKV uint64
	binary.Read(f, binary.LittleEndian, &nTensors)
	binary.Read(f, binary.LittleEndian, &nKV)

	meta := ggufMeta{}

	type target struct {
		suffix string
		dest   *int
	}
	targets := []target{
		{"context_length", &meta.CtxTrain},
		{"block_count", &meta.Layers},
		{"head_count_kv", &meta.KVHeads},
		{"head_count", &meta.HeadCount},
		{"embedding_length", &meta.EmbdSize},
	}

	for i := uint64(0); i < nKV; i++ {
		key, err := readGGUFString(f)
		if err != nil {
			break
		}

		var vtype uint32
		if err := binary.Read(f, binary.LittleEndian, &vtype); err != nil {
			break
		}

		if key == "general.architecture" && vtype == 8 {
			val, err := readGGUFString(f)
			if err == nil {
				meta.Architecture = val
			}
			continue
		}

		handled := false
		for _, t := range targets {
			if strings.HasSuffix(key, "."+t.suffix) {
				if isGGUFIntType(vtype) {
					val, err := readGGUFInt(f, vtype)
					if err == nil {
						*t.dest = int(val)
					}
				} else {
					skipGGUFValue(f, vtype)
				}
				handled = true
				break
			}
		}

		if !handled {
			if err := skipGGUFValue(f, vtype); err != nil {
				break
			}
		}
	}

	for i := uint64(0); i < nTensors; i++ {
		name, err := readGGUFString(f)
		if err != nil {
			break
		}
		if strings.Contains(name, "nextn") {
			meta.HasMTP = true
		}
		var nDims uint32
		if err := binary.Read(f, binary.LittleEndian, &nDims); err != nil {
			break
		}
		for d := uint32(0); d < nDims; d++ {
			var dim uint64
			if err := binary.Read(f, binary.LittleEndian, &dim); err != nil {
				break
			}
		}
		var tensorType uint32
		binary.Read(f, binary.LittleEndian, &tensorType)
		var offset uint64
		binary.Read(f, binary.LittleEndian, &offset)
	}

	return meta, nil
}

func isGGUFIntType(vtype uint32) bool {
	return vtype <= 5 || vtype == 10 || vtype == 11
}

func readGGUFString(r io.Reader) (string, error) {
	var length uint64
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	if length > 1<<20 {
		return "", fmt.Errorf("string too long: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readGGUFInt(r io.Reader, vtype uint32) (int64, error) {
	switch vtype {
	case 0:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 1:
		var v int8
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 2:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 3:
		var v int16
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 4:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 5:
		var v int32
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 10:
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case 11:
		var v int64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	default:
		return 0, fmt.Errorf("not an integer type: %d", vtype)
	}
}

func skipGGUFValue(r io.Reader, vtype uint32) error {
	switch vtype {
	case 0, 1, 7:
		_, err := io.ReadFull(r, make([]byte, 1))
		return err
	case 2, 3:
		_, err := io.ReadFull(r, make([]byte, 2))
		return err
	case 4, 5, 6:
		_, err := io.ReadFull(r, make([]byte, 4))
		return err
	case 10, 11, 12:
		_, err := io.ReadFull(r, make([]byte, 8))
		return err
	case 8:
		_, err := readGGUFString(r)
		return err
	case 9:
		var atype uint32
		if err := binary.Read(r, binary.LittleEndian, &atype); err != nil {
			return err
		}
		var alen uint64
		if err := binary.Read(r, binary.LittleEndian, &alen); err != nil {
			return err
		}
		for j := uint64(0); j < alen; j++ {
			if err := skipGGUFValue(r, atype); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown GGUF value type: %d", vtype)
	}
}
