package profile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestEstimateKVCache(t *testing.T) {
	tests := []struct {
		name    string
		kvHeads int
		headDim int
		layers  int
		ctxSize int
		want    int64
	}{
		{
			name:    "When Qwen3.6-27B params, it should match known estimate",
			kvHeads: 4,
			headDim: 128,
			layers:  64,
			ctxSize: 65536,
			want:    2 * 4 * 128 * 64 * 65536,
		},
		{
			name:    "When zero ctx, it should return zero",
			kvHeads: 8,
			headDim: 128,
			layers:  32,
			ctxSize: 0,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateKVCache(tt.kvHeads, tt.headDim, tt.layers, tt.ctxSize)
			if got != tt.want {
				t.Errorf("estimateKVCache(%d, %d, %d, %d) = %d, want %d",
					tt.kvHeads, tt.headDim, tt.layers, tt.ctxSize, got, tt.want)
			}
		})
	}
}

func TestReadGTTTotal_ConfigFallback(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.gtt_bytes", 112742891520)
	got := readGTTTotal()
	if got != 112742891520 {
		t.Errorf("When gtt_bytes configured, should return config value, got %d", got)
	}
}

func TestReadGTTTotal_NoConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	got := readGTTTotal()
	// On macOS (no sysfs), should return 0
	if got < 0 {
		t.Errorf("When no sysfs and no config, should return >= 0, got %d", got)
	}
}

func TestReadGTTTotalFromSysfs(t *testing.T) {
	tests := []struct {
		name     string
		glob     func(string) ([]string, error)
		readFile func(string) ([]byte, error)
		want     int64
	}{
		{
			name: "When sysfs has multiple cards, it should use largest GTT total",
			glob: func(string) ([]string, error) {
				return []string{"card0", "card1"}, nil
			},
			readFile: func(path string) ([]byte, error) {
				if path == "card0" {
					return []byte("107374182400\n"), nil
				}
				return []byte("113279762432\n"), nil
			},
			want: 113279762432,
		},
		{
			name: "When sysfs is unavailable, it should return zero",
			glob: func(string) ([]string, error) {
				return nil, errors.New("sysfs unavailable")
			},
			readFile: os.ReadFile,
			want:     0,
		},
		{
			name: "When sysfs values are unreadable, it should ignore them",
			glob: func(string) ([]string, error) {
				return []string{"missing", "invalid"}, nil
			},
			readFile: func(path string) ([]byte, error) {
				if path == "missing" {
					return nil, errors.New("read failed")
				}
				return []byte("not-a-number"), nil
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readGTTTotalFromSysfs(tt.glob, tt.readFile); got != tt.want {
				t.Errorf("readGTTTotalFromSysfs() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAssessGTTFit(t *testing.T) {
	const gib = int64(1 << 30)
	tests := []struct {
		name       string
		total      int64
		first      int64
		second     int64
		wantStatus gttFitStatus
		wantMargin int64
		wantPct    float64
	}{
		{
			name:       "When GTT is unavailable, it should leave fit unclassified",
			total:      0,
			first:      40 * gib,
			second:     40 * gib,
			wantStatus: gttFitUnknown,
			wantMargin: 0,
			wantPct:    0,
		},
		{
			name:       "When combined estimate is below warning threshold, it should fit",
			total:      100 * gib,
			first:      40 * gib,
			second:     40 * gib,
			wantStatus: gttFitOK,
			wantMargin: 20 * gib,
			wantPct:    80,
		},
		{
			name:       "When combined estimate is exactly 85 percent, it should fit",
			total:      100 * gib,
			first:      40 * gib,
			second:     45 * gib,
			wantStatus: gttFitOK,
			wantMargin: 15 * gib,
			wantPct:    85,
		},
		{
			name:       "When combined estimate exceeds 85 percent, it should warn",
			total:      100 * gib,
			first:      40 * gib,
			second:     46 * gib,
			wantStatus: gttFitWarning,
			wantMargin: 14 * gib,
			wantPct:    86,
		},
		{
			name:       "When combined estimate equals capacity, it should warn with zero margin",
			total:      100 * gib,
			first:      50 * gib,
			second:     50 * gib,
			wantStatus: gttFitWarning,
			wantMargin: 0,
			wantPct:    100,
		},
		{
			name:       "When combined estimate exceeds capacity, it should error with negative margin",
			total:      100 * gib,
			first:      50 * gib,
			second:     51 * gib,
			wantStatus: gttFitError,
			wantMargin: -1 * gib,
			wantPct:    101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assessGTTFit(tt.total, tt.first, tt.second)
			if got.Status != tt.wantStatus || got.Margin != tt.wantMargin || got.UsagePct != tt.wantPct {
				t.Errorf("assessGTTFit() = %+v, want status=%s margin=%d pct=%.0f", got, tt.wantStatus, tt.wantMargin, tt.wantPct)
			}
			if got.Combined != tt.first+tt.second {
				t.Errorf("combined = %d, want %d", got.Combined, tt.first+tt.second)
			}
		})
	}
}

func TestRunProfileValidate_ShowsGiBUsageAndMargin(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	for _, profileName := range []string{"dense", "moe"} {
		modelPath := writeTestGGUF(t, map[string]any{
			"general.architecture":  "test",
			"test.context_length":   uint32(131072),
			"test.block_count":      uint32(32),
			"test.head_count_kv":    uint32(8),
			"test.head_count":       uint32(32),
			"test.embedding_length": uint32(4096),
		})
		modelFile := profileName + ".gguf"
		if err := os.Rename(modelPath, filepath.Join(ggufDir, modelFile)); err != nil {
			t.Fatal(err)
		}
		viper.Set("profiles."+profileName+".model", modelFile)
		viper.Set("profiles."+profileName+".repo", "org/repo")
		viper.Set("profiles."+profileName+".type", profileName)
		viper.Set("profiles."+profileName+".ctx_size", 65536)
	}

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.gguf_dir", ggufDir)
	viper.Set("llama_server.mmproj_dir", t.TempDir())
	viper.Set("llama_server.slot_1_port", 8090)
	viper.Set("llama_server.slot_2_port", 8091)
	viper.Set("llama_server.ctx_size", 131072)
	viper.Set("llama_server.gtt_bytes", int64(20<<30))

	output := captureValidateOutput(t, func() {
		if err := runProfileValidate(); err != nil {
			t.Errorf("runProfileValidate() error = %v", err)
		}
	})
	for _, want := range []string{"GTT: 20.0 GiB", "MARGIN", "12.0 GiB"} {
		if !strings.Contains(output, want) {
			t.Errorf("validation output missing %q:\n%s", want, output)
		}
	}
}

func captureValidateOutput(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = original })

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original

	var output bytes.Buffer
	if _, err := io.Copy(&output, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestReadGGUFMeta_ValidFile(t *testing.T) {
	path := writeTestGGUF(t, map[string]any{
		"general.architecture":    "qwen35",
		"qwen35.context_length":   uint32(131072),
		"qwen35.block_count":      uint32(64),
		"qwen35.head_count_kv":    uint32(4),
		"qwen35.head_count":       uint32(32),
		"qwen35.embedding_length": uint32(4096),
	})

	meta, err := readGGUFMeta(path)
	if err != nil {
		t.Fatalf("readGGUFMeta failed: %v", err)
	}

	if meta.Architecture != "qwen35" {
		t.Errorf("architecture = %q, want qwen35", meta.Architecture)
	}
	if meta.CtxTrain != 131072 {
		t.Errorf("ctx_train = %d, want 131072", meta.CtxTrain)
	}
	if meta.Layers != 64 {
		t.Errorf("layers = %d, want 64", meta.Layers)
	}
	if meta.KVHeads != 4 {
		t.Errorf("kv_heads = %d, want 4", meta.KVHeads)
	}
	if meta.HeadCount != 32 {
		t.Errorf("head_count = %d, want 32", meta.HeadCount)
	}
	if meta.EmbdSize != 4096 {
		t.Errorf("embd_size = %d, want 4096", meta.EmbdSize)
	}
}

func TestReadGGUFMeta_NotGGUF(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "notgguf.bin")
	os.WriteFile(tmp, []byte("not a gguf file"), 0644)

	_, err := readGGUFMeta(tmp)
	if err == nil {
		t.Error("When not GGUF, should return error")
	}
}

func TestReadGGUFMeta_MissingFile(t *testing.T) {
	_, err := readGGUFMeta("/nonexistent/file.gguf")
	if err == nil {
		t.Error("When file missing, should return error")
	}
}

func TestValidateProfile_CtxExceedsMax(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(32768),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.test-profile.model", modelFile)
	viper.Set("profiles.test-profile.ctx_size", 65536)

	v := validateProfile("test-profile", ggufDir)

	if len(v.Errors) == 0 {
		t.Error("When ctx_size > model max, should have errors")
	}
	found := false
	for _, e := range v.Errors {
		if e == "ctx_size 65536 > model max 32768" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ctx overflow error, got errors: %v", v.Errors)
	}
}

func TestValidateProfile_MTPDrafterMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.drafter-test.model", modelFile)
	viper.Set("profiles.drafter-test.mtp_drafter", "drafter.gguf")
	viper.Set("profiles.drafter-test.flags", []string{"--spec-type", "draft-mtp"})

	v := validateProfile("drafter-test", ggufDir)

	foundFile := false
	for _, w := range v.Warnings {
		if w == "mtp_drafter not on disk: drafter.gguf" {
			foundFile = true
		}
	}
	if !foundFile {
		t.Errorf("expected file-missing warning, got warnings: %v", v.Warnings)
	}
}

func TestValidateProfile_DrafterAutoInjectionDoesNotWarnAboutFlags(t *testing.T) {
	tests := []struct {
		name  string
		field string
	}{
		{name: "When MTP drafter is configured and present, it should not require manual model-draft flag", field: "mtp_drafter"},
		{name: "When DFlash drafter is configured and present, it should not require manual model-draft flag", field: "dflash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			defer viper.Reset()

			ggufDir := t.TempDir()
			modelPath := writeTestGGUF(t, map[string]any{
				"general.architecture":  "test",
				"test.context_length":   uint32(131072),
				"test.block_count":      uint32(32),
				"test.head_count_kv":    uint32(8),
				"test.head_count":       uint32(32),
				"test.embedding_length": uint32(4096),
			})
			modelFile := filepath.Base(modelPath)
			if err := os.Rename(modelPath, filepath.Join(ggufDir, modelFile)); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ggufDir, "drafter.gguf"), []byte("drafter"), 0644); err != nil {
				t.Fatal(err)
			}

			viper.Set("profiles.drafter-test.model", modelFile)
			viper.Set("profiles.drafter-test.repo", "org/repo")
			viper.Set("profiles.drafter-test."+tt.field, "drafter.gguf")

			validation := validateProfile("drafter-test", ggufDir)
			for _, warning := range validation.Warnings {
				if strings.Contains(warning, "--model-draft not in flags") {
					t.Errorf("validateProfile() returned false warning: %s", warning)
				}
			}
		})
	}
}

func TestValidateProfile_MmprojMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.vision-test.model", modelFile)
	viper.Set("profiles.vision-test.mmproj", "missing-mmproj.gguf")
	viper.Set("llama_server.mmproj_dir", t.TempDir())

	v := validateProfile("vision-test", ggufDir)

	found := false
	for _, w := range v.Warnings {
		if w == "mmproj not on disk: missing-mmproj.gguf" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mmproj warning, got warnings: %v", v.Warnings)
	}
}

func TestValidateProfile_DrafterAddsMemory(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	drafterPath := filepath.Join(ggufDir, "drafter.gguf")
	os.WriteFile(drafterPath, make([]byte, 1000000), 0644)

	viper.Set("profiles.drafter-mem.model", modelFile)
	viper.Set("profiles.drafter-mem.mtp_drafter", "drafter.gguf")
	viper.Set("profiles.drafter-mem.flags", []string{"--model-draft", "/path/drafter.gguf"})

	v := validateProfile("drafter-mem", ggufDir)

	modelStat, _ := os.Stat(filepath.Join(ggufDir, modelFile))
	expectedMin := modelStat.Size() + 1000000
	if v.TotalEst < expectedMin {
		t.Errorf("TotalEst should include drafter size, got %d, want >= %d", v.TotalEst, expectedMin)
	}
}

func TestValidateProfile_NoModel(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.empty.repo", "some/repo")

	v := validateProfile("empty", t.TempDir())

	if len(v.Errors) == 0 || v.Errors[0] != "no model configured" {
		t.Errorf("When no model, should error, got: %v", v.Errors)
	}
}

func TestIsGGUFIntType(t *testing.T) {
	intTypes := []uint32{0, 1, 2, 3, 4, 5, 10, 11}
	for _, vt := range intTypes {
		if !isGGUFIntType(vt) {
			t.Errorf("type %d should be int type", vt)
		}
	}
	nonIntTypes := []uint32{6, 7, 8, 9, 12}
	for _, vt := range nonIntTypes {
		if isGGUFIntType(vt) {
			t.Errorf("type %d should NOT be int type", vt)
		}
	}
}

func TestValidateConfigSchema_RequiredMissing(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.test.model", "model.gguf")

	checks := validateConfigSchema(viper.GetStringMap("profiles"))

	missingKeys := map[string]bool{}
	for _, c := range checks {
		if c.Status == "missing" {
			missingKeys[c.Key] = true
		}
	}
	if !missingKeys["llama_server.bin"] {
		t.Error("When bin not set, should report missing")
	}
	if !missingKeys["llama_server.gguf_dir"] {
		t.Error("When gguf_dir not set, should report missing")
	}
}

func TestValidateConfigSchema_AllSet(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("llama_server.bin", "/usr/bin/llama-server")
	viper.Set("llama_server.gguf_dir", "/models")
	viper.Set("llama_server.mmproj_dir", "/mmproj")
	viper.Set("llama_server.slot_1_port", 8090)
	viper.Set("llama_server.slot_2_port", 8091)
	viper.Set("llama_server.ctx_size", 131072)
	viper.Set("llama_server.gtt_bytes", 112742891520)
	viper.Set("profiles.p1.model", "m.gguf")
	viper.Set("profiles.p1.repo", "org/repo")
	viper.Set("profiles.p1.type", "dense")
	viper.Set("profiles.p1.ctx_size", 65536)

	checks := validateConfigSchema(viper.GetStringMap("profiles"))

	for _, c := range checks {
		if c.Status != "ok" {
			t.Errorf("When all set, %s should be ok, got %s", c.Key, c.Status)
		}
	}
}

func TestValidateConfigSchema_ProfileRecommended(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("profiles.noextra.model", "m.gguf")
	viper.Set("profiles.noextra.repo", "org/repo")

	checks := validateConfigSchema(viper.GetStringMap("profiles"))

	recommended := map[string]bool{}
	for _, c := range checks {
		if c.Status == "recommended" {
			recommended[c.Key] = true
		}
	}
	if !recommended["profiles.noextra.type"] {
		t.Error("When type not set, should recommend it")
	}
	if !recommended["profiles.noextra.ctx_size"] {
		t.Error("When ctx_size not set, should recommend it")
	}
}

func TestReadGGUFMeta_MTPDetection(t *testing.T) {
	tests := []struct {
		name    string
		tensors []string
		wantMTP bool
	}{
		{
			name:    "When model has nextn tensors, it should detect MTP",
			tensors: []string{"blk.0.attn_k.weight", "nextn0.blk.0.ffn_gate.weight", "nextn0.blk.0.ffn_up.weight"},
			wantMTP: true,
		},
		{
			name:    "When model has no nextn tensors, it should not detect MTP",
			tensors: []string{"blk.0.attn_k.weight", "blk.0.ffn_gate.weight"},
			wantMTP: false,
		},
		{
			name:    "When model has no tensors, it should not detect MTP",
			tensors: nil,
			wantMTP: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestGGUFWithTensors(t, map[string]any{
				"general.architecture":  "test",
				"test.context_length":   uint32(131072),
				"test.block_count":      uint32(32),
				"test.head_count_kv":    uint32(8),
				"test.head_count":       uint32(32),
				"test.embedding_length": uint32(4096),
			}, tt.tensors)

			meta, err := readGGUFMeta(path)
			if err != nil {
				t.Fatalf("readGGUFMeta failed: %v", err)
			}
			if meta.HasMTP != tt.wantMTP {
				t.Errorf("HasMTP = %v, want %v", meta.HasMTP, tt.wantMTP)
			}
		})
	}
}

func TestValidateProfile_MTPFlagNoHeads(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.nomtp.model", modelFile)
	viper.Set("profiles.nomtp.flags", []string{"--spec-type", "draft-mtp"})

	v := validateProfile("nomtp", ggufDir)

	found := false
	for _, w := range v.Warnings {
		if w == "--spec-type draft-mtp but model has no MTP heads and no external drafter" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected MTP warning, got warnings: %v", v.Warnings)
	}
}

func TestValidateProfile_SpecTypeMTP(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUFWithTensors(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	}, []string{"blk.0.weight", "nextn0.blk.0.ffn_gate.weight"})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.mtp-model.model", modelFile)
	viper.Set("profiles.mtp-model.flags", []string{"--spec-type", "draft-mtp"})

	v := validateProfile("mtp-model", ggufDir)

	if v.SpecType != "mtp" {
		t.Errorf("When model has MTP tensors, SpecType should be mtp, got %q", v.SpecType)
	}
}

func TestValidateProfile_SpecTypeDFlash(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.dflash-model.model", modelFile)
	viper.Set("profiles.dflash-model.dflash", "drafter.gguf")
	viper.Set("profiles.dflash-model.repo", "org/repo")

	v := validateProfile("dflash-model", ggufDir)

	if v.SpecType != "dflash" {
		t.Errorf("When dflash set, SpecType should be dflash, got %q", v.SpecType)
	}
}

func TestValidateProfile_DFlashNoRepo(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.norepo.model", modelFile)
	viper.Set("profiles.norepo.dflash", "drafter.gguf")

	v := validateProfile("norepo", ggufDir)

	found := false
	for _, w := range v.Warnings {
		if w == "dflash set but no dflash_repo or repo for sync" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dflash repo warning, got warnings: %v", v.Warnings)
	}
}

func TestValidateProfile_MTPDrafterNoRepo(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":  "test",
		"test.context_length":   uint32(131072),
		"test.block_count":      uint32(32),
		"test.head_count_kv":    uint32(8),
		"test.head_count":       uint32(32),
		"test.embedding_length": uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.norepo.model", modelFile)
	viper.Set("profiles.norepo.mtp_drafter", "drafter.gguf")
	viper.Set("profiles.norepo.flags", []string{"--model-draft", "/path/drafter.gguf"})

	v := validateProfile("norepo", ggufDir)

	found := false
	for _, w := range v.Warnings {
		if w == "mtp_drafter set but no mtp_drafter_repo or repo for sync" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mtp_drafter repo warning, got warnings: %v", v.Warnings)
	}
}

func TestNewProfileValidateCmd_Registration(t *testing.T) {
	cmd := NewProfileCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "validate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("When listing profile subcommands, validate should be registered")
	}
}

// writeTestGGUF creates a minimal GGUF v3 file with given KV metadata.
// Supports string (for architecture) and uint32 values.
func writeTestGGUF(t *testing.T, kvPairs map[string]any) string {
	return writeTestGGUFWithTensors(t, kvPairs, nil)
}

func writeTestGGUFWithTensors(t *testing.T, kvPairs map[string]any, tensorNames []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.gguf")
	if err != nil {
		t.Fatalf("create temp gguf: %v", err)
	}
	defer f.Close()

	f.Write([]byte("GGUF"))
	binary.Write(f, binary.LittleEndian, uint32(3))
	binary.Write(f, binary.LittleEndian, uint64(len(tensorNames)))
	binary.Write(f, binary.LittleEndian, uint64(len(kvPairs)))

	for key, val := range kvPairs {
		binary.Write(f, binary.LittleEndian, uint64(len(key)))
		f.Write([]byte(key))

		switch v := val.(type) {
		case string:
			binary.Write(f, binary.LittleEndian, uint32(8))
			binary.Write(f, binary.LittleEndian, uint64(len(v)))
			f.Write([]byte(v))
		case uint32:
			binary.Write(f, binary.LittleEndian, uint32(4))
			binary.Write(f, binary.LittleEndian, v)
		default:
			t.Fatalf("unsupported test GGUF value type: %T", val)
		}
	}

	for _, name := range tensorNames {
		binary.Write(f, binary.LittleEndian, uint64(len(name)))
		f.Write([]byte(name))
		binary.Write(f, binary.LittleEndian, uint32(1))  // n_dims = 1
		binary.Write(f, binary.LittleEndian, uint64(64)) // dim[0]
		binary.Write(f, binary.LittleEndian, uint32(0))  // type F32
		binary.Write(f, binary.LittleEndian, uint64(0))  // offset
	}

	return f.Name()
}
