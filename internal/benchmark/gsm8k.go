package benchmark

import (
	"regexp"
	"strings"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func init() {
	formats.Register("gsm8k", &GSM8KRunner{})
}

type GSM8KRunner struct{}

const gsm8kSystemPrompt = `You are an expert math problem solver. Solve the following problem step by step.
Show your reasoning clearly, then give your final numeric answer on the last line in this exact format:
#### <number>

For example, if the answer is 42, end with:
#### 42`

func (g *GSM8KRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	return gsm8kSystemPrompt + "\n\n" + problem.Prompt, nil
}

func (g *GSM8KRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	expected := extractGSM8KAnswer(problem.Test)
	if expected == "" {
		return false, "no expected answer found in problem", nil
	}

	got := extractGSM8KAnswer(response)
	if got == "" {
		return false, "model response missing #### <number> answer marker", nil
	}

	expectedNorm := normalizeNumber(expected)
	gotNorm := normalizeNumber(got)

	if expectedNorm == gotNorm {
		return true, "", nil
	}
	return false, "expected " + expectedNorm + " got " + gotNorm, nil
}

func (g *GSM8KRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	return "Your previous answer was incorrect: " + validationError + "\n\n" +
		"Try again. Solve this problem step by step and end with #### <number>.\n\n" +
		problem.Prompt, nil
}

var gsm8kAnswerRe = regexp.MustCompile(`####\s*(.+)`)

func extractGSM8KAnswer(text string) string {
	matches := gsm8kAnswerRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

func normalizeNumber(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimRight(s, ".")
	return s
}
