//go:build browser

package devkit_test

import (
	"testing"
	"time"
)

const seriesJumpSpec = `{fields: [
	{type: "textarea", label: "One", name: "one", rows: 6},
	{type: "textarea", label: "Two", name: "two", rows: 6},
	{type: "series", label: "Delivery", name: "delivery", required: true, fields: [
		"(p) => ({type: 'select', label: 'Method', name: 'method', options: [{value: 'ship', label: 'Ship'}, {value: 'pickup', label: 'Pickup'}]})",
		"(p) => (((t) => { while (Date.now() - t < 150) {} })(Date.now()), p[0].value === 'ship') ? {type: 'text', label: 'Address', name: 'address'} : {type: 'time', label: 'Pickup time', name: 'at'}"
	]}
]}`

func TestSeriesSwitchingAStepKeepsContentAboveInPlace(t *testing.T) {
	a, f := formApp(t, seriesJumpSpec)
	a.Page.MustSetViewport(800, 300, 1, false)
	delivery := f.Series("Delivery")
	delivery.Step(0).Field("Method").Choose("Ship")
	delivery.Step(1).Field("Address").ExpectNoError()
	a.Page.MustEval(`() => {
		window.scrollTo(0, document.documentElement.scrollHeight)
		const box = [...document.querySelectorAll('fieldset')].find(x => x.textContent.includes('Delivery'))
		window.jumpTops = []
		const sample = () => { window.jumpTops.push(Math.round(box.getBoundingClientRect().top)); requestAnimationFrame(sample) }
		requestAnimationFrame(sample)
	}`)
	for i, step := range []string{"Pickup", "Ship", "Pickup", "Ship"} {
		label := map[string]string{"Ship": "Address", "Pickup": "Pickup time"}[step]
		a.Page.MustEval(`() => { window.jumpTops = [] }`)
		delivery.Step(0).Field("Method").Choose(step)
		delivery.Step(1).Field(label).ExpectNoError()
		time.Sleep(100 * time.Millisecond)
		tops := a.Page.MustEval(`() => [...new Set(window.jumpTops)].join(",")`).Str()
		if a.Page.MustEval(`() => new Set(window.jumpTops).size`).Int() != 1 {
			t.Fatalf("switch %d to %s: series top moved across frames: %s", i, step, tops)
		}
	}
}
