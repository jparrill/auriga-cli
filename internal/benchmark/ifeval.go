package benchmark

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func init() {
	formats.Register("ifeval", &IFEvalRunner{})
}

type IFEvalRunner struct{}

const ifevalSystemPrompt = "Follow the instructions in the prompt exactly."

func (r *IFEvalRunner) BuildPrompt(problem formats.Problem, suite formats.Suite) (string, error) {
	return fmt.Sprintf("%s\n\n%s", ifevalSystemPrompt, problem.Prompt), nil
}

func (r *IFEvalRunner) ValidateResponse(response string, problem formats.Problem, workDir string) (bool, string, error) {
	instructions := problem.InstructionIDList
	if len(instructions) == 0 {
		instructions = problem.Eval
	}
	if len(instructions) == 0 {
		return true, "", nil
	}

	kwargsList := parseKwargs(problem.Kwargs, len(instructions))

	var failures []string
	for i, instruction := range instructions {
		kwargs := kwargsList[i]
		ok, reason := checkIFEvalInstruction(response, instruction, kwargs)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: %s", instruction, reason))
		}
	}

	if len(failures) > 0 {
		return false, fmt.Sprintf("failed: %s", strings.Join(failures, "; ")), nil
	}
	return true, "", nil
}

func (r *IFEvalRunner) BuildRetryPrompt(problem formats.Problem, workDir string, validationError string) (string, error) {
	return fmt.Sprintf(`Your previous response did not follow all instructions. Errors:
%s

Try again. Follow every instruction exactly.

%s`, validationError, problem.Prompt), nil
}

func parseKwargs(raw json.RawMessage, n int) []map[string]interface{} {
	if len(raw) > 0 {
		var list []map[string]interface{}
		if err := json.Unmarshal(raw, &list); err == nil {
			for len(list) < n {
				list = append(list, map[string]interface{}{})
			}
			return list
		}
	}
	result := make([]map[string]interface{}, n)
	for i := range result {
		result[i] = map[string]interface{}{}
	}
	return result
}

func checkIFEvalInstruction(response, instruction string, kwargs map[string]interface{}) (bool, string) {
	switch instruction {
	case "punctuation:no_comma":
		return checkNoComma(response)
	case "length_constraints:number_words":
		return checkWordCount(response, kwargs)
	case "detectable_format:number_highlighted_sections":
		return checkHighlightedSections(response, kwargs)
	case "length_constraints:number_sentences":
		return checkSentenceCount(response, kwargs)
	case "length_constraints:number_paragraphs":
		return checkParagraphCount(response, kwargs)
	case "change_case:english_lowercase":
		return checkLowercase(response)
	case "change_case:english_capital":
		return checkUppercase(response)
	case "startend:end_checker":
		return checkEndPhrase(response, kwargs)
	case "keywords:existence":
		return checkKeywordsExist(response, kwargs)
	case "keywords:forbidden_words":
		return checkForbiddenWords(response, kwargs)
	default:
		return true, ""
	}
}

func checkNoComma(response string) (bool, string) {
	count := strings.Count(response, ",")
	if count > 0 {
		return false, fmt.Sprintf("found %d commas", count)
	}
	return true, ""
}

func checkWordCount(response string, kwargs map[string]interface{}) (bool, string) {
	relation, _ := kwargs["relation"].(string)
	numWords := intFromKwarg(kwargs, "num_words")
	if numWords == 0 {
		return true, ""
	}
	words := len(strings.Fields(response))
	return checkRelation(words, numWords, relation, "words")
}

func checkHighlightedSections(response string, kwargs map[string]interface{}) (bool, string) {
	numHighlights := intFromKwarg(kwargs, "num_highlights")
	if numHighlights == 0 {
		return true, ""
	}
	re := regexp.MustCompile(`\*[^*]+\*`)
	matches := re.FindAllString(response, -1)
	if len(matches) < numHighlights {
		return false, fmt.Sprintf("found %d highlighted sections, need at least %d", len(matches), numHighlights)
	}
	return true, ""
}

func checkSentenceCount(response string, kwargs map[string]interface{}) (bool, string) {
	relation, _ := kwargs["relation"].(string)
	numSentences := intFromKwarg(kwargs, "num_sentences")
	if numSentences == 0 {
		return true, ""
	}
	re := regexp.MustCompile(`[.!?]+`)
	sentences := len(re.FindAllString(response, -1))
	return checkRelation(sentences, numSentences, relation, "sentences")
}

func checkParagraphCount(response string, kwargs map[string]interface{}) (bool, string) {
	relation, _ := kwargs["relation"].(string)
	numParagraphs := intFromKwarg(kwargs, "num_paragraphs")
	if numParagraphs == 0 {
		return true, ""
	}
	parts := strings.Split(response, "\n\n")
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	return checkRelation(count, numParagraphs, relation, "paragraphs")
}

func checkLowercase(response string) (bool, string) {
	for _, r := range response {
		if unicode.IsUpper(r) {
			return false, "contains uppercase characters"
		}
	}
	return true, ""
}

func checkUppercase(response string) (bool, string) {
	for _, r := range response {
		if unicode.IsLower(r) {
			return false, "contains lowercase characters"
		}
	}
	return true, ""
}

func checkEndPhrase(response string, kwargs map[string]interface{}) (bool, string) {
	endPhrase, _ := kwargs["end_phrase"].(string)
	if endPhrase == "" {
		return true, ""
	}
	trimmed := strings.TrimSpace(response)
	if !strings.HasSuffix(trimmed, endPhrase) {
		return false, fmt.Sprintf("does not end with %q", endPhrase)
	}
	return true, ""
}

func checkKeywordsExist(response string, kwargs map[string]interface{}) (bool, string) {
	keywords := stringsFromKwarg(kwargs, "keywords")
	lower := strings.ToLower(response)
	var missing []string
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	if len(missing) > 0 {
		return false, fmt.Sprintf("missing keywords: %s", strings.Join(missing, ", "))
	}
	return true, ""
}

func checkForbiddenWords(response string, kwargs map[string]interface{}) (bool, string) {
	forbidden := stringsFromKwarg(kwargs, "forbidden_words")
	lower := strings.ToLower(response)
	var found []string
	for _, fw := range forbidden {
		if strings.Contains(lower, strings.ToLower(fw)) {
			found = append(found, fw)
		}
	}
	if len(found) > 0 {
		return false, fmt.Sprintf("contains forbidden words: %s", strings.Join(found, ", "))
	}
	return true, ""
}

func checkRelation(actual, expected int, relation, unit string) (bool, string) {
	switch relation {
	case "at least":
		if actual < expected {
			return false, fmt.Sprintf("found %d %s, need at least %d", actual, unit, expected)
		}
	case "at most":
		if actual > expected {
			return false, fmt.Sprintf("found %d %s, need at most %d", actual, unit, expected)
		}
	default:
		if actual < expected {
			return false, fmt.Sprintf("found %d %s, need at least %d", actual, unit, expected)
		}
	}
	return true, ""
}

func intFromKwarg(kwargs map[string]interface{}, key string) int {
	v, ok := kwargs[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func stringsFromKwarg(kwargs map[string]interface{}, key string) []string {
	v, ok := kwargs[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
