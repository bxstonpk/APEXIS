package services

import (
	"context"
	"log/slog"

	"account/backend/internal/modules/identity/application/ports"
	"account/backend/internal/modules/identity/domain/events"
)

var _ ports.EventPublisher = (*LogEventPublisher)(nil)

// LogEventPublisher เขียน event ลง log — เปลี่ยนเป็น message bus / outbox ได้ภายหลัง
type LogEventPublisher struct{ logger *slog.Logger }

func NewLogEventPublisher(logger *slog.Logger) *LogEventPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogEventPublisher{logger: logger}
}

func (p *LogEventPublisher) Publish(ctx context.Context, evts ...events.Event) {
	for _, e := range evts {
		p.logger.InfoContext(ctx, "domain event", "event", e.EventName(), "at", e.OccurredAt(), "payload", e)
	}
}
