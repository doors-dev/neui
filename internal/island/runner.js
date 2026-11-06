const __run = (() => {
	const stringify = JSON.stringify
	return (f, arg) => {
		if (typeof f !== "function") {
			throw new TypeError("the source is not a function")
		}
		return stringify(f(arg) ?? null)
	}
})();
(() => {
	const seen = new Set()
	const freeze = (o) => {
		if (o === null || (typeof o !== "object" && typeof o !== "function") || seen.has(o)) {
			return
		}
		seen.add(o)
		Object.freeze(o)
		for (const key of Reflect.ownKeys(o)) {
			const d = Object.getOwnPropertyDescriptor(o, key)
			freeze(d.value)
			freeze(d.get)
			freeze(d.set)
		}
		freeze(Object.getPrototypeOf(o))
	}
	freeze(globalThis)
	freeze(app)
	freeze(function* () { })
	freeze(async function () { })
	freeze(async function* () { })
	freeze([][Symbol.iterator]())
	freeze(new Map()[Symbol.iterator]())
	freeze(new Set()[Symbol.iterator]())
	freeze(""[Symbol.iterator]())
	freeze(/a/[Symbol.matchAll](""))
})()
