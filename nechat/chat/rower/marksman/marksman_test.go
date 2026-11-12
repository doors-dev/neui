package marksman

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
)

func TestMainRendersInitialPartial(t *testing.T) {
	m := New("still typing the first para", time.Hour, nil)
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Elem {
		return m.Main()
	})
	srv := httptest.NewServer(app)
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "still typing the first para") {
		t.Fatalf("initial partial not rendered: %q", body)
	}
}
