package core

// AgentRegistry resolves a mode/agent name to the agent the runtime should use.
// It is a placeholder for the eventual real registry; today it only needs to
// supply the built-in default when no mode is set.
type AgentRegistry struct {
	defaultAgent string
}

// NewAgentRegistry creates a registry with built-in defaults.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{defaultAgent: "build"}
}

// ResolveAgent normalises a mode/agent name and falls back to the default.
func (r *AgentRegistry) ResolveAgent(mode string) string {
	if mode == "" {
		return r.defaultAgent
	}
	return mode
}
