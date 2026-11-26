// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

func newNormalizationFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType("normalize_arrays"),
		func() component.Config { return &struct{}{} },
		processor.WithTraces(
			func(_ context.Context, _ processor.Settings, _ component.Config, next consumer.Traces,
			) (processor.Traces, error) {
				return &normalizationProcessor{next: next}, nil
			},
			component.StabilityLevelStable,
		),
	)
}

type normalizationProcessor struct {
	component.StartFunc
	component.ShutdownFunc
	next consumer.Traces
}

func (*normalizationProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *normalizationProcessor) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	for _, resourceSpans := range traces.ResourceSpans().All() {
		if err := normalizeArrayAttributes(resourceSpans.Resource().Attributes()); err != nil {
			return err
		}
		for _, scopeSpans := range resourceSpans.ScopeSpans().All() {
			for _, span := range scopeSpans.Spans().All() {
				if err := normalizeArrayAttributes(span.Attributes()); err != nil {
					return err
				}
				for _, event := range span.Events().All() {
					if err := normalizeArrayAttributes(event.Attributes()); err != nil {
						return err
					}
				}
			}
		}
	}
	return p.next.ConsumeTraces(ctx, traces)
}

func normalizeArrayAttributes(attributes pcommon.Map) error {
	for key, value := range attributes.All() {
		if value.Type() != pcommon.ValueTypeSlice {
			continue
		}
		// Azure Monitor drops array attributes; preserve the hopper's JSON representation.
		data, err := json.Marshal(value.Slice().AsRaw())
		if err != nil {
			return fmt.Errorf("failed to normalize array attribute (%s): %w", key, err)
		}
		value.SetStr(string(data))
	}
	return nil
}
