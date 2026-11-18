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
