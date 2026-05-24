package harness

import "fmt"

type AgentMode string
type AgentID string

type Registry map[AgentID]*Agent

const (
	ModeBuild   AgentMode = "build"
	ModePlan    AgentMode = "plan"
	ModeExplore AgentMode = "explore"
	ModeRogue   AgentMode = "rogue"
)

func (m AgentMode) String() string {
	return string(m)
}

type Config struct {
	Agents []Agent `json:"agents"`
}

type Agent struct {
	ID           AgentID    `json:"id"`
	Mode         AgentMode  `json:"mode"`
	SystemPrompt string     `json:"system_prompt"`
	Permissions  Permission `json:"permissions"`
}

func NewAgent(id AgentID, mode AgentMode, permissions Permission, systemPrompt string) *Agent {
	return &Agent{
		ID:           id,
		Mode:         mode,
		SystemPrompt: systemPrompt,
		Permissions:  permissions,
	}
}

func RegistryFromConfig(cfg Config) (Registry, error) {
	registry := make(Registry, len(cfg.Agents))

	for i := range cfg.Agents {
		agent := &cfg.Agents[i]
		if agent.ID == "" {
			return nil, fmt.Errorf("agent id is required")
		}
		if _, exists := registry[agent.ID]; exists {
			return nil, fmt.Errorf("duplicate agent id %q", agent.ID)
		}

		registry[agent.ID] = agent
	}

	return registry, nil
}

func DefaultRegistry() Registry {
	return Registry{
		AgentID(ModeBuild):   NewBuildAgent(),
		AgentID(ModePlan):    NewPlanAgent(),
		AgentID(ModeExplore): NewExploreAgent(),
	}
}

// Fallback default config
func NewBuildAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead | PermissionEdit)
	return NewAgent(AgentID(ModeBuild), ModeBuild, permission, "TODO")
}

func NewPlanAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead)
	return NewAgent(AgentID(ModePlan), ModePlan, permission, "TODO")
}

func NewExploreAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead | PermissionWebSearch | PermissionLocalSearch)
	return NewAgent(AgentID(ModeExplore), ModeExplore, permission, "TODO")
}

func NewRogueAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionAll)
	return NewAgent(AgentID(ModeRogue), ModeRogue, permission, "TODO")
}
