package harness

type AgentMode string

const (
	ModeBuild   AgentMode = "build"
	ModePlan    AgentMode = "plan"
	ModeExplore AgentMode = "explore"
	ModeRogue   AgentMode = "rogue" // Absolute all permissions
)

func (m AgentMode) String() string {
	return string(m)
}

type Agent struct {
	Mode              AgentMode
	SystemPrompt      string
	AgentPermission   Permission
	SessionPermission Permission
}

func NewAgent(mode AgentMode, p Permission, systemPrompt string) *Agent {
	return &Agent{
		Mode:              mode,
		SystemPrompt:      systemPrompt,
		AgentPermission:   p,
		SessionPermission: 0,
	}
}

func NewBuildAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead | PermissionEdit)
	return NewAgent(ModeBuild, permission, "TODO")
}

func NewPlanAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead)
	return NewAgent(ModePlan, permission, "TODO")
}

func NewExploreAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionBashCustom | PermissionBashSafe | PermissionRead | PermissionWebSearch | PermissionLocalSearch)
	return NewAgent(ModeExplore, permission, "TODO")
}

func NewRogueAgent() *Agent {
	var permission Permission
	permission.Allow(PermissionAll)
	return NewAgent(ModeRogue, permission, "TODO")
}
