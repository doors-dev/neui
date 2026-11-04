// Package api holds the contracts shared by hosts, providers and storages.
//
// The protocol types (Request, Message, the Content blocks and the Event
// stream a Provider emits) are what a provider adapter implements against.
// The storage interfaces (MessageStorage, FileStorage, WorkspaceStorage and the
// FS a tool works in) are what a host implements to keep threads and files
// somewhere other than the defaults. Instance and the Chat* types are the
// session-side view a chat UI renders from, and ThreadConfig, FilesConfig and
// IslandConfig are the tunables passed to the root package's With* options.
// The errors here are the sentinels callers compare with errors.Is.
package api
