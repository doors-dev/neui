// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package neui

import (
	"context"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance"
)

// Enable turns neui on for the doors session ctx belongs to, typically from an
// auth hook or handler once the user is known; ctx must be a doors context of
// that session. Every tab of the session, open now or later, links its chat to
// it. Once enabled, later calls do nothing unless OptionForce is passed.
//
// The session's provider is the one OptionProvider sets, else the app's
// WithProvider; when there is neither, Enable returns api.ErrNoProvider and
// leaves the session as it was.
//
// Without a message or file storage set, threads are kept in memory and files
// in a per-session temporary directory, both lost when the session ends.
//
// Enable creates a per-session temporary directory for the agent's workspace
// and returns an error when it cannot; a later call tries again.
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

// WithProvider sets the app-wide provider, as an app option for doors.NewApp.
// It is optional: a session enabled with OptionProvider runs on that
// provider instead, and Enable returns api.ErrNoProvider when neither is set.
func WithProvider(p api.Provider) doors.With {
	return doors.WithValue(common.ProviderKey{}, p)
}

// OptionProvider sets the session's provider, taking precedence over the
// app's WithProvider if there is one. A nil p leaves the session without a
// provider, and Enable returns api.ErrNoProvider.
func OptionProvider(p api.Provider) Option {
	return common.OptionProvider(p)
}

// WithThreadConfig sets the configuration every session the app serves runs
// on, as an app option for doors.NewApp.
func WithThreadConfig(c api.ThreadConfig) doors.With {
	return doors.WithValue(common.ConfigKey{}, common.ConfigDefaults(c))
}

// WithFilesConfig sets the workspace filesystem and files module limits of
// every session the app serves, as an app option for doors.NewApp; without
// it every limit takes its default. OptionFilesConfig takes precedence for
// one session.
func WithFilesConfig(c api.FilesConfig) doors.With {
	return doors.WithValue(common.FilesConfigKey{}, common.FilesConfigDefaults(c))
}

// WithIslandConfig sets the limits of the scripts that draw into islands, as
// an app option for doors.NewApp.
func WithIslandConfig(c api.IslandConfig) doors.With {
	return doors.WithValue(common.IslandConfigKey{}, common.IslandConfigDefaults(c))
}

// OptionFilesConfig sets the session's workspace filesystem and files
// module limits, taking precedence over the app's WithFilesConfig if there
// is one. It is not merged with the app's config: a zero field takes the
// built-in default.
func OptionFilesConfig(c api.FilesConfig) Option {
	return common.OptionFilesConfig(c)
}

// OptionMessageStorage sets where the session's threads, rounds and records
// are stored, replacing the in-memory default that is lost with the session.
func OptionMessageStorage(s api.MessageStorage) Option {
	return common.OptionMessageStorage(s)
}

// OptionUploadStorage sets where the session's uploaded files are stored, both
// the user's (api.UserUploadsDir) and the agent's (api.AgentUploadsDir),
// overriding the app's WithUploadStorage and the default temporary directory.
func OptionUploadStorage(s api.UploadStorage) Option {
	return common.OptionUploadStorage(s)
}

// WithUploadStorage sets where every session's uploaded files are stored, as an
// app option for doors.NewApp, replacing the per-session temporary directory.
// OptionUploadStorage overrides it for one session.
func WithUploadStorage(s api.UploadStorage) doors.With {
	return doors.WithValue(common.UploadStorageKey{}, s)
}

// WithSystemPrompt adds text to the system prompt of every session the app
// serves, as an app option for doors.NewApp. Its parts are joined by blank
// lines after neui's own runtime note and before the modules' guidance.
// OptionSystemPrompt adds more after it for one session.
func WithSystemPrompt(prompt ...string) doors.With {
	return doors.WithValue(common.SystemKey{}, prompt)
}

// OptionSystemPrompt appends to the app's system prompt for one session.
func OptionSystemPrompt(prompt ...string) Option {
	return common.OptionSystem(prompt)
}

// OptionForce makes Enable rebuild the configuration of a session that is
// already enabled, instead of doing nothing. The new configuration starts
// from the app-wide values again, so options passed to an earlier Enable do
// not carry over. Every open tab then relinks its chat to it.
func OptionForce() Option {
	return common.OptionForce()
}
