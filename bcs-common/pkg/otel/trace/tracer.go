/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package trace

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/exporter/jaeger"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/exporter/otlp/otlpgrpctrace"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/resource"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/utils"

	"go.opentelemetry.io/otel/attribute"
	oteljaeger "go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

var (
	// errSwitchType switch type error
	errSwitchType error = errors.New("error switch type, please input: [on or off]")
	// errTracingType tracing type error
	errTracingType error = errors.New("error tracing type, please input: [jaeger, zipkin, otlpgrpc or otlphttp]")
	// errServiceName for service name is null
	errServiceName error = errors.New("error service name is null")
)

const (
	// defaultSwitchType for default switch type
	defaultSwitchType = "off"
	// defaultTracingType for default tracing type
	defaultTracingType = "jaeger"
	// defaultJaegerCollectorEndpoint sets default jaeger collector endpoint
	defaultJaegerCollectorEndpoint = "http://localhost:14268/api/traces"
	// defaultAgentEndpointHost sets default jaeger agent endpoint host
	defaultJaegerAgentEndpointHost = "localhost"
	// defaultAgentEndpointPort sets default jaeger agent endpoint host
	defaultJaegerAgentEndpointPort = "6831"
)

// TracerProviderConfig set TracerProviderConfig for different tracing systems
type TracerProviderConfig struct {
	TracingSwitch string `json:"tracingSwitch" value:"off" usage:"tracing switch"`
	TracingType   string `json:"tracingType" value:"jaeger" usage:"tracing type(default jaeger)"`
	ServiceName   string `json:"serviceName" value:"bcs-common/pkg/otel" usage:"tracing serviceName"`
	// Jaeger exporter endpoint
	JaegerConfig jaeger.EndpointConfig `json:"jaegerConfig"`
	// GrpcConfig
	GrpcConfig otlpgrpctrace.GrpcConfig
	// Resource attributes
	ResourceAttrs []attribute.KeyValue `json:"resourceAttrs" value:"" usage:"attributes of traced service"`
	// Sampler kinds
	AlwaysOnSampler   bool
	AlwaysOffSampler  bool
	RatioBasedSampler float64
	DefaultOnSampler  bool
	DefaultOffSampler bool
	// ParentBasedSampler bool
}

// InitTracerProvider initialize an OTLP tracer provider with processors and exporters.
func InitTracerProvider(serviceName string, tpos ...TracerProviderOption) (*sdktrace.TracerProvider, error) {
	defaultOptions := &TracerProviderConfig{
		TracingSwitch: defaultSwitchType,
		TracingType:   defaultTracingType,
		ServiceName:   serviceName,
		JaegerConfig: jaeger.EndpointConfig{
			CollectorEndpointConfig: &jaeger.CollectorEndpointConfig{
				CollectorEndpoint: defaultJaegerCollectorEndpoint,
				Username:          "",
				Password:          "",
				HttpClient:        http.DefaultClient,
			},
		},
		GrpcConfig: otlpgrpctrace.GrpcConfig{
			Endpoint: fmt.Sprintf("%s:%d", DefaultCollectorHost, DefaultCollectorPort),
			URLPath:  DefaultTracesPath,
		},
		AlwaysOnSampler:   false,
		AlwaysOffSampler:  false,
		RatioBasedSampler: 0,
		DefaultOnSampler:  false,
		DefaultOffSampler: false,
	}

	for _, t := range tpos {
		t(defaultOptions)
	}

	err := validateTracingOptions(defaultOptions)
	if err != nil {
		blog.Errorf("validateTracingOptions failed: %v", err)
		return nil, err
	}

	if defaultOptions.TracingSwitch == "off" {
		return &sdktrace.TracerProvider{}, nil
	}
	sampler := initSampler(defaultOptions)
	switch defaultOptions.TracingType {
	case string(Jaeger):
		err := validateJaegerEndpoint(defaultOptions)
		if err != nil {
			blog.Errorf("failed to create jaeger exporter:", err.Error())
			return nil, err
		}
		blog.Info("Using jaeger exporter")
		opts := initCollectorEndpoint(defaultOptions.JaegerConfig.CollectorEndpointConfig)
		jaegerExporter, err := jaeger.New(jaeger.WithCollectorEndpoint(opts...))
		if err != nil {
			blog.Errorf("failed to create jaeger exporter:", err.Error())
			return nil, err
		}
		processors := initProcessors(jaegerExporter)
		return newTracerProvider(defaultOptions.ServiceName, processors, sampler)
	case string(OTLPGrpc):
		blog.Info("Using otlpgrpc exporter")
		ctx := context.Background()
		otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if !ok {
			otelAgentAddr = "0.0.0.0:4317"
		}
		traceClient := otlpgrpctrace.NewClient(
			otlpgrpctrace.WithInsecure(),
			otlpgrpctrace.WithEndpoint(otelAgentAddr),
			otlpgrpctrace.WithDialOption(grpc.WithBlock()))
		grpcExporter, err := otlpgrpctrace.New(ctx, traceClient)
		handleErr(err, "failed to create otelgrpc exporter")
		processors := initProcessors(grpcExporter)
		return newTracerProvider(defaultOptions.ServiceName, processors, sampler)
	case string(OTLPHttp):
		blog.Info("Using otlphttp exporter")

	case string(Zipkin):
	}
	return &sdktrace.TracerProvider{}, nil
}

func newTracerProvider(serviceName string, processors []sdktrace.SpanProcessor, sampler sdktrace.Sampler) (*sdktrace.TracerProvider, error) {
	var tpos []sdktrace.TracerProviderOption
	for i := 0; i < len(processors); i++ {
		tpos = append(tpos, sdktrace.WithSpanProcessor(processors[i]))
	}
	tpos = append(tpos, utils.WithResource(resource.New(serviceName)), utils.WithSampler(sampler))
	tp := utils.NewTracerProvider(tpos...)
	utils.SetTracerProvider(tp)
	return tp, nil
}

// ValidateTracerProviderOption set a slice of TracerProviderOption based on the tracer provider configuration.
func ValidateTracerProviderOption(config *TracerProviderConfig) []TracerProviderOption {
	var tpos []TracerProviderOption
	switch {
	case config.TracingSwitch != "":
		tpos = append(tpos, TracerSwitch(config.TracingSwitch))
	case config.TracingType != "":
		tpos = append(tpos, TracerType(config.TracingType))
	case config.ServiceName != "":
		tpos = append(tpos, ServiceName(config.ServiceName))
	case config.ResourceAttrs != nil:
		tpos = append(tpos, ResourceAttrs(config.ResourceAttrs))
	case config.JaegerConfig.CollectorEndpointConfig != nil:
		switch {
		case config.JaegerConfig.CollectorEndpointConfig.CollectorEndpoint != "":
			tpos = append(tpos, JaegerCollectorEndpoint(config.JaegerConfig.CollectorEndpointConfig.CollectorEndpoint))
		case config.JaegerConfig.CollectorEndpointConfig.Username != "":
			tpos = append(tpos, JaegerCollectorUsername(config.JaegerConfig.CollectorEndpointConfig.Username))
		case config.JaegerConfig.CollectorEndpointConfig.Password != "":
			tpos = append(tpos, JaegerCollectorPassword(config.JaegerConfig.CollectorEndpointConfig.Password))
		case config.JaegerConfig.CollectorEndpointConfig.HttpClient != nil:
			tpos = append(tpos, JaegerCollectorHttpClient(config.JaegerConfig.CollectorEndpointConfig.HttpClient))
		}
	case config.GrpcConfig.Endpoint != "":
		tpos = append(tpos, WithGrpcEndpoint(config.GrpcConfig.Endpoint))
	case config.GrpcConfig.URLPath != "":
		tpos = append(tpos, WithGrpcURLPath(config.GrpcConfig.URLPath))
	case config.AlwaysOnSampler:
		tpos = append(tpos, WithAlwaysOnSampler())
	case config.AlwaysOffSampler:
		tpos = append(tpos, WithAlwaysOffSampler())
	case fmt.Sprint(config.RatioBasedSampler) != "0":
		tpos = append(tpos, WithRatioBasedSampler(config.RatioBasedSampler))
	case config.DefaultOnSampler:
		tpos = append(tpos, WithDefaultOnSampler())
	case config.DefaultOffSampler:
		tpos = append(tpos, WithDefaultOffSampler())
	}
	return tpos
}

// initProcessors sets processors for OTEL.
func initProcessors(exporter sdktrace.SpanExporter) (sps []sdktrace.SpanProcessor) {
	// By default, no processors are enabled. Depending on the data source, it may be recommended
	// that multiple processors be enabled. Processors must be enabled for every data source.
	// Always be sure to batch in production.
	sp := utils.NewBatchSpanProcessor(exporter)
	sps = append(sps, sp)
	return sps
}

func initSampler(tpc *TracerProviderConfig) sdktrace.Sampler {
	if tpc.AlwaysOnSampler {
		return sdktrace.AlwaysSample()
	}
	if tpc.AlwaysOffSampler {
		return sdktrace.NeverSample()
	}
	if fmt.Sprint(tpc.RatioBasedSampler) != "0" {
		return sdktrace.TraceIDRatioBased(tpc.RatioBasedSampler)
	}
	if tpc.DefaultOnSampler {
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
	if tpc.DefaultOffSampler {
		return sdktrace.ParentBased(sdktrace.NeverSample())
	}
	// will not sample if parent sampler was not set
	return sdktrace.ParentBased(sdktrace.NeverSample())
}

func initCollectorEndpoint(c *jaeger.CollectorEndpointConfig, op ...oteljaeger.CollectorEndpointOption) []oteljaeger.CollectorEndpointOption {
	if c.CollectorEndpoint != "" {
		op = append(op, oteljaeger.WithEndpoint(c.CollectorEndpoint))
	}
	if c.Username != "" {
		op = append(op, oteljaeger.WithUsername(c.Username))
	}
	if c.Password != "" {
		op = append(op, oteljaeger.WithPassword(c.Password))
	}
	if c.HttpClient != nil {
		op = append(op, oteljaeger.WithHTTPClient(c.HttpClient))
	}
	return op
}

func initAgentEndpoint(a *jaeger.AgentEndpointConfig, op ...oteljaeger.AgentEndpointOption) []oteljaeger.AgentEndpointOption {
	if a.Host != "" {
		op = append(op, oteljaeger.WithAgentHost(a.Host))
	}
	if a.Port != "" {
		op = append(op, oteljaeger.WithAgentPort(a.Port))
	}
	if a.MaxPacketSize != 0 {
		op = append(op, oteljaeger.WithMaxPacketSize(a.MaxPacketSize))
	}
	if a.Logger != nil {
		op = append(op, oteljaeger.WithLogger(a.Logger))
	}
	if !a.AttemptReconnecting {
		op = append(op, oteljaeger.WithDisableAttemptReconnecting())
	}
	if a.AttemptReconnectInterval != 0 {
		op = append(op, oteljaeger.WithAttemptReconnectingInterval(a.AttemptReconnectInterval))
	}
	return op
}

func validateTracingOptions(opt *TracerProviderConfig) error {
	err := validateTracingSwitch(opt.TracingSwitch)
	if err != nil {
		return err
	}

	err = validateTracingType(opt.TracingType)
	if err != nil {
		return err
	}

	err = validateServiceName(opt.ServiceName)
	if err != nil {
		return err
	}
	return nil
}

func validateTracingSwitch(s string) error {
	if s == "on" || s == "off" {
		return nil
	}
	return errSwitchType
}

func validateTracingType(t string) error {
	if t == string(Jaeger) || t == string(Zipkin) || t == "otlpgrpc" || t == "otlphttp" {
		return nil
	}
	return errTracingType
}

func validateServiceName(sn string) error {
	if sn == "" {
		return errServiceName
	}
	return nil
}

func validateJaegerEndpoint(op *TracerProviderConfig) error {
	if op.JaegerConfig.CollectorEndpointConfig == nil && op.JaegerConfig.AgentEndpointConfig == nil {
		return errors.New("neither a jaeger collector nor a jaeger agent endpoint is configured")
	}
	return nil
}

func handleErr(err error, message string) {
	if err != nil {
		blog.Errorf("%s: %v", message, err)
	}
}
