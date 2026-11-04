package neui

import (
	"context"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance"
)

// Enable turns neui on for the session. It runs on the app's WithProvider
// unless OptionProvider sets another, and returns api.ErrNoProvider when
// neither does. Once enabled, later calls do nothing unless OptionForce is
// passed.
func Enable(ctx context.Context, opts ...Option) error {
	return instance.GetSession(ctx).Enable(opts)
}

func Disable(ctx context.Context) {
	sess := instance.GetSession(ctx)
	sess.Disable()
}

func Instance(ctx context.Context) api.Instance {
	return instance.GetInstance(ctx)
}

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

func OptionMessageStorage(s api.MessageStorage) Option {
	return common.OptionMessageStorage(s)
}

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

func OptionForce() Option {
	return common.OptionForce()
}
