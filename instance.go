package neui

import (
	"context"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance"
)

// Enable turns neui on for the doors session ctx belongs to, typically from an
// auth hook or handler once the user is known, and every open tab of the
// session links its chat to it. It runs on the app's WithProvider unless
// OptionProvider sets another, and returns api.ErrNoProvider when neither
// does. Without OptionMessageStorage and OptionFileStorage, threads are kept
// in memory and files in a per-session temporary directory; neither is
// reachable after the session ends, and the directory is not removed. Once
// enabled, later calls do nothing unless OptionForce is passed.
func Enable(ctx context.Context, opts ...Option) error {
	return instance.GetSession(ctx).Enable(opts)
}

// Disable turns neui off for the doors session ctx belongs to: every tab's
// chat closes its thread and shows the disabled state. It does nothing when
// the session is not enabled; a later Enable turns it on again.
func Disable(ctx context.Context) {
	sess := instance.GetSession(ctx)
	sess.Disable()
}

// Instance returns the neui view of the doors instance (browser tab) ctx
// belongs to: its chat state, the session's open chats, and thread switching.
// A host uses it to build its own chat list or thread controls. ctx must come
// from a doors render or handler; it works before Enable, reporting
// api.InstanceDisabled.
func Instance(ctx context.Context) api.Instance {
	return instance.GetInstance(ctx)
}

// Option configures one session in Enable. Options apply in order on top of
// the app-wide With* values.
type Option = common.Option

// WithProvider sets the provider every session the app serves runs on, as an
// app option for doors.NewApp. OptionProvider replaces it for one session.
func WithProvider(p api.Provider) doors.With {
	return doors.WithValue(common.ProviderKey{}, p)
}

// OptionProvider replaces the app's provider for one session.
func OptionProvider(p api.Provider) Option {
	return common.OptionProvider(p)
}

// WithThreadConfig sets the configuration every session the app serves runs
// on, as an app option for doors.NewApp.
func WithThreadConfig(c api.ThreadConfig) doors.With {
	return doors.WithValue(common.ConfigKey{}, common.ConfigDefaults(c))
}

// WithFilesConfig sets the workspace filesystem and files module limits every
// session the app serves runs on, as an app option for doors.NewApp.
// OptionFilesConfig replaces them for one session.
func WithFilesConfig(c api.FilesConfig) doors.With {
	return doors.WithValue(common.FilesConfigKey{}, common.FilesConfigDefaults(c))
}

// WithIslandConfig sets the limits of the scripts that draw into islands, as
// an app option for doors.NewApp.
func WithIslandConfig(c api.IslandConfig) doors.With {
	return doors.WithValue(common.IslandConfigKey{}, common.IslandConfigDefaults(c))
}

// OptionFilesConfig replaces the app's workspace filesystem and files module
// limits for one session.
func OptionFilesConfig(c api.FilesConfig) Option {
	return common.OptionFilesConfig(c)
}

// OptionMessageStorage sets where the session's threads, rounds and records
// are stored, replacing the in-memory default that is lost with the session.
func OptionMessageStorage(s api.MessageStorage) Option {
	return common.OptionMessageStorage(s)
}

// OptionFileStorage sets where the files users upload into the session's
// chats are stored, replacing the default temporary directory. Thread
// workspaces stay in the temporary directory.
func OptionFileStorage(s api.FileStorage) Option {
	return common.OptionFileStorage(s)
}

// WithSystemPrompt sets the system prompt every session the app serves starts
// with, as an app option for doors.NewApp. OptionSystemPrompt appends to it.
func WithSystemPrompt(prompt ...string) doors.With {
	return doors.WithValue(common.SystemKey{}, prompt)
}

// OptionSystemPrompt appends to the app's system prompt for one session.
func OptionSystemPrompt(prompt ...string) Option {
	return common.OptionSystem(prompt)
}

// OptionForce makes Enable apply its options to a session that is already
// enabled, instead of doing nothing. Every open tab then relinks its chat to
// the new configuration; use it to switch provider or storage mid-session.
func OptionForce() Option {
	return common.OptionForce()
}
