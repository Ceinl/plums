package harness

// Bitmask for Agent permissions, two states, allow or no.
// No to annoying "asking"
type Permission uint64

const (

	// PermissionAll is a bitmask that allows all permissions
	// Permission(0) = 0000...000, and ^ is revering it, so each bit is set to 1
	PermissionAll = ^Permission(0)

	//Allow Agent to read files from session root and downwards
	PermissionRead Permission = 1 << iota

	//Allow Agent to read files from user root folder
	PermissionLocalSearch

	//Allow Agent to edit files
	PermissionEdit

	//Allow bask execution of trusted commands, by default commands that do not change anything on PC
	PermissionBashSafe

	//Allow bash execution of ALL commands
	PermissionBashUnsafe

	//Allow bash execution of commands specefied in config file, in case user wants to allow only specific commands
	PermissionBashCustom

	//Allow usign WebSearch tools
	PermissionWebSearch
)

func (p Permission) IsAllowed(required Permission) bool {
	return p&required == required
}

func (p *Permission) Allow(required Permission) {
	*p |= required
}

func (p *Permission) Deny(required Permission) {
	*p &^= required
}
