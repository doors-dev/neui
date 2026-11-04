package stack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
)

func Page(t *testing.T, h Host) *Chat {
	t.Helper()
	rendered := make(chan page.Page, 1)
	app := doors.NewApp(func(context.Context, doors.Request) gox.Elem {
		return func(cur gox.Cursor) error {
			p := page.NewPage(doors.InstanceContext(cur.Context()))
			rendered <- p
			return doors.Ctx(context.WithValue(cur.Context(), common.KeySpace, p.Space())).Proxy(cur, h.Page)
		}
	}, doors.WithID(doors.IDRand()), doors.WithConf(doors.Conf{InstanceConnectTimeout: time.Minute}))
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the page rendered with status %d: %s", response.Code, response.Body)
	}
	select {
	case p := <-rendered:
		return open(t, h, context.WithValue(context.Background(), common.ConfigKey{}, h.Config), p)
	default:
		t.Fatal("the app served no page")
		return nil
	}
}
