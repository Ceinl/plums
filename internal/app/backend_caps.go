package app

import (
	"fmt"

	"github.com/Ceinl/plums/capabilities"
)

func backendSessions(backend capabilities.Backend) (capabilities.BackendSessions, error) {
	if sessions, ok := backend.(capabilities.BackendSessions); ok {
		return sessions, nil
	}
	return nil, fmt.Errorf("backend does not support sessions")
}

func backendModels(backend capabilities.Backend) (capabilities.BackendModels, error) {
	if models, ok := backend.(capabilities.BackendModels); ok {
		return models, nil
	}
	return nil, fmt.Errorf("backend does not support model listing")
}

func backendQuestions(backend capabilities.Backend) (capabilities.BackendQuestions, error) {
	if questions, ok := backend.(capabilities.BackendQuestions); ok {
		return questions, nil
	}
	return nil, fmt.Errorf("backend does not support questions")
}
