package views

import (
	"context"

	"github.com/ubccr/grendel/frontend/types"
)

func CtxString(key string, ctx context.Context) string {
	s, _ := ctx.Value(key).(string)
	return s
}

func CtxBool(key string, ctx context.Context) bool {
	b, _ := ctx.Value(key).(bool)
	return b
}
func CtxEvents(ctx context.Context) []types.EventStruct {
	e, _ := ctx.Value("events").([]types.EventStruct)
	return e
}
