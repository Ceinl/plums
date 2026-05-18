package harness

// Bitmask for Agent permissions, two states, allow or no.
// No to annoying "asking"
type Permision uint64

const (
	//Allow Agent to read files from session root and downwards
	PermissionRead Permision = 1 << iota

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

func (p Permision) IsAllowed(required Permision) bool {
	return p&required == required
}

func (p *Permision) Allow(required Permision) {
	*p |= required
}

func (p *Permision) Deny(required Permision) {
	*p &^= required
}
