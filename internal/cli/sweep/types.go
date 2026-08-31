package sweep

type SweepConfig struct {
	Profile       string              `yaml:"profile"`
	Iterations    int                 `yaml:"iterations"`
	ProfileFields map[string][]string `yaml:"profile_fields"`
	Parameters    map[string][]string `yaml:"parameters"`
	Toggles       map[string][]string `yaml:"toggles"`
}

type Combination struct {
	ProfileFields map[string]string
	Parameters    map[string]string
	Toggles       map[string]string
}

type SweepResult struct {
	Index         int               `json:"index"`
	Overrides     map[string]string `json:"overrides"`
	ResolvedFlags []string          `json:"resolved_flags"`
	TTFT          int64             `json:"ttft_ms"`
	Prompt        float64           `json:"prompt_tok_s"`
	NoThink       BenchData         `json:"no_think"`
	Think         BenchData         `json:"think"`
	DurationMs    int64             `json:"duration_ms"`
	Error         string            `json:"error,omitempty"`
}

type BenchData struct {
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type SweepReport struct {
	Profile       string        `json:"profile"`
	Model         string        `json:"model"`
	Hostname      string        `json:"hostname"`
	Timestamp     string        `json:"timestamp"`
	TotalDuration int64         `json:"total_duration_ms"`
	Results       []SweepResult `json:"results"`
	Complete      bool          `json:"complete"`
}

type ValidationIssue struct {
	Level   string // "error" or "warning"
	Check   string // category: "profile", "binary", "parameter", "toggle", "iterations", "value"
	Message string
}

var knownParameters = map[string]bool{
	"cache-type":       true,
	"cache-type-k":     true,
	"cache-type-v":     true,
	"batch":            true,
	"batch-size":       true,
	"ubatch-size":      true,
	"threads":          true,
	"spec-draft-p-min": true,
	"spec-draft-n-max": true,
}

type aliasTarget struct {
	Flag   string
	Divide int // 0 = use value as-is, >0 = integer divide
}

var paramAliases = map[string][]aliasTarget{
	"cache-type": {{Flag: "cache-type-k"}, {Flag: "cache-type-v"}},
	"batch":      {{Flag: "batch-size"}, {Flag: "ubatch-size", Divide: 4}},
}

var knownToggles = map[string]bool{
	"np":        true,
	"load-mode": true,
}

var knownProfileFields = map[string]bool{
	"bin": true,
}

var validQuantTypes = map[string]bool{
	"q4_0": true, "q4_1": true, "q5_0": true, "q5_1": true,
	"q8_0": true, "q2_k": true, "q3_k": true, "q4_k": true,
	"q5_k": true, "q6_k": true, "f16": true, "f32": true,
}
