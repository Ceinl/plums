package app

import (
	"reflect"
	"testing"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func TestParseQuestionAnswersSingleCustom(t *testing.T) {
	req := &adapter.QuestionRequest{Questions: []adapter.QuestionInfo{{Question: "Name?"}}}
	got := parseQuestionAnswers("  Alice  ", req)
	want := [][]string{{"Alice"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestParseQuestionAnswersMultipleChoice(t *testing.T) {
	req := &adapter.QuestionRequest{Questions: []adapter.QuestionInfo{
		{Question: "Pick env"},
		{Question: "Pick features", Multiple: true},
	}}
	got := parseQuestionAnswers("Production\nA, B", req)
	want := [][]string{{"Production"}, {"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
