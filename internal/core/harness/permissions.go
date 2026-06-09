package harness

type Permission uint64

const (

	// Allow Agent to read files from session root and downwards
	PermissionRead Permission = 1 << iota

	// Allow Agent to read files from user root folder and downwards
	PermissionLocalSearch

	// Allow Agent to edit files
	PermissionEdit

	// Allow bash execution of trusted commands, by default commands that do not change anything on PC
	PermissionBashSafe

	// Allow bash execution of ALL commands
	PermissionBashUnsafe

	// Allow bash execution of commands specefied in config file, in case user wants to allow only specific commands
	PermissionBashCustom

	// Allow usign WebSearch tools
	PermissionWebSearch

	// PermissionAll is a bitmask that allows all permissions
	// Permission(0) = 0000...000, and ^ is reversing it, so each bit is set to 1
	PermissionAll = ^Permission(0)
)

var permissionByName = map[string]Permission{
	"all":         PermissionAll,
	"read":        PermissionRead,
	"local":       PermissionLocalSearch,
	"edit":        PermissionEdit,
	"bash":        PermissionBashSafe,
	"bash_unsafe": PermissionBashUnsafe,
	"bash_custom": PermissionBashCustom,
	"web":         PermissionWebSearch,
}

func PermissionByName(name string) (Permission, bool) {
	perm, ok := permissionByName[name]
	return perm, ok
}

func (p Permission) IsAllowed(required Permission) bool {
	return p&required == required
}

func (p *Permission) Allow(required Permission) {
	*p |= required
}

func (p *Permission) Deny(required Permission) {
	*p &^= required
}
