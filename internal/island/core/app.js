const app = (() => {
	const fetches = __fetches
	delete globalThis.__fetches;
	const app = {}
	for (const name of fetches()) {
		const globalName = "__fetch__" + name
		app[name] = globalThis[globalName]
		delete globalThis[globalName]
	}
	return app
})()
Object.freeze(app)
