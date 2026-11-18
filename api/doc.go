// Package api holds the contracts shared by hosts, providers and storages.
//
// The protocol types (Request, Message, the Content blocks and the Event
// stream a Provider emits) are what a provider adapter implements against.
// MessageStorage and UploadStorage are what a host implements to keep threads
// and uploaded files somewhere other than the defaults; WorkspaceStorage and
// FS describe the workspace neui gives each thread, and FS is what a tool
// works in. Instance and the Chat* types are the session-side view a chat UI
// renders from. ThreadConfig, FilesConfig and IslandConfig are the tunables
// for the root package's WithThreadConfig, WithFilesConfig (or the per-session
// OptionFilesConfig) and WithIslandConfig.
// The errors here are the sentinels callers compare with errors.Is.
package api
