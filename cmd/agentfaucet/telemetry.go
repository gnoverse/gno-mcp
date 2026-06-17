package main

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"

	"github.com/gnoverse/gno-mcp/faucet"
)

// telemetry holds the wired-up metrics pipeline for agentfaucet.
type telemetry struct {
	handler  http.Handler // serves /metrics
	metrics  *otelMetrics // implements faucet.Metrics
	meter    otelmetric.Meter
	provider *metric.MeterProvider
	res      *resource.Resource // shared with a future tracer provider
	shutdown func(context.Context) error
}

// setupTelemetry builds the resource, Prometheus exporter (on a private
// registry), meter provider, and the outcome counter. The returned handler
// serves /metrics. shutdown flushes and stops the provider; chain a tracer
// provider's Shutdown into it when traces are added later.
func setupTelemetry(_ context.Context, version string) (*telemetry, error) {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("agentfaucet"),
			semconv.ServiceVersion(version),
		))
	if err != nil {
		return nil, err
	}

	reg := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, err
	}
	provider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(exporter))
	meter := provider.Meter("github.com/gnoverse/gno-mcp/cmd/agentfaucet")

	m, err := newOtelMetrics(meter)
	if err != nil {
		return nil, err
	}

	return &telemetry{
		handler:  promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		metrics:  m,
		meter:    meter,
		provider: provider,
		res:      res,
		shutdown: provider.Shutdown,
	}, nil
}

// otelMetrics implements faucet.Metrics with a single bounded-cardinality
// counter labeled by outcome.
type otelMetrics struct {
	fundRequests otelmetric.Int64Counter
}

func newOtelMetrics(meter otelmetric.Meter) (*otelMetrics, error) {
	c, err := meter.Int64Counter(
		"faucet.fund.requests",
		otelmetric.WithDescription("Total fund requests by outcome."),
		otelmetric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	return &otelMetrics{fundRequests: c}, nil
}

func (m *otelMetrics) RecordOutcome(ctx context.Context, outcome string) {
	m.fundRequests.Add(ctx, 1, otelmetric.WithAttributes(attribute.String("outcome", outcome)))
}

// registerGauges wires the observable gauges: the funding-wallet balance reads
// the atomic that the balance poller keeps fresh (never blocking the scrape on
// an RPC), and the burst-state gauges read Limiter.Snapshot. daily-cap-remaining
// and drip-tokens-available are derived from the snapshot.
func (t *telemetry) registerGauges(lim *faucet.Limiter, balance *atomic.Int64) error {
	bal, err := t.meter.Int64ObservableGauge(
		"faucet.funding.balance.ugnot",
		otelmetric.WithDescription("Funding wallet balance in ugnot (TTL-polled)."),
	)
	if err != nil {
		return err
	}
	dripAvail, err := t.meter.Float64ObservableGauge(
		"faucet.drip.tokens.available",
		otelmetric.WithDescription("Global drip token-bucket tokens available now (ugnot)."),
	)
	if err != nil {
		return err
	}
	capRemain, err := t.meter.Int64ObservableGauge(
		"faucet.daily.cap.remaining.ugnot",
		otelmetric.WithDescription("Remaining daily outflow budget in ugnot."),
	)
	if err != nil {
		return err
	}

	_, err = t.meter.RegisterCallback(
		func(_ context.Context, o otelmetric.Observer) error {
			// A negative sentinel means the balance has never been polled
			// successfully; leave the gauge unset so an unseeded balance reads as
			// "no data" rather than a misleading 0 (empty wallet).
			if v := balance.Load(); v >= 0 {
				o.ObserveInt64(bal, v)
			}
			s := lim.Snapshot()
			if s.DripEnabled {
				o.ObserveFloat64(dripAvail, s.DripTokens)
			}
			o.ObserveInt64(capRemain, s.DailyCapUgnot-s.DaySpentUgnot)
			return nil
		},
		bal, dripAvail, capRemain,
	)
	return err
}
