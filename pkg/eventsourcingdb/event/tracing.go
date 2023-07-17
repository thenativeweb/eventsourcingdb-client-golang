package event

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
)

type TracingContext struct {
	TraceID    trace.TraceID    `json:"TraceId"`
	SpanID     trace.SpanID     `json:"SpanId"`
	TraceFlags trace.TraceFlags `json:"TraceFlag"`
	TraceState trace.TraceState `json:"Tracestate"`
}

type rawTracingContext struct {
	TraceID    string `json:"TraceId"`
	SpanID     string `json:"SpanId"`
	TraceFlags string `json:"TraceFlag"`
	TraceState string `json:"Tracestate"`
}

func (tracingContext *TracingContext) UnmarshalJSON(bytes []byte) error {
	var rawContext rawTracingContext
	if err := json.Unmarshal(bytes, &rawContext); err != nil {
		return err
	}

	traceID, err := trace.TraceIDFromHex(rawContext.TraceID)
	if err != nil {
		return err
	}

	spanID, err := trace.SpanIDFromHex(rawContext.SpanID)
	if err != nil {
		return err
	}

	rawTraceFlags, err := hex.DecodeString(rawContext.TraceFlags)
	if err != nil {
		return err
	}
	if len(rawTraceFlags) != 1 {
		return fmt.Errorf("trace flag must consist of exactly one byte")
	}
	traceFlags := trace.TraceFlags(rawTraceFlags[0])

	traceState, err := trace.ParseTraceState(rawContext.TraceState)
	if err != nil {
		return err
	}

	tracingContext.TraceID = traceID
	tracingContext.SpanID = spanID
	tracingContext.TraceFlags = traceFlags
	tracingContext.TraceState = traceState

	return nil
}

func FromSpanContext(spanContext trace.SpanContext) *TracingContext {
	return &TracingContext{
		TraceID:    spanContext.TraceID(),
		SpanID:     spanContext.SpanID(),
		TraceFlags: spanContext.TraceFlags(),
		TraceState: spanContext.TraceState(),
	}
}

func (tracingContext *TracingContext) ToSpanContext() trace.SpanContext {
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tracingContext.TraceID,
		SpanID:     tracingContext.SpanID,
		TraceFlags: tracingContext.TraceFlags,
		TraceState: tracingContext.TraceState,
		Remote:     true,
	})
}
