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
		return open(t, h, context.WithValue(context.WithValue(context.Background(), common.ConfigKey{}, h.Config), common.IslandConfigKey{}, h.Island), p)
	default:
		t.Fatal("the app served no page")
		return nil
	}
}
