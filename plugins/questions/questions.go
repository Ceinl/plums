// Package questions owns default question prompt rendering and answer parsing.
package questions

import (
	"strings"

	"github.com/Ceinl/plums/capabilities"
)

type Plugin struct{}

func (*Plugin) Name() string                      { return "questions" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Title(req *capabilities.QuestionRequest) string {
	if req == nil || len(req.Questions) == 0 {
		return "Question"
	}
	q := req.Questions[0]
	if q.Header != "" {
		return q.Header + ": " + q.Question
	}
	return q.Question
}

func (*Plugin) Options(req *capabilities.QuestionRequest) []capabilities.QuestionOption {
	if req == nil || len(req.Questions) == 0 {
		return nil
	}
	return append([]capabilities.QuestionOption(nil), req.Questions[0].Options...)
}

func (*Plugin) ParseAnswers(input string, req *capabilities.QuestionRequest) [][]string {
	if req == nil || len(req.Questions) == 0 {
		return [][]string{{strings.TrimSpace(input)}}
	}

	lines := splitNonEmptyLines(input)
	answers := make([][]string, len(req.Questions))
	for i, q := range req.Questions {
		answer := ""
		if len(req.Questions) == 1 {
			answer = strings.TrimSpace(input)
		} else if i < len(lines) {
			answer = lines[i]
		}
		if q.Multiple {
			answers[i] = splitCommaAnswers(answer)
		} else {
			answers[i] = []string{answer}
		}
	}
	return answers
}

func splitNonEmptyLines(input string) []string {
	var out []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitCommaAnswers(input string) []string {
	var out []string
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(input)}
	}
	return out
}
