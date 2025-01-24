package api

import (
	"context"
	"slices"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/ubccr/grendel/pkg/model"
)

func (h *Handler) GetEvents(c fuego.ContextNoBody) (model.EventList, error) {
	events := h.Events
	slices.Reverse(events)
	return events, nil
}

func (h *Handler) writeEvent(ctx context.Context, severity, msg string, jobMessages ...model.JobMessage) {
	var username string
	switch ctx.Value("username").(type) {
	case string:
		username = ctx.Value("username").(string)
	default:
		return
	}

	h.Events = append(h.Events, model.Event{
		Severity:    severity,
		User:        username,
		Time:        time.Now().UTC(),
		Message:     msg,
		JobMessages: jobMessages,
	})

	if len(h.Events) > 50 {
		h.Events = h.Events[:50]
	}
}
