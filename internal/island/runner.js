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
