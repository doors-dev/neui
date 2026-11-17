package pending

import (
	"context"

	"github.com/doors-dev/doors"
	"neui.dev"
	"neui.dev/internal/common"
)

func Indicator(ctx context.Context) doors.Indicators {
	id := neui.IslandID(ctx)
	if id == "" {
		return nil
	}
	return doors.IndicateAttrQueryAll(submits(id), "aria-disabled", "true").
		And(doors.IndicateAttr("aria-busy", "true")).
		And(doors.IndicateAttr("aria-disabled", nil))
}

func FormIndicator(ctx context.Context) doors.Indicators {
	id := neui.IslandID(ctx)
	if id == "" {
		return nil
	}
	return doors.IndicateAttrQueryAll(submits(id), "aria-disabled", "true").
		And(doors.IndicateAttrSubmitter("aria-busy", "true")).
		And(doors.IndicateAttrSubmitter("aria-disabled", nil))
}

func submits(id string) string {
	return "[" + common.IslandIDAttr + `="` + id + `"] [data-neui="submit"]`
}
