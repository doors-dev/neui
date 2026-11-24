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

package stat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"neui.dev/internal/schema"
)

var english = message.NewPrinter(language.AmericanEnglish)

var german = message.NewPrinter(language.German)

func params(t *testing.T, raw string) Params {
	t.Helper()
	s, err := schema.NewSchema[Params]()
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Decode(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return p
}

func TestNewRefusesInvalidCombinations(t *testing.T) {
	for raw, want := range map[string]string{
		`{"label": "Revenue"}`: valueOrQueryMessage,
		`{"label": "Revenue", "value": 1, "query": "() => ({value: 1})"}`:       valueOrQueryMessage,
		`{"label": "Revenue", "value": 1, "refresh": 30}`:                       refreshWithoutQueryMessage,
		`{"label": "Revenue", "query": "() => ({value: 1})", "reference": 2}`:   referenceWithoutValueMessage,
		`{"label": "Revenue", "value": 1, "format": "currency"}`:                currencyUnitMessage,
		`{"label": "Revenue", "value": 1, "format": "currency", "unit": "BTC"}`: fmt.Sprintf(currencyCodeMessage, "BTC"),
		`{"label": "Revenue", "value": 1, "format": "currency", "unit": "XXX"}`: fmt.Sprintf(currencyCodeMessage, "XXX"),
	} {
		if _, err := New(context.Background(), params(t, raw)); err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %q", raw, err, want)
		}
	}
}

func TestNewAcceptsAndSetsDefaults(t *testing.T) {
	w, err := New(context.Background(), params(t, `{"label": "Revenue", "value": 1200, "reference": 1000}`))
	if err != nil {
		t.Fatal(err)
	}
	got := w.(*widget)
	if got.params.Format != "number" || got.params.Good != "up" || got.refresh != 0 {
		t.Fatalf("defaults: format %q, good %q, refresh %v", got.params.Format, got.params.Good, got.refresh)
	}
	if got.value != 1200 || got.reference == nil || *got.reference != 1000 {
		t.Fatalf("value %v, reference %v, want 1200 compared with 1000", got.value, got.reference)
	}
	w, err = New(context.Background(), params(t, `{"label": "Revenue", "value": 1, "format": "currency", "unit": "eur"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.(*widget); got.currency != currency.EUR || got.scale != 2 {
		t.Fatalf("eur: currency %v, scale %d", got.currency, got.scale)
	}
	for refresh, want := range map[int]time.Duration{0: 0, 3: max(3*time.Second, refreshFloor), 60: time.Minute, 86400: 24 * time.Hour} {
		w, err := New(context.Background(), params(t, fmt.Sprintf(`{"label": "Load", "query": "() => ({value: 1})", "refresh": %d}`, refresh)))
		if err != nil {
			t.Fatal(err)
		}
		if got := w.(*widget).refresh; got != want {
			t.Errorf("refresh %d: got %v, want %v", refresh, got, want)
		}
	}
}

func TestShow(t *testing.T) {
	for _, c := range []struct {
		format numberFormat
		value  float64
		want   string
	}{
		{numberFormat{printer: english, kind: "number"}, 0, "0"},
		{numberFormat{printer: english, kind: "number"}, 1234, "1,234"},
		{numberFormat{printer: english, kind: "number"}, 1234.567, "1,234.57"},
		{numberFormat{printer: english, kind: "number"}, -9999.5, "-9,999.5"},
		{numberFormat{printer: english, kind: "number"}, 0.0012345, "0.0012"},
		{numberFormat{printer: english, kind: "number"}, -0.0000001, "0"},
		{numberFormat{printer: english, kind: "number"}, 10000, "10K"},
		{numberFormat{printer: english, kind: "number"}, 9999.999, "10K"},
		{numberFormat{printer: english, kind: "number"}, 9999.994, "9,999.99"},
		{numberFormat{printer: english, kind: "number"}, 12345, "12.3K"},
		{numberFormat{printer: english, kind: "number"}, 999950, "1M"},
		{numberFormat{printer: english, kind: "number"}, -4260000, "-4.3M"},
		{numberFormat{printer: english, kind: "number"}, 2.5e9, "2.5B"},
		{numberFormat{printer: english, kind: "number"}, 1.5e12, "1,500B"},
		{numberFormat{printer: english, kind: "number", unit: "users"}, 1284, "1,284 users"},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, 1234.5, "$1,234.50"},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, -42, "-$42.00"},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, 4200000, "$4.2M"},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, 9999.999, "$10K"},
		{numberFormat{printer: english, kind: "currency", symbol: "¥", scale: 0}, 1234.4, "¥1,234"},
		{numberFormat{printer: english, kind: "currency", symbol: "CHF", scale: 2}, 99.9, "CHF 99.90"},
		{numberFormat{printer: english, kind: "percent"}, 0.12, "12%"},
		{numberFormat{printer: english, kind: "percent"}, 0.1234, "12.3%"},
		{numberFormat{printer: english, kind: "percent"}, 0.001234, "0.12%"},
		{numberFormat{printer: english, kind: "percent"}, -0.5, "-50%"},
		{numberFormat{printer: english, kind: "percent"}, 150, "15K%"},
		{numberFormat{printer: english, kind: "duration"}, 0, "0s"},
		{numberFormat{printer: english, kind: "duration"}, 0.25, "250ms"},
		{numberFormat{printer: english, kind: "duration"}, 45, "45s"},
		{numberFormat{printer: english, kind: "duration"}, 90, "1m 30s"},
		{numberFormat{printer: english, kind: "duration"}, 3600, "1h"},
		{numberFormat{printer: english, kind: "duration"}, 3900, "1h 5m"},
		{numberFormat{printer: english, kind: "duration"}, 3929, "1h 5m"},
		{numberFormat{printer: english, kind: "duration"}, 2*86400 + 3*3600 + 59, "2d 3h"},
		{numberFormat{printer: english, kind: "duration"}, -90, "-1m 30s"},
		{numberFormat{printer: english, kind: "bytes"}, 512, "512 B"},
		{numberFormat{printer: english, kind: "bytes"}, 1536, "1.5 KB"},
		{numberFormat{printer: english, kind: "bytes"}, 1048576, "1 MB"},
		{numberFormat{printer: english, kind: "bytes"}, 1048575, "1 MB"},
		{numberFormat{printer: english, kind: "bytes"}, 5.5 * 1024 * 1024 * 1024, "5.5 GB"},
	} {
		if got := c.format.show(c.value); got != c.want {
			t.Errorf("%s %v: got %q, want %q", c.format.kind, c.value, got, c.want)
		}
	}
}

func TestExact(t *testing.T) {
	for _, c := range []struct {
		format numberFormat
		value  float64
		want   string
	}{
		{numberFormat{printer: english, kind: "number"}, 12345.678, "12,345.678"},
		{numberFormat{printer: english, kind: "number"}, 0.30000000000000004, "0.3"},
		{numberFormat{printer: english, kind: "number", unit: "users"}, 1284, "1,284 users"},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, -4212345.5, "-$4,212,345.5"},
		{numberFormat{printer: english, kind: "currency", symbol: "CHF", scale: 2}, 99.9, "CHF 99.9"},
		{numberFormat{printer: english, kind: "percent"}, 0.123, "12.3%"},
		{numberFormat{printer: english, kind: "duration"}, 3929, "3,929 s"},
		{numberFormat{printer: english, kind: "bytes"}, 1234567, "1,234,567 B"},
	} {
		if got := c.format.exact(c.value); got != c.want {
			t.Errorf("%s %v: got %q, want %q", c.format.kind, c.value, got, c.want)
		}
	}
}

func TestChange(t *testing.T) {
	for _, c := range []struct {
		format    numberFormat
		value     float64
		reference float64
		want      string
		direction int
	}{
		{numberFormat{printer: english, kind: "number"}, 120, 100, "+20.0%", 1},
		{numberFormat{printer: english, kind: "number"}, 80, 100, "-20.0%", -1},
		{numberFormat{printer: english, kind: "number"}, 100, 100, "0%", 0},
		{numberFormat{printer: english, kind: "number"}, -50, -100, "+50.0%", 1},
		{numberFormat{printer: english, kind: "number"}, 1001, 1000, "+0.1%", 1},
		{numberFormat{printer: english, kind: "number"}, 250, 1, "+24,900.0%", 1},
		{numberFormat{printer: english, kind: "number", unit: "users"}, 12, 0, "+12 users", 1},
		{numberFormat{printer: english, kind: "currency", symbol: "$", scale: 2}, -50, 0, "-$50.00", -1},
		{numberFormat{printer: english, kind: "duration"}, 3900, 0, "+1h 5m", 1},
		{numberFormat{printer: english, kind: "number"}, 0, 0, "0%", 0},
	} {
		got, direction := c.format.change(c.value, c.reference)
		if got != c.want || direction != c.direction {
			t.Errorf("%v against %v: got %q %d, want %q %d", c.value, c.reference, got, direction, c.want, c.direction)
		}
	}
}

func TestFormatFollowsTheLanguage(t *testing.T) {
	for _, c := range []struct {
		format numberFormat
		value  float64
		want   string
	}{
		{numberFormat{printer: german, kind: "number"}, 1234.567, "1.234,57"},
		{numberFormat{printer: german, kind: "number"}, 12345, "12,3K"},
		{numberFormat{printer: german, kind: "number", unit: "users"}, -1284, "-1.284 users"},
		{numberFormat{printer: german, kind: "currency", symbol: german.Sprint(currency.Symbol(currency.EUR)), scale: 2}, 1234.5, "€1.234,50"},
		{numberFormat{printer: german, kind: "percent"}, 0.1234, "12,3\u00a0%"},
		{numberFormat{printer: german, kind: "duration"}, 3900, "1h 5m"},
		{numberFormat{printer: german, kind: "bytes"}, 1536, "1,5 KB"},
		{numberFormat{printer: message.NewPrinter(language.French), kind: "number"}, 1234.5, "1\u00a0234,5"},
		{numberFormat{printer: message.NewPrinter(language.Turkish), kind: "percent"}, 0.12, "%12"},
		{numberFormat{printer: message.NewPrinter(language.Persian), kind: "number"}, -42, "-۴۲"},
		{numberFormat{printer: message.NewPrinter(language.Und), kind: "number"}, 1234.5, "1,234.5"},
	} {
		if got := c.format.show(c.value); got != c.want {
			t.Errorf("%s %v: got %q, want %q", c.format.kind, c.value, got, c.want)
		}
	}
	if got := (numberFormat{printer: german, kind: "number"}).exact(12345.678); got != "12.345,678" {
		t.Errorf("exact: got %q, want 12.345,678", got)
	}
	if got, _ := (numberFormat{printer: german, kind: "number"}).change(120, 100); got != "+20,0\u00a0%" {
		t.Errorf("change: got %q, want +20,0 %%", got)
	}
	if got := german.Sprint(currency.Symbol(currency.USD)); got != "$" {
		t.Errorf("usd in german: got %q, want $", got)
	}
}

func TestSchemaReadsAsDescribed(t *testing.T) {
	s, err := schema.NewSchema[Params]()
	if err != nil {
		t.Fatal(err)
	}
	raw := string(s.View().Raw)
	for _, want := range []string{`"label"`, `"referenceLabel"`, `"refresh"`, `"good"`, `"maximum":86400`} {
		if !strings.Contains(raw, want) {
			t.Errorf("schema lacks %s: %s", want, raw)
		}
	}
	if _, err := schema.NewSchema[Patch](); err != nil {
		t.Fatal(err)
	}
}
