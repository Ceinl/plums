package core

// AgentRegistry is a factory and registry for agents.
// FOR NOW this contains temporary fake outputs so the rest of the application
// can compile and run while the real core package is being built.
type AgentRegistry struct {
	defaultAgent string
	agents       []string
}

// NewAgentRegistry creates a registry with built-in defaults.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		defaultAgent: "build",
		agents:       []string{"build", "plan"},
	}
}

// DefaultAgent returns the fallback agent when none is configured.
func (r *AgentRegistry) DefaultAgent() string {
	return r.defaultAgent
}

// ResolveAgent normalises a mode/agent name and falls back to the default.
func (r *AgentRegistry) ResolveAgent(mode string) string {
	if mode == "" {
		return r.defaultAgent
	}
	return mode
}

// ListAgents returns the names of registered agents.
// (fake output – will be replaced with real registry lookup.)
func (r *AgentRegistry) ListAgents() []string {
	return r.agents
}

// AgentCapabilities returns the capabilities for a given agent.
// (fake output – will be replaced with real capability discovery.)
func (r *AgentRegistry) AgentCapabilities(agent string) []string {
	return nil
}

// AgentDescription returns a human-readable description for an agent.
// (fake output – will be replaced with real metadata.)
func (r *AgentRegistry) AgentDescription(agent string) string {
	return ""
}
