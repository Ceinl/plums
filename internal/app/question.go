package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/debuglog"
)

func replyQuestion(ctx context.Context, state *State, client adapter.Backend, requestID string, answers [][]string, timeout time.Duration) bool {
	replyCtx, cancelReply := context.WithTimeout(ctx, timeout)
	err := client.ReplyQuestion(replyCtx, requestID, answers)
	cancelReply()
	if err != nil {
		debuglog.Printf("question: reply failed: %v", err)
		state.AddMessage("system", fmt.Sprintf("failed to answer question: %v", err))
		return false
	}
	state.SetStreaming(true)
	return true
}

func questionTitle(req *adapter.QuestionRequest) string {
	if req == nil || len(req.Questions) == 0 {
		return "Question"
	}
	q := req.Questions[0]
	if q.Header != "" {
		return q.Header + ": " + q.Question
	}
	return q.Question
}

func questionOptionItems(req *adapter.QuestionRequest) []QuestionOptionItem {
	if req == nil || len(req.Questions) == 0 {
		return nil
	}
	q := req.Questions[0]
	items := make([]QuestionOptionItem, len(q.Options))
	for i, option := range q.Options {
		items[i] = QuestionOptionItem{Label: option.Label, Description: option.Description}
	}
	return items
}

func parseQuestionAnswers(input string, req *adapter.QuestionRequest) [][]string {
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
