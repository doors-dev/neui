package form_test

import (
	"context"
	"encoding/json"
	"testing"

	"neui.dev/nekit/internal/island/form"
)

func TestNewRefusesSubmitsWithTheSameTextAndValue(t *testing.T) {
	for _, tc := range []struct {
		submits string
		want    string
	}{
		{`[{"text": "Save"}, {"text": "Save"}]`, `submits: two buttons have text "Save" and value ""`},
		{`[{"text": "Save", "value": "a"}, {"text": "Keep"}, {"text": "Save", "value": "a", "style": "danger"}]`, `submits: two buttons have text "Save" and value "a"`},
		{`[{"text": "Save", "value": "a"}, {"text": "Save", "value": "b"}, {"text": "Keep", "value": "a"}]`, ""},
	} {
		var p form.Params
		if err := json.Unmarshal([]byte(`{"fields": [], "submits": `+tc.submits+`}`), &p); err != nil {
			t.Fatal(err)
		}
		_, err := form.New(context.Background(), p, nil)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("submits %s refused with %v, want them taken", tc.submits, err)
		case tc.want != "" && (err == nil || err.Error() != tc.want):
			t.Errorf("submits %s gave %v, want %q", tc.submits, err, tc.want)
		}
	}
}
