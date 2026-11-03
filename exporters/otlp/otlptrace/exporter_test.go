// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlptrace_test

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type client struct {
	uploadErr   error
	logEndpoint string
}

var _ otlptrace.Client = &client{}

func (c *client) Start(ctx context.Context) error {
	return nil
}

func (c *client) Stop(ctx context.Context) error {
	return nil
}

func (c *client) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	return c.uploadErr
}

func (c *client) MarshalLog() interface{} {
	return struct{ Endpoint string }{Endpoint: c.logEndpoint}
}

func TestExporterMarshalLogDoesNotIncludeClientConfig(t *testing.T) {
	const sensitiveEndpoint = "user:pass@collector.internal:4318"

	var buf bytes.Buffer
	logger := funcr.New(func(_, args string) {
		_, _ = buf.WriteString(args)
	}, funcr.Options{})

	exp := otlptrace.NewUnstarted(&client{logEndpoint: sensitiveEndpoint})
	logger.Info("exporter", "config", exp)

	logged := buf.String()
	assert.Contains(t, logged, "otlptrace")
	assert.NotContains(t, logged, sensitiveEndpoint)
}

func TestExporterMarshalLogClientIsTypeName(t *testing.T) {
	const sensitiveEndpoint = "user:pass@collector.internal:4318"

	exp := otlptrace.NewUnstarted(&client{logEndpoint: sensitiveEndpoint})

	v := reflect.ValueOf(exp.MarshalLog())
	if !assert.Equal(t, reflect.Struct, v.Kind()) {
		return
	}
	assert.Equal(t, "otlptrace", v.FieldByName("Type").String())

	// The Client field must only describe the client type. Embedding the
	// client value lets a logr.Marshaler client expose its configuration.
	c := v.FieldByName("Client")
	if !assert.True(t, c.IsValid(), "missing Client field") {
		return
	}
	assert.Equal(t, reflect.String, c.Kind())
	assert.Equal(t, "*otlptrace_test.client", c.String())
	assert.NotContains(t, c.String(), sensitiveEndpoint)
}

func TestExporterClientError(t *testing.T) {
	ctx := context.Background()
	exp, err := otlptrace.New(ctx, &client{
		uploadErr: context.Canceled,
	})
	assert.NoError(t, err)

	spans := tracetest.SpanStubs{{Name: "Span 0"}}.Snapshots()
	err = exp.ExportSpans(ctx, spans)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, strings.HasPrefix(err.Error(), "traces export: "), err)

	assert.NoError(t, exp.Shutdown(ctx))
}
