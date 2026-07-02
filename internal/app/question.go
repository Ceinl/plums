package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/debuglog"
)

func replyQuestion(ctx context.Context, state *State, client capabilities.Backend, requestID string, answers [][]string, timeout time.Duration) bool {
	questions, err := backendQuestions(client)
	if err != nil {
		state.AddMessage("system", err.Error())
		return false
	}
	replyCtx, cancelReply := context.WithTimeout(ctx, timeout)
	err = questions.ReplyQuestion(replyCtx, requestID, answers)
	cancelReply()
	if err != nil {
		debuglog.Printf("question: reply failed: %v", err)
		state.AddMessage("system", fmt.Sprintf("failed to answer question: %v", err))
		return false
	}
	state.SetStreaming(true)
	return true
}
