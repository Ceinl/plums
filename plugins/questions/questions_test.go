package questions

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestParseAnswersSingleCustom(t *testing.T) {
	req := &capabilities.QuestionRequest{Questions: []capabilities.QuestionInfo{{Question: "Name?"}}}
	got := (&Plugin{}).ParseAnswers("  Alice  ", req)
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != "Alice" {
		t.Fatalf("answers = %#v", got)
	}
}

func TestParseAnswersMultipleChoice(t *testing.T) {
	req := &capabilities.QuestionRequest{Questions: []capabilities.QuestionInfo{
		{Question: "Pick env"},
		{Question: "Pick features", Multiple: true},
	}}
	got := (&Plugin{}).ParseAnswers("Production\nA, B", req)
	if len(got) != 2 || got[0][0] != "Production" || len(got[1]) != 2 || got[1][0] != "A" || got[1][1] != "B" {
		t.Fatalf("answers = %#v", got)
	}
}

func TestTitleAndOptions(t *testing.T) {
	req := &capabilities.QuestionRequest{Questions: []capabilities.QuestionInfo{{
		Header:   "Choose",
		Question: "Which?",
		Options:  []capabilities.QuestionOption{{Label: "A", Description: "first"}},
	}}}
	plugin := &Plugin{}
	if got := plugin.Title(req); got != "Choose: Which?" {
		t.Fatalf("Title() = %q", got)
	}
	options := plugin.Options(req)
	if len(options) != 1 || options[0].Label != "A" {
		t.Fatalf("Options() = %#v", options)
	}
}
