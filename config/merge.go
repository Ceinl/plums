package config

// MergeOpts overlays the external Opts over the base (Default Config) Opts,
// last-wins, field by field. A field counts as "set" only when its sentinel
// tri-state says so (Pref.Set, Bool.Set, Count.Set) or, for plain slices/ints,
// when it is non-empty/non-zero. Unset fields fall through to the base.
//
// The result is itself an Opts (still expressed in sentinel types) so the
// resolved value of every field is unambiguous and callers can re-merge.
func MergeOpts(base, over Opts) Opts {
	out := base

	if over.Backend.Set() {
		out.Backend = over.Backend
	}
	if over.Model.Set() {
		out.Model = over.Model
	}
	if over.DefaultLayout.Set() {
		out.DefaultLayout = over.DefaultLayout
	}
	if over.Mode.Set() {
		out.Mode = over.Mode
	}
	if over.OpencodeServerURL.Set() {
		out.OpencodeServerURL = over.OpencodeServerURL
	}

	if over.Theme != (Theme{}) {
		out.Theme = over.Theme
	}
	if len(over.Keybinds) > 0 {
		out.Keybinds = over.Keybinds
	}
	if over.ClipboardCommand != "" {
		out.ClipboardCommand = over.ClipboardCommand
	}

	if over.HideThinking.Set() {
		out.HideThinking = over.HideThinking
	}
	if over.ThinkingVisibility != 0 {
		out.ThinkingVisibility = over.ThinkingVisibility
	}
	if over.ToolCallVisibility != 0 {
		out.ToolCallVisibility = over.ToolCallVisibility
	}

	if over.SplitLeftWidth.Set() {
		out.SplitLeftWidth = over.SplitLeftWidth
	}
	if over.OutputPercent.Set() {
		out.OutputPercent = over.OutputPercent
	}

	if over.ClearHistory.Set() {
		out.ClearHistory = over.ClearHistory
	}

	if len(over.Disable) > 0 {
		out.Disable = append(append([]RegistryKey(nil), base.Disable...), over.Disable...)
	}

	if over.HealthTimeoutMS != 0 {
		out.HealthTimeoutMS = over.HealthTimeoutMS
	}
	if over.QuestionReplyTimeoutMS != 0 {
		out.QuestionReplyTimeoutMS = over.QuestionReplyTimeoutMS
	}
	if over.RecentModelTimeoutMS != 0 {
		out.RecentModelTimeoutMS = over.RecentModelTimeoutMS
	}
	if over.ListTimeoutMS != 0 {
		out.ListTimeoutMS = over.ListTimeoutMS
	}
	if over.SpinnerIntervalMS != 0 {
		out.SpinnerIntervalMS = over.SpinnerIntervalMS
	}

	return out
}

// Merge overlays the external Config over the base (Default Config): Opts merge
// field-by-field (MergeOpts) and Plugins concatenate base-then-external so the
// external plugins load later and win on name collisions (last-wins).
func Merge(base, over Config) Config {
	plugins := make([]Plugin, 0, len(base.Plugins)+len(over.Plugins))
	plugins = append(plugins, base.Plugins...)
	plugins = append(plugins, over.Plugins...)
	return Config{
		Opts:    MergeOpts(base.Opts, over.Opts),
		Plugins: plugins,
	}
}
