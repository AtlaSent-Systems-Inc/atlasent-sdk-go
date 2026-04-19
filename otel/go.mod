module github.com/atlasent-systems-inc/atlasent-sdk-go/otel

go 1.24.7

require (
	github.com/atlasent-systems-inc/atlasent-sdk-go v0.0.0
	go.opentelemetry.io/otel v1.33.0
	go.opentelemetry.io/otel/metric v1.33.0
	go.opentelemetry.io/otel/trace v1.33.0
)

replace github.com/atlasent-systems-inc/atlasent-sdk-go => ../
