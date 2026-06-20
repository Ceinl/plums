// Package capabilities holds the plums plugin floor: the capability interfaces
// and domain types that plugin authors implement and consume. It has no
// dependencies on the rest of plums; both the public config package and the
// kernel build on top of it.
//
// This package was split out of the former top-level `plums` package as part of
// the v2 rebuild (Phase 0). Plugin authors import this; config authors import
// github.com/Ceinl/plums/config (which imports this).
package capabilities

import "context"

// Plugin is the narrow required interface for replaceable plums behavior. Init
// receives the per-plugin options declared alongside it in config.Plugin.Opts
// (opts is nil when none were supplied).
type Plugin interface {
	Name() string
	Init(Host, any) error
}

// Host is available only during plugin initialization. Services exposes the
// host capability services for registration-time use (e.g. contributing a
// Completion source); the same services are reachable at runtime via Ctx.
type Host interface {
	Settings() Settings
	Log(format string, args ...any)
	Services() Services
}

// Services is the set of host capability services Core provides and
// plugins/components consume. It is obtained from Host at Init (for
// registration, e.g. Completion.Register) and from Ctx at runtime (for
// invocation, e.g. Palette.Open / Clipboard.Copy). Theme and keybinds are NOT
// services — Theme is read via RenderCtx.Theme(); keybinds are Opts.Keybinds
// resolved centrally by Core.
type Services interface {
	Palette() Palette
	Clipboard() Clipboard
	Completion() Completion
	Selection() Selection
}

// Palette opens a list/command picker. It backs the cm-palette behavior and the
// Ctx.OpenList convenience. Open shows an arbitrary item list with a pick
// callback; OpenCommandPalette opens the built-in command palette view.
type Palette interface {
	Open(title string, items []ListItem, onPick func(ListItem))
	OpenCommandPalette()
}

// Clipboard copies/pastes text through the host's configured clipboard command
// (Settings.ClipboardCommand). It backs the kb-clip behavior.
type Clipboard interface {
	Copy(text string) error
	Paste() (string, error)
}

// Completion is the registry of completion sources keyed by trigger rune (e.g.
// '@' for files, '/' for slash commands). Components/plugins contribute a source
// at Init; Core queries the matching source as the user types. The built-in
// @file and /slash behaviors are modeled as two registered sources.
type Completion interface {
	// Register adds a completion source. Later registrations for the same trigger
	// take precedence (last-wins), consistent with the registry merge rule.
	Register(source CompletionSource)
	// Sources returns the registered sources (most-recent-first per trigger).
	Sources() []CompletionSource
}

// CompletionSource produces candidates for a single trigger rune. Trigger is the
// rune that activates the source ('@', '/'); Candidates is called with the query
// text typed after the trigger and returns the ranked completions.
type CompletionSource interface {
	Trigger() rune
	Candidates(query string) []Candidate
}

// Candidate is a single completion result. Value is inserted on accept; Detail
// is shown alongside it in the dropdown.
type Candidate struct {
	Value  string
	Detail string
}

// Selection is the host-level glue between a component's selection and the
// clipboard: Current returns the active selection text (editor selection, else
// the focused component's SelectionProvider), and Copy copies it. Components
// that own selection still implement the optional SelectionProvider/MouseHandler
// interfaces — this service is only for host-level selection→clipboard glue.
type Selection interface {
	Current() string
	Copy() error
}

// Settings is the resolved, merged runtime configuration handed to the kernel
// and plugins. It is produced by merging the external config's Opts over the
// built-in Default Config's Opts (see github.com/Ceinl/plums/config). Authors do
// not construct this directly; they declare config.Opts.
type Settings struct {
	Backend           string
	Layout            Layout
	DefaultLayout     string
	Mode              string
	Theme             Theme
	Keybinds          []Keybind
	HideThinking      bool
	SplitLeftWidth    int
	OutputPercent     int
	ClipboardCommand  string
	ClearHistory      bool
	OpencodeServerURL string
	Disable           []RegistryKey
}

// Keybind maps a key chord to a command name. The keymap is data the user owns;
// plugins register commands, not bindings.
type Keybind struct {
	Key string
	Do  string
}

// Theme is the token set read by components through RenderCtx.Theme().
type Theme struct {
	Name string
}

// Optional capability interfaces — a Plugin (or config) may implement any of
// these; the kernel discovers them by type assertion.

type CommandProvider interface {
	Commands() []Command
}

type ComponentProvider interface {
	Components() []Component
}

type LayoutProvider interface {
	Layouts() []Layout
}

type BackendProvider interface {
	Backends() []BackendRegistration
}

type OnMessage interface {
	OnMessage(Ctx, Message)
}

type OnSessionStart interface {
	OnSessionStart(Ctx, Session)
}

type OnToolCall interface {
	OnToolCall(Ctx, ToolCall)
}

// --- Commands & runtime context ---

type Command struct {
	Name   string
	Detail string
	Do     func(context.Context, Ctx) error
	// Title, when set, supplies the palette row's title/detail dynamically from
	// host state (e.g. "Switch to build mode", "Current mode: plan"). It replaces
	// the static Name/Detail in the command palette when present. Returning a
	// disabled item hides the row from selection. Slash-command dropdowns always
	// use Name; Title only affects the palette.
	Title func(CommandState) PaletteLabel
}

// PaletteLabel is a command's rendered palette row. Title/Detail are the visible
// text; Disabled greys the row out; Adjust marks the row as a left/right
// adjuster (the output-percent slider) and Step is its increment.
type PaletteLabel struct {
	Title    string
	Detail   string
	Disabled bool
	Adjust   bool
	Step     int
}

// Ctx is the runtime surface exposed to commands and hooks.
type Ctx interface {
	Session() Session
	Input() Editor
	Selection() string
	Send(text string)
	Chat(role, text string)
	Copy(text string)
	Shell(context.Context, string, ...string) (string, error)
	SetLayout(name string)
	OpenList(title string, items []ListItem, onPick func(ListItem))
	// Services exposes the host capability services for runtime invocation. Copy
	// and OpenList above are conveniences that delegate to Services().Clipboard()
	// and Services().Palette() respectively.
	Services() Services

	// Command verbs — the built-in command set drives the host through these. Each
	// enqueues the same effect the host would run for the corresponding action
	// (open a picker, toggle a setting, start a session). They wrap existing host
	// behavior; backends are not yet refactored into capabilities.
	OpenCommandPalette()
	ChangeModel()
	SwitchBackend()
	NewSession()
	OpenSessions()
	OpenSkills()
	SwitchMode()
	CycleThinkingVisibility()
	CycleToolCallVisibility()
	AdjustOutputPercent(delta int)
	SwitchLayout()

	// State reads commands use to render dynamic titles (mode/layout/visibility
	// labels, output percent, active backend).
	State() CommandState
}

// CommandState is the read-only snapshot of host state a command consults to
// render dynamic palette labels. It mirrors the template fields the legacy
// commands.json titles used ({mode}, {layout}, {thinking_visibility}, ...).
type CommandState struct {
	Mode               string
	Layout             string
	ThinkingVisibility string
	ToolCallVisibility string
	OutputPercent      int
	BackendProvider    string
}

type Editor interface {
	Text() string
	SetText(string)
}

type ListItem struct {
	ID      string
	Label   string
	Detail  string
	Payload any
}

// --- Components, layouts, rendering ---

type Rect struct {
	X int
	Y int
	W int
	H int
}

// Surface is the public drawing target handed to a component's Render. It is the
// minimal cell-based primitive set every built-in needs; the kernel's terminal
// screen satisfies it. Colors/decor are passed as strings (ANSI/theme tokens).
type Surface interface {
	Width() int
	Height() int
	Set(x, y int, ch rune, fg, bg, decor string)
	SetCursor(x, y int)
	ShowCursor()
}

type Component interface {
	Name() string
	Arrange(Rect)
	Render(RenderCtx, Surface)
}

// ComponentInstancer lets a registered component act as a template that produces
// a fresh instance per layout slot. The kernel caches one instance per component
// name for the life of a build, so each instance can own private render/input
// state (cursor, scroll, selection, measured-line caches) that survives rebuilds.
// A component that needs no private state can omit this and be used directly.
type ComponentInstancer interface {
	NewComponent() Component
}

type KeyHandler interface {
	HandleKey(Ctx, KeyEvent) bool
}

type MouseHandler interface {
	HandleMouse(Ctx, MouseEvent) bool
}

type SelectionProvider interface {
	Selection() string
}

// Scrollable is implemented by a component that owns a scrollable body. The host
// routes scroll input (keys, wheel) to it so scroll state lives on the component,
// not in central app state. Scroll reports whether the offset actually changed.
type Scrollable interface {
	Scroll(delta int) bool
	ScrollToBottom() bool
}

type DirtyTracker interface {
	IsDirty() bool
	ClearDirty()
}

type Layout interface {
	Name() string
	Tree() Node
}

type LayoutVisibility interface {
	Selectable() bool
}

type Node interface{}

type RenderCtx interface {
	Rect() Rect
	Theme() Theme
	Background() string
	Messages() []Message
	Streaming() bool
	StreamingText() string
	ThinkingVisibility() int
	ToolCallVisibility() int
	Session() Session
	Input() EditorView

	// Server/runtime status, read by the status bar and separator panes.
	ServerStarting() bool
	ServerReady() bool
	Mode() string
	SpinnerFrame() int

	// Sessions pane state.
	Sessions() []SessionItem
	SelectedSession() string

	// Info pane state (chat vs git diff tabs).
	InfoView() string
	InfoTabs() []InfoTabItem
	GitDiff() string

	// Split geometry percentages.
	SplitLeftPercent() int
	OutputPercent() int

	// Command palette panel state.
	PaletteTitle() string
	PaletteQuery() string
	PaletteItems() []PaletteItem
	PaletteIndex() int
}

// SessionItem is the read-only view of a session row handed to the sessions
// pane through RenderCtx.
type SessionItem struct {
	ID        string
	Title     string
	Directory string
	Updated   int64
	Current   bool
}

// InfoTabItem is one tab in the info pane header (e.g. "AI output", "Git diff").
type InfoTabItem struct {
	Label  string
	Active bool
}

// PaletteItem is one row in the command palette panel.
type PaletteItem struct {
	Title    string
	Detail   string
	Disabled bool
}

type EditorView interface {
	Text() string
}

type KeyEvent struct {
	Key   string
	Rune  rune
	Text  string
	Ctrl  bool
	Alt   bool
	Shift bool
	Cmd   bool
}

type MouseButton int

const (
	MouseButtonNone MouseButton = iota
	MouseButtonLeft
	MouseButtonMiddle
	MouseButtonRight
)

type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseDrag
	MouseMove
	MouseWheel
)

type MouseEvent struct {
	X      int
	Y      int
	Button MouseButton
	Action MouseAction
	Delta  int
}

// --- Registry ---

type RegistryKind string

const (
	RegistryCommand   RegistryKind = "command"
	RegistryComponent RegistryKind = "component"
	RegistryLayout    RegistryKind = "layout"
	RegistryBackend   RegistryKind = "backend"
	RegistryHook      RegistryKind = "hook"
)

type RegistryKey struct {
	Kind RegistryKind
	Name string
}

// --- Backends & domain model ---

type Backend interface {
	Health(ctx context.Context) error
	CreateSession(ctx context.Context, dir string) (*Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	GetSession(ctx context.Context, id string) (*Session, error)
	ListMessages(ctx context.Context, id string) ([]MessageResponse, error)
	ListProviders(ctx context.Context) ([]Provider, []string, error)
	SendMessageEvents(ctx context.Context, id, text, providerID, modelID, agent string) <-chan StreamEvent
	ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error
}

type BackendRegistration struct {
	Name    string
	Label   string
	Backend Backend
	Startup func(context.Context, Backend) (*StartupResult, error)
}

type StartupResult struct {
	Session *Session
	Server  ServerProcess
}

type ServerProcess interface {
	Stop()
	Done() <-chan struct{}
}

type Session struct {
	ID        string
	Title     string
	Directory string
	Model     *ModelRef
	Time      SessionTime
}

type SessionTime struct {
	Created int64
	Updated int64
}

type ModelRef struct {
	ID         string
	ProviderID string
	Variant    string
}

type MessageModelRef struct {
	ProviderID string
	ModelID    string
}

type Provider struct {
	ID     string
	Name   string
	Models map[string]Model
}

type Model struct {
	ID         string
	ProviderID string
	Name       string
	Status     string
}

type Message struct {
	Role    string
	Content string
}

type Part struct {
	Type string
	Text string
	Tool *ToolCall
}

type StreamEvent struct {
	Source   string
	Text     string
	Tool     *ToolCall
	Question *QuestionRequest
}

type ToolCall struct {
	ID     string
	Name   string
	Input  string
	Output string
	Error  string
}

type QuestionOption struct {
	Label       string
	Description string
}

type QuestionInfo struct {
	Question string
	Header   string
	Options  []QuestionOption
	Multiple bool
	Custom   *bool
}

type QuestionRequest struct {
	ID        string
	SessionID string
	Questions []QuestionInfo
}

type MessageInfo struct {
	ID   string
	Role string
}

type MessageResponse struct {
	Info  MessageInfo
	Parts []Part
}
