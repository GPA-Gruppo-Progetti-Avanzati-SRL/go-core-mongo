package mongo

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var otelMeter = otel.Meter("go-core-mongo")

type poolMetric struct {
	ActiveConnection metric.Int64UpDownCounter

	TimeToReadyConnection   metric.Int64Histogram
	TotalCloseConnection    metric.Int64Counter
	TotalReturnedConnection metric.Int64Counter

	TimeToAcquireConnection      metric.Int64Histogram
	TotalFailedAcquireConnection metric.Int64Counter
}

func (m *poolMetric) init() {
	m.ActiveConnection, _ = otelMeter.Int64UpDownCounter("mongo.connection.active", metric.WithDescription("Total active connection in the pool"))

	// TODO make bucket configurable
	m.TimeToReadyConnection, _ = otelMeter.Int64Histogram("mongo.connection.time.ready",
		metric.WithUnit("us"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10_000, 100_000, 200_000, 500_000, 1_000_000, 2_000_000),
		metric.WithDescription("Time for a connection completes a handshake and is ready to be used for operations"))

	m.TotalCloseConnection, _ = otelMeter.Int64Counter("mongo.connection.close", metric.WithDescription("Total closed connection"))
	m.TotalReturnedConnection, _ = otelMeter.Int64Counter("mongo.connection.pool.returned", metric.WithDescription("Incremented when a connection is checked back into the pool after an operation is executed"))

	m.TimeToAcquireConnection, _ = otelMeter.Int64Histogram("mongo.connection.pool.time.acquire", metric.WithUnit("us"), metric.WithDescription("Time for an operation successfully acquires a connection for execution"))

	m.TotalFailedAcquireConnection, _ = otelMeter.Int64Counter("mongo.connection.pool.acquire.failed", metric.WithDescription("Total operation cannot acquire a connection for execution"))

}
