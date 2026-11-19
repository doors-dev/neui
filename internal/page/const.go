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

package page

import "errors"

const moduleName = "page"
const moduleDesc = `Your access to the live page the user has open: use it to help the user operate the application — navigate, fill inputs, press buttons, call the tools the page exposes, or focus an element to point the user at it. The page is a tree of spaces, UI regions carrying tools, inputs and child spaces. Only what the application chose to expose is there, so a missing element means unavailable to you, not absent from the page: say so instead of guessing. read, write, click, focus and tool need the space read first, with sync, by you or a script, and act only while it is as it was last read; otherwise they fail without acting. The user works the same page alongside you: a change you did not make, shown in a refusal's diff, in a call's diff or changes, or in a read that differs from what you or your script set, most often comes from them, so check what they did and build on it instead of redoing or undoing it. Every page call made as a tool call costs a round; for repetitive work — the same steps over several rows or items — write a script when the js module is available. In a script, a refused or failed page call throws, and its e.value holds what its error carries: diff, space, or the failed call's result. You are part of this page, and to the user you see what they see: reading it, and its spaces, paths, syncs, rerenders and refused calls, are your plumbing, invisible to them. Don't announce looking at the page or what you find on it, such as "I'll look at the page first" or "I see there are tools": read it and act. Report outcomes in page terms: what was filled, pressed or changed.`
const toolName = "tool"
const toolDesc = `Call a tool defined by the currently rendered page.`
const readName = "read"
const readDesc = `Read the state of a UI input on the currently rendered page.`
const writeName = "write"
const writeDesc = `Set the value of a UI input on the currently rendered page.`
const clickName = "click"
const clickDesc = `Click a UI input on the currently rendered page.`
const focusName = "focus"
const focusDesc = `Focus a UI input and scroll it into view, to point the user at it.`
const syncName = "sync"
const syncDesc = `Read one space of the currently rendered page whole.`
const treeName = "tree"
const treeDesc = `Show the spaces of the currently rendered page: names, descriptions and how many tools, inputs and data items each has, without their content. It does not count as reading them.`
const refusedUnreadMessage = "not done: this space has not been read; space shows it as it is now and counts as a read, so check it and call again"
const refusedChangedMessage = "not done: the space changed since it was last read; diff shows what changed and brings the read up to date, so check it and call again"
const refusedDuplicateMessage = "not done: more than one input in this space has this id, so no call can address any of them; use a tool of the space that does the same, or ask the user to do it on the page"
const refusedTooLargeMessage = "not done: this space has not been read as it is now, and it is too large to show in a result: act on it from a script, syncing it there first; without a script tool, tell the user this part of the page is out of your reach"
const spaceTooLargeMessage = "the space is too large for a result: sync it and act on it from a script, where results have no size limit"
const islandRevisionMessage = "the island's spec changed since that revision: read island_spec again"
const toolNotFoundMessage = "no such tool in the space: sync its path to see the current tools"
const spaceNotFoundMessage = "no such space: it was removed or renamed since you saw its path; call tree to see the current spaces"
const rightsMessage = "the input's rights do not allow this call: sync reports each input's rights, use a call they permit"
const inputNotFoundMessage = "no such input in the space: sync its path to see the current inputs"
const islandSpecName = "island_spec"
const islandSpecDesc = `Read an island's spec: how to write its app, a catalog of the views, controls, layouts and app functions the app can use, the tools its script has, and where its logs and files go; island_spec_schema gives the schemas of the items you use. It seldom changes: reuse what you read until island_begin refuses its revision. Not accessible from scripts.`
const islandNotFoundMessage = "this space is not an island: call tree to see which spaces are islands"
const islandSpecPageMessage = "no such page of the spec"
const islandSpecItemMessage = "one item of the spec is larger than a page can hold, so the spec cannot be read and no app can start in this island"
const islandSpecSchemaName = "island_spec_schema"
const islandSpecSchemaDesc = `Read the schemas of items from an island's spec catalog: params, patch and output of views and controls, params of layouts, input and result of app functions, with the definitions they refer to. Read the ones your app uses before writing it. Islands often share items: one with the same name and description as in another island's catalog is most likely the same, so reuse the schema you already read. Not accessible from scripts.`
const islandSchemaSizeMessage = "the schemas of these items are larger than a result can hold: ask for fewer items at a time"
const islandBeginName = "island_begin"
const islandBeginDesc = `Begin a new app in an island with the first part of its script. An app already running there is replaced: its task ends, its queued parts and the values it notified since its last ui.draw() are dropped, and its screen stays until the new app's first ui.draw(). Returns once the part is queued, before it runs. Not accessible from scripts.`
const islandAppendName = "island_append"
const islandAppendDesc = `Append the next part to the script of an island's running app: it runs once the parts before it have ended. Not accessible from scripts.`
const islandInterruptName = "island_interrupt"
const islandInterruptDesc = `Send the next part to an island's running app to take over: the running part stops at its first ui.draw(), which still presents its frame, the parts queued behind it are dropped, and this part runs. Not accessible from scripts.`
const islandEndName = "island_end"
const islandEndDesc = `End an island's app now, whether a part is running or not: the island replaces the app's screen with its own, and the values a running part notified since its last ui.draw() are dropped. Not accessible from scripts.`
const fileOrCodeMessage = "set exactly one of file and code"
const islandFromJSMessage = "island tools are not accessible from a script"

const islandName = "island"
const islandsDesc = `Islands

An island is a space where you run your own app for the user, built and driven by a script you write. Its description usually says what its apps can work with and do. To the user, an island is part of the page: tell them what its app shows or asks, not how its script, parts and task work, unless they ask.

Open one when it serves the user better than chat or direct page calls do:
- Automation: repetitive or multi-step work in the app that you can drive for them, with the user confirming or choosing along the way instead of being asked in chat at every step.
- Complex input: several fields, a choice from options, a confirmation, or a structured answer that would be clumsy to collect in prose.
- A custom view: a table, summary, comparison or overview built from app data that the app itself doesn't show.

Don't open one when a script alone does the job and you have the js module; for example, when bulk work on the page needs nothing from the user, such as the same steps over many rows.

An island's app functions are also tools of its app child space: for a single change, call one directly instead of starting an app, or call them from a js script when you have that module and the task needs no UI.

Before starting an app in an island, read its spec with island_spec: it teaches how to write the app and lists the views, controls, layouts, app functions and tools the app can use; use only what it lists, and read the schemas of the items you use with island_spec_schema.`

var errRefusedDuplicate = errors.New(refusedDuplicateMessage)
var errRefusedTooLarge = errors.New(refusedTooLargeMessage)
var errSpaceTooLarge = errors.New(spaceTooLargeMessage)
var errIslandRevision = errors.New(islandRevisionMessage)
var errToolNotFound = errors.New(toolNotFoundMessage)
var errSpaceNotFound = errors.New(spaceNotFoundMessage)
var errInputNotFound = errors.New(inputNotFoundMessage)
var errRights = errors.New(rightsMessage)
var errIslandNotFound = errors.New(islandNotFoundMessage)
var errIslandSpecPage = errors.New(islandSpecPageMessage)
var errIslandSpecItem = errors.New(islandSpecItemMessage)
var errIslandSchemaSize = errors.New(islandSchemaSizeMessage)
var errFileOrCode = errors.New(fileOrCodeMessage)
var errIslandFromJS = errors.New(islandFromJSMessage)
