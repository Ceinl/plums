package ai

import (
	"encoding/json"
	"testing"
)

func TestSendMessageBodyIncludesAgentMode(t *testing.T) {
	body := newSendMessageBody("hello", "openai", "gpt-5.5", "plan")
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got["agent"] != "plan" {
		t.Fatalf("expected agent mode plan, got %#v", got["agent"])
	}
	model, ok := got["model"].(map[string]any)
	if !ok {
		t.Fatalf("expected model object, got %#v", got["model"])
	}
	if model["providerID"] != "openai" || model["modelID"] != "gpt-5.5" {
		t.Fatalf("unexpected model: %#v", model)
	}
}

func TestSendMessageBodyOmitsEmptyAgentMode(t *testing.T) {
	body := newSendMessageBody("hello", "", "", "")
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := got["agent"]; ok {
		t.Fatalf("expected empty agent mode to be omitted, got %#v", got["agent"])
	}
	if _, ok := got["model"]; ok {
		t.Fatalf("expected empty model to be omitted, got %#v", got["model"])
	}
}
