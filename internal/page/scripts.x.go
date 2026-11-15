// Managed by GoX v0.4.0

//line scripts.gox:1
package page

import "github.com/doors-dev/gox"

//line scripts.gox:5
func element(id string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line scripts.gox:6
			__e = __c.Set("data:id", id); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const id = $data(\"id\")\n\t\t$on(id, async ({ action, value }) => {\n\t\t\tconst el = document.querySelector('[data-kid=\"' + id + '\"]')\n\t\t\tif (!el) {\n\t\t\t\tthrow \"element is not rendered\"\n\t\t\t}\n\t\t\tconst tag = el.tagName.toLowerCase()\n\t\t\tconst type = (el.getAttribute(\"type\") || \"\").toLowerCase()\n\t\t\tswitch (action) {\n\t\t\t\tcase \"click\":\n\t\t\t\t\treturn {\n\t\t\t\t\t\thooks: await $sys.emit(\n\t\t\t\t\t\t\tel,\n\t\t\t\t\t\t\tnew PointerEvent(\"click\", { bubbles: true }),\n\t\t\t\t\t\t),\n\t\t\t\t\t}\n\t\t\t\tcase \"focus\":\n\t\t\t\t\tel.focus({ preventScroll: true })\n\t\t\t\t\tel.scrollIntoView({ behavior: \"smooth\", block: \"nearest\" })\n\t\t\t\t\treturn {}\n\t\t\t\tcase \"read\": {\n\t\t\t\t\tconst label =\n\t\t\t\t\t\tel.labels && el.labels[0] ? el.labels[0].textContent.trim() : \"\"\n\t\t\t\t\tif (tag === \"a\") {\n\t\t\t\t\t\treturn { label: el.textContent.trim(), href: el.getAttribute(\"href\") }\n\t\t\t\t\t}\n\t\t\t\t\tconst disabled = el.disabled\n\t\t\t\t\tif (tag === \"button\") {\n\t\t\t\t\t\treturn {\n\t\t\t\t\t\t\t...(el.value ? { value: el.value } : {}),\n\t\t\t\t\t\t\tlabel: el.textContent.trim(),\n\t\t\t\t\t\t\tdisabled,\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\tif ([\"submit\", \"button\", \"reset\"].includes(type)) {\n\t\t\t\t\t\treturn { value: el.value, label: el.value, disabled }\n\t\t\t\t\t}\n\t\t\t\t\tif (type === \"image\") {\n\t\t\t\t\t\treturn { value: el.value, label: el.alt || \"\", disabled }\n\t\t\t\t\t}\n\t\t\t\t\tif (type === \"checkbox\") {\n\t\t\t\t\t\treturn { value: el.checked, label, disabled }\n\t\t\t\t\t}\n\t\t\t\t\tif (tag === \"select\") {\n\t\t\t\t\t\tconst options = Array.from(el.options).map((o) => ({\n\t\t\t\t\t\t\tvalue: o.value,\n\t\t\t\t\t\t\tlabel: o.label,\n\t\t\t\t\t\t\tselected: o.selected,\n\t\t\t\t\t\t\tdisabled: o.disabled,\n\t\t\t\t\t\t}))\n\t\t\t\t\t\tconst value = el.multiple\n\t\t\t\t\t\t\t? Array.from(el.selectedOptions).map((o) => o.value)\n\t\t\t\t\t\t\t: el.value\n\t\t\t\t\t\treturn { value, options, label, disabled }\n\t\t\t\t\t}\n\t\t\t\t\treturn { value: el.value, label, disabled }\n\t\t\t\t}\n\t\t\t\tcase \"input\": {\n\t\t\t\t\tif (\n\t\t\t\t\t\ttag === \"a\" ||\n\t\t\t\t\t\ttag === \"button\" ||\n\t\t\t\t\t\t[\"submit\", \"button\", \"reset\", \"image\"].includes(type)\n\t\t\t\t\t) {\n\t\t\t\t\t\tthrow \"not an input\"\n\t\t\t\t\t}\n\t\t\t\t\tif (type === \"checkbox\") {\n\t\t\t\t\t\tel.checked = !!value\n\t\t\t\t\t\treturn {\n\t\t\t\t\t\t\thooks: await $sys.emit(el, new Event(\"change\", { bubbles: true })),\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\tif (tag === \"select\") {\n\t\t\t\t\t\tif (el.multiple) {\n\t\t\t\t\t\t\tconst values = Array.isArray(value) ? value : [value]\n\t\t\t\t\t\t\tfor (const o of el.options) {\n\t\t\t\t\t\t\t\to.selected = values.includes(o.value)\n\t\t\t\t\t\t\t}\n\t\t\t\t\t\t} else {\n\t\t\t\t\t\t\tel.value = value\n\t\t\t\t\t\t}\n\t\t\t\t\t\treturn {\n\t\t\t\t\t\t\thooks: await $sys.emit(el, new Event(\"change\", { bubbles: true })),\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\tel.value = value\n\t\t\t\t\tconst n = await $sys.emit(el, new Event(\"input\", { bubbles: true }))\n\t\t\t\t\treturn {\n\t\t\t\t\t\thooks:\n\t\t\t\t\t\t\tn + (await $sys.emit(el, new Event(\"change\", { bubbles: true }))),\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t\tthrow \"unknown action \" + action\n\t\t})"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line scripts.gox:102
}

//line scripts.gox:104
func radioGroup(id string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line scripts.gox:105
			__e = __c.Set("data:id", id); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const id = $data(\"id\")\n\t\t$on(id, async ({ action, value }) => {\n\t\t\tconst els = Array.from(document.querySelectorAll('[data-kid=\"' + id + '\"]'))\n\t\t\tif (els.length === 0) {\n\t\t\t\tthrow \"element is not rendered\"\n\t\t\t}\n\t\t\tswitch (action) {\n\t\t\t\tcase \"focus\": {\n\t\t\t\t\tconst el = els.find((el) => el.checked) || els[0]\n\t\t\t\t\tel.focus({ preventScroll: true })\n\t\t\t\t\tel.scrollIntoView({ behavior: \"smooth\", block: \"nearest\" })\n\t\t\t\t\treturn {}\n\t\t\t\t}\n\t\t\t\tcase \"read\": {\n\t\t\t\t\tconst options = els.map((el) => ({\n\t\t\t\t\t\tvalue: el.value,\n\t\t\t\t\t\tlabel: el.labels && el.labels[0] ? el.labels[0].textContent.trim() : \"\",\n\t\t\t\t\t\tselected: el.checked,\n\t\t\t\t\t\tdisabled: el.disabled,\n\t\t\t\t\t}))\n\t\t\t\t\tconst checked = els.find((el) => el.checked)\n\t\t\t\t\treturn { value: checked ? checked.value : null, options }\n\t\t\t\t}\n\t\t\t\tcase \"input\": {\n\t\t\t\t\tconst el = els.find((el) => el.value === value)\n\t\t\t\t\tif (!el) {\n\t\t\t\t\t\tthrow \"no option with value \" + JSON.stringify(value)\n\t\t\t\t\t}\n\t\t\t\t\tel.checked = true\n\t\t\t\t\treturn {\n\t\t\t\t\t\thooks: await $sys.emit(el, new Event(\"change\", { bubbles: true })),\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t\tthrow \"unknown action \" + action\n\t\t})"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line scripts.gox:143
}
