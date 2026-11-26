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

const ui = (() => {
	const release = __release
	const create = __create
	const update = __update
	const createSlot = __createSlot
	const releaseSlot = __releaseSlot
	const setSlot = __setSlot
	const widgets = __widgets
	const layouts = __layouts
	const validateLayout = __validateLayout
	const draw = __draw
	const notify = __notify
	const end = __end
	const fallback = __fallback
	delete globalThis.__create;
	delete globalThis.__update;
	delete globalThis.__release;
	delete globalThis.__createSlot
	delete globalThis.__releaseSlot
	delete globalThis.__setSlot
	delete globalThis.__widgets
	delete globalThis.__layouts
	delete globalThis.__validateLayout
	delete globalThis.__draw
	delete globalThis.__notify
	delete globalThis.__end
	delete globalThis.__fallback
	const flatten = (content) => content.flat(Infinity).filter(c => c !== null && c !== undefined && c !== false)
	const Widget = class {
		static #reg = new FinalizationRegistry(id => release(id));
		#id;
		constructor(name, params) {
			this.#id = create({ name, params });
			Widget.#reg.register(this, this.#id);
		}
		update(patch) {
			if (patch === undefined) {
				throw new TypeError("update needs a patch object that matches the view's or control's patch schema")
			}
			update({ id: this.#id, patch });
		}
		get id() {
			return this.#id
		}
	}
	const Slot = class {
		static #reg = new FinalizationRegistry(id => releaseSlot(id));
		#id;
		constructor(root) {
			if (root) {
				this.#id = 0
			} else {
				this.#id = createSlot();
				Slot.#reg.register(this, this.#id);
			}
		}
		#buildContent(content) {
			return content.map(c => {
				if (c instanceof Slot) {
					return { kind: "slot", id: c.#id }
				}
				if (c instanceof Widget) {
					return { kind: "widget", id: c.id }
				}
				if (c instanceof Layout) {
					return { kind: "layout", name: c.name, params: c.params, content: this.#buildContent(c.children) }
				}
				throw new TypeError("slot content must be a view, control, layout or slot")
			})
		}
		set(...content) {
			content = flatten(content)
			if (content.length > 1) {
				throw new TypeError("a slot holds one item: put several in a layout")
			}
			setSlot({ id: this.#id, content: this.#buildContent(content) })
		}
	}
	const Layout = class {
		#name
		#params
		#children
		constructor(name, params, children) {
			this.#name = name
			this.#params = params
			this.#children = flatten(children)
		}
		get name() {
			return this.#name
		}
		get params() {
			return this.#params
		}
		get children() {
			return this.#children
		}
	}
	const ui = {}
	for (const name of widgets()) {
		ui[name] = (params) => {
			return new Widget(name, params ?? {})
		}
	}
	for (const name of layouts()) {
		ui[name] = (params, ...children) => {
			validateLayout({ name, params: params ?? {} })
			return new Layout(name, params ?? {}, children)
		}
	}
	ui.slot = () => new Slot()
	ui.draw = (ms) => {
		const result = draw(ms ?? -1)
		if (result?.interrupt) {
			while (true) { }
		}
		if (result === null) {
			return result
		}
		return new Proxy(result, {
			get(target, key, receiver) {
				const output = target.output
				if (typeof key !== "string" || key in target || output === null || typeof output !== "object" || Array.isArray(output) || !Object.hasOwn(output, key)) {
					return Reflect.get(target, key, receiver)
				}
				fallback(key)
				return output[key]
			}
		})
	}
	ui.root = new Slot(true)
	ui.notify = notify
	ui.end = () => {
		end()
		while (true) { }
	}
	return ui
})()
Object.freeze(ui)
