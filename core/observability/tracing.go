package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TraceID represents a trace identifier.
type TraceID string

// SpanID represents a span identifier.
type SpanID string

// Span represents a span in a distributed trace.
type Span struct {
	TraceID    TraceID
	SpanID     SpanID
	ParentID   SpanID
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Tags       map[string]string
	Logs       []SpanLog
	mu         sync.RWMutex
}

// SpanLog represents a log entry within a span.
type SpanLog struct {
	Timestamp time.Time
	Fields    map[string]interface{}
}

// Tracer provides distributed tracing capabilities.
type Tracer struct {
	mu sync.RWMutex

	spans map[SpanID]*Span
}

// NewTracer creates a new tracer.
func NewTracer() *Tracer {
	return &Tracer{
		spans: make(map[SpanID]*Span),
	}
}

// StartSpan starts a new span.
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	var traceID TraceID
	var parentID SpanID

	// Try to get trace ID and parent span ID from context
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		traceID = parentSpan.TraceID
		parentID = parentSpan.SpanID
	} else {
		// Create new trace ID
		traceID = generateTraceID()
	}

	spanID := generateSpanID()
	span := &Span{
		TraceID:   traceID,
		SpanID:    spanID,
		ParentID:  parentID,
		Name:      name,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Logs:      make([]SpanLog, 0),
	}

	t.mu.Lock()
	t.spans[spanID] = span
	t.mu.Unlock()

	// Add span to context
	ctx = WithSpan(ctx, span)

	return ctx, span
}

// Finish finishes a span.
func (s *Span) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
}

// SetTag sets a tag on the span.
func (s *Span) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tags[key] = value
}

// Log adds a log entry to the span.
func (s *Span) Log(fields map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Logs = append(s.Logs, SpanLog{
		Timestamp: time.Now(),
		Fields:    fields,
	})
}

// GetTraceID returns the trace ID.
func (s *Span) GetTraceID() TraceID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TraceID
}

// GetSpanID returns the span ID.
func (s *Span) GetSpanID() SpanID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SpanID
}

// GetDuration returns the span duration.
func (s *Span) GetDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Duration
}

// GetAllSpans returns all spans in the tracer.
func (t *Tracer) GetAllSpans() []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	spans := make([]*Span, 0, len(t.spans))
	for _, span := range t.spans {
		spans = append(spans, span)
	}
	return spans
}

// GetSpansByTraceID returns all spans for a given trace ID.
func (t *Tracer) GetSpansByTraceID(traceID TraceID) []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	spans := make([]*Span, 0)
	for _, span := range t.spans {
		if span.TraceID == traceID {
			spans = append(spans, span)
		}
	}
	return spans
}

// generateTraceID generates a new trace ID.
func generateTraceID() TraceID {
	return TraceID(generateID(16)) // 128 bits
}

// generateSpanID generates a new span ID.
func generateSpanID() SpanID {
	return SpanID(generateID(8)) // 64 bits
}

// generateID generates a random hexadecimal ID of the given byte length.
func generateID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// SpanFromContext extracts span from context.
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanKey).(*Span); ok {
		return span
	}
	return nil
}

// WithSpan adds span to context.
func WithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanKey, span)
}

// TraceIDFromContext extracts trace ID from context.
func TraceIDFromContext(ctx context.Context) TraceID {
	if span := SpanFromContext(ctx); span != nil {
		return span.GetTraceID()
	}
	return ""
}

type spanContextKey string

const spanKey spanContextKey = "span"

// DefaultTracer is the default tracer instance.
var defaultTracer *Tracer
var defaultTracerOnce sync.Once

// DefaultTracer returns the default tracer instance.
func DefaultTracer() *Tracer {
	defaultTracerOnce.Do(func() {
		defaultTracer = NewTracer()
	})
	return defaultTracer
}

// StartSpan is a convenience function that starts a span using the default tracer.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	return DefaultTracer().StartSpan(ctx, name)
}

