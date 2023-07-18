package event_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

func TestTracing(t *testing.T) {
	t.Run("unmarshals a full tracing context correctly.", func(t *testing.T) {
		rawTracingContext := []byte("{\"traceId\":\"eb0e08452e7ee4b0d3b8b30987c37951\",\"spanId\":\"c31bc0a7013beab8\",\"traceFlag\":\"01\",\"traceState\":\"foo=bar\"}")

		var unmarshalledTracingContext event.TracingContext
		err := json.Unmarshal(rawTracingContext, &unmarshalledTracingContext)
		assert.NoError(t, err)

		assert.Equal(t, "eb0e08452e7ee4b0d3b8b30987c37951", unmarshalledTracingContext.TraceID.String())
		assert.Equal(t, "c31bc0a7013beab8", unmarshalledTracingContext.SpanID.String())
		assert.Equal(t, "01", unmarshalledTracingContext.TraceFlags.String())
		assert.Equal(t, "foo=bar", unmarshalledTracingContext.TraceState.String())
	})

	t.Run("fails on a partial tracing context.", func(t *testing.T) {
		rawTracingContexts := []string{
			"{\"spanId\":\"c31bc0a7013beab8\",\"traceFlag\":\"01\",\"traceState\":\"foo=bar\"}",
			"{\"traceId\":\"eb0e08452e7ee4b0d3b8b30987c37951\",\"traceFlag\":\"01\",\"traceState\":\"heck=meck,foo=bar\"}",
			"{\"traceId\":\"eb0e08452e7ee4b0d3b8b30987c37951\",\"spanId\":\"c31bc0a7013beab8\",\"traceState\":\"\"}",
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
