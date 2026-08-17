package profile

import (
	"encoding/binary"
	"os"
	"path/filepath"
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

func TestReadGGUFMeta_ValidFile(t *testing.T) {
	path := writeTestGGUF(t, map[string]any{
		"general.architecture":     "qwen35",
		"qwen35.context_length":    uint32(131072),
		"qwen35.block_count":       uint32(64),
		"qwen35.head_count_kv":     uint32(4),
		"qwen35.head_count":        uint32(32),
		"qwen35.embedding_length":  uint32(4096),
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
		"general.architecture":    "test",
		"test.context_length":     uint32(32768),
		"test.block_count":        uint32(32),
		"test.head_count_kv":      uint32(8),
		"test.head_count":         uint32(32),
		"test.embedding_length":   uint32(4096),
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

func TestValidateProfile_MTPDrafterWarning(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	ggufDir := t.TempDir()
	modelPath := writeTestGGUF(t, map[string]any{
		"general.architecture":    "test",
		"test.context_length":     uint32(131072),
		"test.block_count":        uint32(32),
		"test.head_count_kv":      uint32(8),
		"test.head_count":         uint32(32),
		"test.embedding_length":   uint32(4096),
	})
	modelFile := filepath.Base(modelPath)
	os.Rename(modelPath, filepath.Join(ggufDir, modelFile))

	viper.Set("profiles.drafter-test.model", modelFile)
	viper.Set("profiles.drafter-test.mtp_drafter", "drafter.gguf")
	viper.Set("profiles.drafter-test.flags", []string{"--spec-type", "draft-mtp"})

	v := validateProfile("drafter-test", ggufDir)

	foundFlag := false
	foundFile := false
	for _, w := range v.Warnings {
		if w == "mtp_drafter set but --model-draft not in flags" {
			foundFlag = true
		}
		if w == "mtp_drafter not on disk: drafter.gguf" {
			foundFile = true
		}
	}
	if !foundFlag {
		t.Errorf("expected --model-draft warning, got warnings: %v", v.Warnings)
	}
	if !foundFile {
		t.Errorf("expected file-missing warning, got warnings: %v", v.Warnings)
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
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.gguf")
	if err != nil {
		t.Fatalf("create temp gguf: %v", err)
	}
	defer f.Close()

	// Header
	f.Write([]byte("GGUF"))
	binary.Write(f, binary.LittleEndian, uint32(3))        // version
	binary.Write(f, binary.LittleEndian, uint64(0))        // n_tensors
	binary.Write(f, binary.LittleEndian, uint64(len(kvPairs))) // n_kv

	for key, val := range kvPairs {
		// Key string
		binary.Write(f, binary.LittleEndian, uint64(len(key)))
		f.Write([]byte(key))

		switch v := val.(type) {
		case string:
			binary.Write(f, binary.LittleEndian, uint32(8)) // string type
			binary.Write(f, binary.LittleEndian, uint64(len(v)))
			f.Write([]byte(v))
		case uint32:
			binary.Write(f, binary.LittleEndian, uint32(4)) // uint32 type
			binary.Write(f, binary.LittleEndian, v)
		default:
			t.Fatalf("unsupported test GGUF value type: %T", val)
		}
	}

	return f.Name()
}
