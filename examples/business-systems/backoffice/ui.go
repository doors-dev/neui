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

package main

import (
	"context"
	"neui.dev/nekit"
	"strings"

	"github.com/doors-dev/doors"
)

const btnBase = "inline-flex shrink-0 cursor-pointer items-center justify-center gap-1.5 rounded-lg px-3.5 py-2 text-sm font-medium whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-50 aria-busy:cursor-wait aria-busy:opacity-70"

var (
	btnPrimary    = doors.Class(btnBase, "bg-stone-900 text-white shadow-xs hover:bg-stone-700 focus-visible:outline-stone-900")
	btnSecondary  = doors.Class(btnBase, "bg-white text-stone-800 shadow-xs ring-1 ring-stone-300 ring-inset hover:bg-stone-50 focus-visible:outline-stone-400")
	btnDanger     = doors.Class(btnBase, "bg-red-600 text-white shadow-xs hover:bg-red-500 focus-visible:outline-red-600")
	btnDangerSoft = doors.Class(btnBase, "bg-white text-red-700 shadow-xs ring-1 ring-red-200 ring-inset hover:bg-red-50 focus-visible:outline-red-500")
	btnGhost      = doors.Class(btnBase, "text-stone-600 hover:bg-stone-100 hover:text-stone-900 focus-visible:outline-stone-400")
	linkText      = doors.Class("font-medium text-stone-900 underline decoration-stone-300 underline-offset-4 hover:decoration-amber-500")
	inputBase     = doors.Class("block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-stone-900 shadow-xs ring-1 ring-stone-300 ring-inset placeholder:text-stone-400 focus:ring-2 focus:ring-amber-500 focus:outline-none aria-invalid:ring-red-400 aria-invalid:focus:ring-red-500")
	tableClass    = doors.Class("min-w-full text-sm")
	thClass       = doors.Class("px-3 py-2.5 @xl:px-4 text-left text-xs font-semibold tracking-wide text-stone-500 uppercase first:pl-5 last:pr-5")
	tdClass       = doors.Class("px-3 py-3 @xl:px-4 whitespace-nowrap first:pl-5 last:pr-5")
	rowClass      = doors.Class("border-t border-stone-100 transition-colors hover:bg-amber-50/40")
)

const (
	iconDashboard = "M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z"
	iconOrders    = "M15.75 10.5V6a3.75 3.75 0 1 0-7.5 0v4.5m11.356-1.993 1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 0 1-1.12-1.243l1.264-12A1.125 1.125 0 0 1 5.513 7.5h12.974c.576 0 1.059.435 1.119 1.007ZM8.625 10.5a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm7.5 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z"
	iconCustomers = "M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z"
	iconProducts  = "m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"
	iconSearch    = "m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
	iconLeft      = "M15.75 19.5 8.25 12l7.5-7.5"
	iconRight     = "m8.25 4.5 7.5 7.5-7.5 7.5"
	iconUp        = "m4.5 15.75 7.5-7.5 7.5 7.5"
	iconDown      = "m19.5 8.25-7.5 7.5-7.5-7.5"
	iconUpDown    = "M8.25 15 12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9"
	iconPlus      = "M12 4.5v15m7.5-7.5h-15"
	iconCheck     = "M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
	iconWarning   = "M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
	iconTrash     = "m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"
	iconEdit      = "m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10"
	iconClose     = "M6 18 18 6M6 6l12 12"
	iconMinus     = "M5 12h14"
	iconBack      = "M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
	iconCup       = "M4.5 9h11.25v4.5a5.25 5.25 0 0 1-5.25 5.25h-.75A5.25 5.25 0 0 1 4.5 13.5V9Zm11.25 1.5h1.5a2.625 2.625 0 0 1 0 5.25h-1.5M8.25 3v2.25M12 3v2.25"
	iconMoney     = "M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
	iconClock     = "M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
	iconSparkles  = "M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09Z"
)

const pageSize = 10

type pageInfo struct {
	Page  int `json:"page"`
	Pages int `json:"pages"`
	From  int `json:"from" description:"1-based index of the first row shown"`
	To    int `json:"to" description:"1-based index of the last row shown"`
	Total int `json:"total" description:"rows matching the filter"`
}

func paginate[T any](rows []T, page int, size int) ([]T, pageInfo) {
	info := pageInfo{Total: len(rows), Pages: max(1, (len(rows)+size-1)/size)}
	info.Page = min(max(page, 1), info.Pages)
	start := (info.Page - 1) * size
	end := min(start+size, len(rows))
	if start >= end {
		return nil, info
	}
	info.From, info.To = start+1, end
	return rows[start:end], info
}

func nextSort(current string, key string, first string) string {
	if current == key {
		return "-" + key
	}
	if current == "-"+key {
		return key
	}
	return first
}

type noticeKey struct{}

type notice struct {
	ID   int
	Text string
}

func notices(ctx context.Context) doors.Source[notice] {
	return doors.InstanceStore(ctx).Init(noticeKey{}, func() any {
		return doors.NewSource(notice{})
	}).(doors.Source[notice])
}

func notify(ctx context.Context, text string) {
	notices(ctx).Mutate(ctx, func(n notice) notice {
		return notice{ID: n.ID + 1, Text: text}
	})
}

var scrollTop = doors.ActionScroll{Selector: "#top"}

func when(ok bool, class string) string {
	if ok {
		return class
	}
	return ""
}

func opt(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func ariaTrue(ok bool) any {
	if ok {
		return "true"
	}
	return nil
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	out := parts[0][:1]
	if len(parts) > 1 {
		out += parts[len(parts)-1][:1]
	}
	return strings.ToUpper(out)
}

var chatTheme = nekit.ChatTheme{
	AppBg:            "#ffffff",
	AppBorder:        "#d6d3d1",
	AppInk:           "#1c1917",
	AppInkQuiet:      "#78716c",
	PanelBg:          "#fafaf9",
	PanelBorderFocus: "#d6d3d1",
	ButtonBorder:     "#e7e5e4",
	ButtonInk:        "#44403c",
	ButtonInkOff:     "#a8a29e",
	SecondaryBg:      "#e7e5e4",
	DangerInk:        "#dc2626",
	DangerBgHover:    "#dc2626",
	DangerBgActive:   "#b91c1c",
	DangerInkOnFill:  "#ffffff",
	LauncherBg:       "#1c1917",
	LauncherInk:      "#ffffff",
}
