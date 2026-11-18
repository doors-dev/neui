// Package nekit is the ready-made chat widget for neui. Render it on a doors
// page:
//
//	~nekit.Chat{Disable: nekit.ChatSimple}
//
// It shows a disabled chat until neui.Enable succeeds for the page's session,
// and again after neui.Disable. Chat.Disable hides parts of the UI, with
// ChatSimple as the preset for non-technical users, and ThemeDark and
// ThemeLight recolour it. ModuleAsk adds a tool that lets the agent ask the
// user to pick from a list inside the chat.
//
// Package nekit is the ready-made widget set for neui islands: layouts,
// markdown, forms, tables and buttons. Register them on Island.Library:
//
//	Library: nekit.IslandRegisterAll
//
// NewSQLiteMessages keeps the threads of all users in one SQLite database that the
// host opens with a driver of its choice, and User gives each session its
// user's part:
//
//	import _ "modernc.org/sqlite"
//
//	db, err := sql.Open("sqlite", "neui.db")
//	store, err := nekit.NewSQLiteMessages(db)
//	neui.Enable(ctx, neui.OptionMessageStorage(store.User(userID)))
package nekit
