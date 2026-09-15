package benchmark

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jparrill/auriga-cli/internal/benchmark/formats"
)

func TestIFEvalBuildPrompt(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID: "ifeval_1000",
		Prompt: "Write a 300+ word summary of the Declaration of Independence.",
	}

	prompt, err := runner.BuildPrompt(problem, formats.Suite{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, problem.Prompt) {
		t.Error("prompt should contain the original problem prompt")
	}
	if !strings.Contains(prompt, ifevalSystemPrompt) {
		t.Error("prompt should contain the system prompt")
	}
}

func TestIFEvalValidateResponse_NoComma_Pass(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1001",
		Prompt:            "Write without commas.",
		InstructionIDList: []string{"punctuation:no_comma"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("This is a response without any commas at all", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_NoComma_Fail(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1002",
		Prompt:            "Write without commas.",
		InstructionIDList: []string{"punctuation:no_comma"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("This has a comma, right here, and another one", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for response with commas")
	}
	if !strings.Contains(errMsg, "punctuation:no_comma") {
		t.Errorf("error should mention the instruction, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "2 commas") {
		t.Errorf("error should report comma count, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_Lowercase_Pass(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1003",
		Prompt:            "Write in lowercase.",
		InstructionIDList: []string{"change_case:english_lowercase"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("this is all lowercase text with numbers 123 and symbols !@#", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_Lowercase_Fail(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1004",
		Prompt:            "Write in lowercase.",
		InstructionIDList: []string{"change_case:english_lowercase"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("This Has Uppercase Letters", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for response with uppercase")
	}
	if !strings.Contains(errMsg, "uppercase") {
		t.Errorf("error should mention uppercase, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_WordCount_Pass(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1005",
		Prompt:            "Write at least 10 words.",
		InstructionIDList: []string{"length_constraints:number_words"},
		Kwargs:            json.RawMessage(`[{"relation": "at least", "num_words": 10}]`),
	}

	response := "one two three four five six seven eight nine ten eleven twelve"
	ok, errMsg, err := runner.ValidateResponse(response, problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_WordCount_Fail(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1006",
		Prompt:            "Write at least 10 words.",
		InstructionIDList: []string{"length_constraints:number_words"},
		Kwargs:            json.RawMessage(`[{"relation": "at least", "num_words": 10}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("only five words here now", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for too few words")
	}
	if !strings.Contains(errMsg, "words") {
		t.Errorf("error should mention words, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_WordCount_AtMost(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1007",
		Prompt:            "Write at most 5 words.",
		InstructionIDList: []string{"length_constraints:number_words"},
		Kwargs:            json.RawMessage(`[{"relation": "at most", "num_words": 5}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("one two three four five six seven", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for too many words")
	}
	if !strings.Contains(errMsg, "at most") {
		t.Errorf("error should mention at most, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_MultipleInstructions(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1008",
		Prompt:            "Write lowercase with no commas.",
		InstructionIDList: []string{"punctuation:no_comma", "change_case:english_lowercase"},
		Kwargs:            json.RawMessage(`[{}, {}]`),
	}

	ok, errMsg, err := runner.ValidateResponse("this is a clean response without commas", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected pass, got fail: %s", errMsg)
	}

	ok, errMsg, err = runner.ValidateResponse("This has Uppercase, and commas", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for response with both violations")
	}
	if !strings.Contains(errMsg, "punctuation:no_comma") {
		t.Errorf("error should mention no_comma, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "change_case:english_lowercase") {
		t.Errorf("error should mention lowercase, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_Uppercase(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1009",
		Prompt:            "Write in uppercase.",
		InstructionIDList: []string{"change_case:english_capital"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, _, err := runner.ValidateResponse("ALL UPPERCASE 123!", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass for all uppercase response")
	}

	ok, _, err = runner.ValidateResponse("NOT all UPPERCASE here", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail for mixed case response")
	}
}

func TestIFEvalValidateResponse_EndChecker(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1010",
		Prompt:            "End with a specific phrase.",
		InstructionIDList: []string{"startend:end_checker"},
		Kwargs:            json.RawMessage(`[{"end_phrase": "The End."}]`),
	}

	ok, _, err := runner.ValidateResponse("Some content here. The End.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass when response ends with the phrase")
	}

	ok, errMsg, err := runner.ValidateResponse("Some content here. Not the end.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail when response does not end with the phrase")
	}
	if !strings.Contains(errMsg, "does not end with") {
		t.Errorf("error should mention end phrase, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_Keywords(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1011",
		Prompt:            "Include these keywords.",
		InstructionIDList: []string{"keywords:existence"},
		Kwargs:            json.RawMessage(`[{"keywords": ["apple", "banana"]}]`),
	}

	ok, _, err := runner.ValidateResponse("I like apple pie and banana smoothies.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass when all keywords present")
	}

	ok, errMsg, err := runner.ValidateResponse("I like apple pie.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail when keyword missing")
	}
	if !strings.Contains(errMsg, "banana") {
		t.Errorf("error should mention missing keyword, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_ForbiddenWords(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1012",
		Prompt:            "Avoid certain words.",
		InstructionIDList: []string{"keywords:forbidden_words"},
		Kwargs:            json.RawMessage(`[{"forbidden_words": ["bad", "evil"]}]`),
	}

	ok, _, err := runner.ValidateResponse("This is a good and kind response.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass when no forbidden words present")
	}

	ok, errMsg, err := runner.ValidateResponse("This is a bad and evil response.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail when forbidden words present")
	}
	if !strings.Contains(errMsg, "forbidden words") {
		t.Errorf("error should mention forbidden words, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_HighlightedSections(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1013",
		Prompt:            "Highlight sections.",
		InstructionIDList: []string{"detectable_format:number_highlighted_sections"},
		Kwargs:            json.RawMessage(`[{"num_highlights": 3}]`),
	}

	ok, _, err := runner.ValidateResponse("*Section One* text *Section Two* more *Section Three* end", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass with 3 highlighted sections")
	}

	ok, errMsg, err := runner.ValidateResponse("*Section One* text *Section Two* end", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail with only 2 highlighted sections")
	}
	if !strings.Contains(errMsg, "highlighted sections") {
		t.Errorf("error should mention highlighted sections, got: %s", errMsg)
	}
}

func TestIFEvalValidateResponse_Paragraphs(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1014",
		Prompt:            "Write at least 3 paragraphs.",
		InstructionIDList: []string{"length_constraints:number_paragraphs"},
		Kwargs:            json.RawMessage(`[{"relation": "at least", "num_paragraphs": 3}]`),
	}

	response := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here."
	ok, _, err := runner.ValidateResponse(response, problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass with 3 paragraphs")
	}

	ok, _, err = runner.ValidateResponse("Just one paragraph.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail with only 1 paragraph")
	}
}

func TestIFEvalValidateResponse_Sentences(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1015",
		Prompt:            "Write at least 3 sentences.",
		InstructionIDList: []string{"length_constraints:number_sentences"},
		Kwargs:            json.RawMessage(`[{"relation": "at least", "num_sentences": 3}]`),
	}

	ok, _, err := runner.ValidateResponse("First sentence. Second sentence. Third sentence.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected pass with 3 sentences")
	}

	ok, _, err = runner.ValidateResponse("Just one sentence.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected fail with only 1 sentence")
	}
}

func TestIFEvalValidateResponse_UnknownInstruction(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID:            "ifeval_1016",
		Prompt:            "Some prompt.",
		InstructionIDList: []string{"unknown:future_instruction"},
		Kwargs:            json.RawMessage(`[{}]`),
	}

	ok, _, err := runner.ValidateResponse("Any response is fine.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("unknown instructions should be skipped, not fail")
	}
}

func TestIFEvalValidateResponse_NoInstructions(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID: "ifeval_1017",
		Prompt: "Some prompt.",
	}

	ok, _, err := runner.ValidateResponse("Any response.", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("no instructions should pass")
	}
}

func TestIFEvalBuildRetryPrompt(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID: "ifeval_1018",
		Prompt: "Write in lowercase.",
	}

	retry, err := runner.BuildRetryPrompt(problem, t.TempDir(), "failed: change_case:english_lowercase: contains uppercase characters")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(retry, "change_case:english_lowercase") {
		t.Error("retry prompt should contain the validation error")
	}
	if !strings.Contains(retry, problem.Prompt) {
		t.Error("retry prompt should contain the original prompt")
	}
}

func TestIFEvalValidateResponse_EvalFallback(t *testing.T) {
	runner := &IFEvalRunner{}
	problem := formats.Problem{
		TaskID: "ifeval_1019",
		Prompt: "Write without commas.",
		Eval:   []string{"punctuation:no_comma"},
		Kwargs: json.RawMessage(`[{}]`),
	}

	ok, _, err := runner.ValidateResponse("no commas here", problem, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("should use Eval field as fallback for InstructionIDList")
	}
}
