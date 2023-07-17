package event_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

func TestTracing(t *testing.T) {
	t.Run("unmarshals a full tracing context correctly.", func(t *testing.T) {
		_, span := trace.NewTracerProvider().Tracer("test").Start(context.Background(), "test")

		tracingContext := event.TracingContext{
			TraceID:    span.SpanContext().TraceID(),
			SpanID:     span.SpanContext().SpanID(),
			TraceFlags: span.SpanContext().TraceFlags(),
			TraceState: span.SpanContext().TraceState(),
		}

		bytes, err := json.Marshal(tracingContext)
		assert.NoError(t, err)

		fmt.Printf("marshalled tracing context: %s\n", string(bytes))

		var unmarshalledTracingContext event.TracingContext
		err = json.Unmarshal(bytes, &unmarshalledTracingContext)
		assert.NoError(t, err)

		assert.Equal(t, tracingContext, unmarshalledTracingContext)
	})

	t.Run("fails on a partial tracing context.", func(t *testing.T) {
		rawTracingContexts := []string{
			"{\"SpanId\":\"c31bc0a7013beab8\",\"TraceFlag\":\"01\",\"Tracestate\":\"\"}",
			"{\"TraceId\":\"eb0e08452e7ee4b0d3b8b30987c37951\",\"TraceFlag\":\"01\",\"Tracestate\":\"\"}",
			"{\"TraceId\":\"eb0e08452e7ee4b0d3b8b30987c37951\",\"SpanId\":\"c31bc0a7013beab8\",\"Tracestate\":\"\"}",
		}

		for _, rawTracingContext := range rawTracingContexts {
			var tracingContext event.TracingContext
			err := json.Unmarshal([]byte(rawTracingContext), &tracingContext)
			assert.Error(t, err)
		}
	})

	t.Run("converts tracing context to span context and back again.", func(t *testing.T) {
		_, span := trace.NewTracerProvider().Tracer("test").Start(context.Background(), "test")

		tracingContext := event.TracingContext{
			TraceID:    span.SpanContext().TraceID(),
			SpanID:     span.SpanContext().SpanID(),
			TraceFlags: span.SpanContext().TraceFlags(),
			TraceState: span.SpanContext().TraceState(),
		}

		spanContext := tracingContext.ToSpanContext()

		tracingContextAgain := event.FromSpanContext(spanContext)

		assert.Equal(t, tracingContext, *tracingContextAgain)
	})
}
