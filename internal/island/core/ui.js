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
	const flatten = (content) => content.flat(Infinity).filter(c => c !== null && c !== undefined && c !== false)
	const Widget = class {
		static #reg = new FinalizationRegistry(id => release(id));
		#id;
		constructor(name, params) {
			this.#id = create({ name, params });
			Widget.#reg.register(this, this.#id);
		}
		update(patch) {
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
				throw new TypeError("slot content must be a widget, layout or slot")
			})
		}
		set(...content) {
			setSlot({ id: this.#id, content: this.#buildContent(flatten(content)) })
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
		return result
	}
	ui.root = new Slot(true)
	ui.notify = notify
	return ui
})()
Object.freeze(ui)
