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

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/exporter/jaeger"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/exporter/otlp/otlpgrpctrace"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/exporter/otlp/otlphttptrace"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/resource"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/utils"

	"go.opentelemetry.io/otel/attribute"
	oteljaeger "go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
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
)

// TracerProviderConfig set TracerProviderConfig for different tracing systems
type TracerProviderConfig struct {
	TracingSwitch string `json:"tracingSwitch" value:"off" usage:"tracing switch"`
	TracingType   string `json:"tracingType" value:"jaeger" usage:"tracing type(default jaeger)"`
	ServiceName   string `json:"serviceName" value:"bcs-common/pkg/otel" usage:"tracing serviceName"`
	// Jaeger agent endpoint config
	JaegerAgentHost    string                           `json:"jaegerAgentHost,omitempty" value:"localhost" usage:"host to be used in the agent client endpoint"`
	JaegerAgentPort    string                           `json:"JaegerAgentPort,omitempty" value:"6831" usage:"port to be used in the agent client endpoint"`
	JaegerAgentOptions []oteljaeger.AgentEndpointOption `json:"-"`
	// Jaeger collector endpoint config
	JaegerColEndpoint   string                               `json:"jaegerColEndpoint,omitempty" usage:"jaegerCollectorEndpoint for sending spans directly to a collector"`
	JaegerColUsername   string                               `json:"jaegerUsername,omitempty" usage:"jaegerUsername to be used for authentication with the collector endpoint"`
	JaegerColPassword   string                               `json:"jaegerPassword,omitempty" usage:"jaegerPassword to be used for authentication with the collector endpoint"`
	JaegerColHttpClient *http.Client                         `json:"jaegerHttpClient,omitempty" usage:"jaegerHttpClient to be used to make requests to the collector endpoint"`
	JaegerColOptions    []oteljaeger.CollectorEndpointOption `json:"-"`
	// OTLP collector GRPC endpoint
	OTLPGRPCEndpoint string                 `json:"OTLPGRPCEndpoint,omitempty" usage:"OTLPGRPCEndpoint sets GRPC client endpoint"`
	OTLPGRPCInsecure bool                   `json:"OTLPGRPCSecure,omitempty" usage:"OTLPGRPCInsecure disables GRPC client transport security"`
	GRPCOptions      []otlptracegrpc.Option `json:"-"`
	// OTLP collector HTTP endpoint
	OTLPHTTPEndpoint string                 `json:"OTLPHTTPEndpoint,omitempty" usage:"OTLPHTTPEndpoint sets HTTP client endpoint"`
	OTLPHTTPInsecure bool                   `json:"OTLPHTTPInsecure,omitempty" usage:"OTLPHTTPInsecure disables HTTP client transport security"`
	HTTPOptions      []otlptracehttp.Option `json:"-"`
	// Resource attributes
	ResourceAttrs   []attribute.KeyValue  `json:"resourceAttrs,omitempty" usage:"attributes of traced service"`
	ResourceOptions []otelresource.Option `json:"-"`
	// Sampler kinds
	AlwaysOnSampler   bool    `json:"alwaysOnSampler,omitempty"`
	AlwaysOffSampler  bool    `json:"alwaysOffSampler,omitempty"`
	RatioBasedSampler float64 `json:"ratioBasedSampler,omitempty"`
	DefaultOnSampler  bool    `json:"defaultOnSampler,omitempty"`
	DefaultOffSampler bool    `json:"defaultOffSampler,omitempty"`
}

// InitTracerProvider initialize an OTLP tracer provider with processors and exporters.
func InitTracerProvider(serviceName string, options ...TracerProviderOption) (*sdktrace.TracerProvider, error) {
	defaultOptions := &TracerProviderConfig{
		TracingSwitch: defaultSwitchType,
		TracingType:   defaultTracingType,
		ServiceName:   serviceName,
	}

	for _, o := range options {
		o(defaultOptions)
	}

	err := validateTracingOptions(defaultOptions)
	if err != nil {
		blog.Errorf("validateTracingOptions failed: %v", err)
		return nil, err
	}

	if defaultOptions.TracingSwitch == "off" {
		return &sdktrace.TracerProvider{}, nil
	}
	resource := initResource(defaultOptions)
	sampler := initSampler(defaultOptions)

	switch defaultOptions.TracingType {
	case string(Jaeger):
		blog.Info("Creating jaeger exporter...")
		if defaultOptions.JaegerAgentHost != "" && defaultOptions.JaegerAgentPort != "" {
			opts := append(initAgentEndpointOptions(defaultOptions), defaultOptions.JaegerAgentOptions...)
			jaegerExporter, err := jaeger.NewAgentExporter(opts...)
			handleErr(err, "failed to create jaeger exporter")
			processors := initProcessors(jaegerExporter)
			return newTracerProvider(processors, resource, sampler)
		}
		if defaultOptions.JaegerColEndpoint != "" {
			opts := append(initCollectorEndpointOptions(defaultOptions), defaultOptions.JaegerColOptions...)
			jaegerExporter, err := jaeger.NewCollectorExporter(opts...)
			handleErr(err, "failed to create jaeger exporter")
			processors := initProcessors(jaegerExporter)
			return newTracerProvider(processors, resource, sampler)
		}
		blog.Info("Jaeger agent and collector are not set, trying default agent endpoint %s:%v first...",
			DefaultJaegerAgentEndpointHost, DefaultJaegerAgentEndpointPort)
		opts := append(initAgentEndpointOptions(defaultOptions), defaultOptions.JaegerAgentOptions...)
		jaegerExporter, err := jaeger.NewAgentExporter(opts...)
		if err != nil {
			blog.Info("failed to connect jaeger agent, trying default jaeger collector endpoint %s...",
				DefaultJaegerCollectorEndpoint)
			opts := append(initCollectorEndpointOptions(defaultOptions), defaultOptions.JaegerColOptions...)
			jaegerExporter, err = jaeger.NewCollectorExporter(opts...)
			handleErr(err, "failed to create jaeger exporter")
			processors := initProcessors(jaegerExporter)
			return newTracerProvider(processors, resource, sampler)
		}
		processors := initProcessors(jaegerExporter)
		return newTracerProvider(processors, resource, sampler)
	case string(OTLP_GRPC):
		blog.Info("Using otlpgrpc exporter...")
		opts := append(initGRPCConfigOptions(defaultOptions), defaultOptions.GRPCOptions...)
		if defaultOptions.OTLPGRPCEndpoint != "" {
			ctx := context.Background()
			traceClient := otlpgrpctrace.NewClient(opts...)
			grpcExporter, err := otlpgrpctrace.New(ctx, traceClient)
			handleErr(err, "failed to create otelgrpc exporter")
			processors := initProcessors(grpcExporter)
			return newTracerProvider(processors, resource, sampler)
		}
		blog.Info("Using default OTLPGrpc endpoint: %s:%v", DefaultOTLPCollectorHost,
			DefaultOTLPCollectorPort)
		ctx := context.Background()
		traceClient := otlpgrpctrace.NewClient(opts...)
		grpcExporter, err := otlpgrpctrace.New(ctx, traceClient)
		handleErr(err, "failed to create otelgrpc exporter")
		processors := initProcessors(grpcExporter)
		return newTracerProvider(processors, resource, sampler)
	case string(OTLP_HTTP):
		blog.Info("Using otlphttp exporter...")
		opts := append(initHTTPConfigOptions(defaultOptions), defaultOptions.HTTPOptions...)
		if defaultOptions.OTLPHTTPEndpoint != "" {
			ctx := context.Background()
			httpExporter, err := otlphttptrace.New(ctx, opts...)
			handleErr(err, "failed to create otlphttp exporter")
			processors := initProcessors(httpExporter)
			return newTracerProvider(processors, resource, sampler)
		}
		blog.Info("Using default OTLPHttp endpoint: %s:%v", DefaultOTLPCollectorHost,
			DefaultOTLPCollectorPort)
		ctx := context.Background()
		httpExporter, err := otlphttptrace.New(ctx, opts...)
		handleErr(err, "failed to create otlphttp exporter")
		processors := initProcessors(httpExporter)
		return newTracerProvider(processors, resource, sampler)
	case string(Zipkin):
	}
	return &sdktrace.TracerProvider{}, nil
}

func newTracerProvider(processors []sdktrace.SpanProcessor,
	resource *otelresource.Resource, sampler sdktrace.Sampler) (*sdktrace.TracerProvider, error) {
	var tpos []sdktrace.TracerProviderOption
	for i := 0; i < len(processors); i++ {
		tpos = append(tpos, utils.WithSpanProcessor(processors[i]))
	}
	tpos = append(tpos, utils.WithResource(resource), utils.WithSampler(sampler))
	tp := utils.NewTracerProvider(tpos...)
	utils.SetTracerProvider(tp)
	return tp, nil
}

// ValidateTracerProviderOption sets a slice of TracerProviderOption based on a tracer provider configuration.
func ValidateTracerProviderOption(config *TracerProviderConfig) []TracerProviderOption {
	var tpos []TracerProviderOption
	if config.TracingSwitch != "" {
		tpos = append(tpos, TracerSwitch(config.TracingSwitch))
	}
	if config.TracingType != "" {
		tpos = append(tpos, TracerType(config.TracingType))
	}
	if config.ServiceName != "" {
		tpos = append(tpos, ServiceName(config.ServiceName))
	}
	if config.JaegerColEndpoint != "" {
		tpos = append(tpos, JaegerCollectorEndpoint(config.JaegerColEndpoint))
	}
	if config.JaegerColUsername != "" {
		tpos = append(tpos, JaegerCollectorUsername(config.JaegerColUsername))
	}
	if config.JaegerColPassword != "" {
		tpos = append(tpos, JaegerCollectorPassword(config.JaegerColPassword))
	}
	if config.JaegerColHttpClient != nil {
		tpos = append(tpos, JaegerCollectorHttpClient(config.JaegerColHttpClient))
	}
	if config.JaegerAgentHost != "" {
		tpos = append(tpos, JaegerAgentHost(config.JaegerAgentHost))
	}
	if config.JaegerAgentPort != "" {
		tpos = append(tpos, JaegerAgentPort(config.JaegerAgentPort))
	}
	if config.OTLPGRPCEndpoint != "" {
		tpos = append(tpos, WithOTLPGRPCEndpoint(config.OTLPGRPCEndpoint))
	}
	if config.OTLPGRPCInsecure {
		tpos = append(tpos, WithOTLPGRPCInsecure())
	}
	if config.OTLPHTTPEndpoint != "" {
		tpos = append(tpos, WithOTLPHTTPEndpoint(config.OTLPGRPCEndpoint))
	}
	if config.OTLPHTTPInsecure {
		tpos = append(tpos, WithOTLPHTTPInsecure())
	}
	if config.ResourceAttrs != nil {
		tpos = append(tpos, ResourceAttrs(config.ResourceAttrs))
	}
	if config.AlwaysOnSampler {
		tpos = append(tpos, WithAlwaysOnSampler())
	}
	if config.AlwaysOffSampler {
		tpos = append(tpos, WithAlwaysOffSampler())
	}
	if fmt.Sprint(config.RatioBasedSampler) != "0" {
		tpos = append(tpos, WithRatioBasedSampler(config.RatioBasedSampler))
	}
	if config.DefaultOnSampler {
		tpos = append(tpos, WithDefaultOnSampler())
	}
	if config.DefaultOffSampler {
		tpos = append(tpos, WithDefaultOffSampler())
	}
	return tpos
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
	return sdktrace.ParentBased(sdktrace.AlwaysSample())
}

func initResource(tpc *TracerProviderConfig) *otelresource.Resource {
	ctx := context.Background()
	tpc.ResourceOptions = append(tpc.ResourceOptions,
		otelresource.WithSchemaURL(semconv.SchemaURL),
		otelresource.WithAttributes(resource.ServiceNameKey.String(tpc.ServiceName)))
	r, err := otelresource.New(ctx, tpc.ResourceOptions...)
	handleErr(err, "failed to create resource")
	if tpc.ResourceAttrs != nil {
		for _, a := range tpc.ResourceAttrs {
			r, _ = otelresource.Merge(r, otelresource.NewSchemaless(a))
		}
	}
	return r
}

func initCollectorEndpointOptions(config *TracerProviderConfig) []oteljaeger.CollectorEndpointOption {
	var op []oteljaeger.CollectorEndpointOption
	if config.JaegerColEndpoint != "" {
		op = append(op, oteljaeger.WithEndpoint(config.JaegerColEndpoint))
	}
	if config.JaegerColUsername != "" {
		op = append(op, oteljaeger.WithUsername(config.JaegerColUsername))
	}
	if config.JaegerColPassword != "" {
		op = append(op, oteljaeger.WithPassword(config.JaegerColPassword))
	}
	if config.JaegerColHttpClient != nil {
		op = append(op, oteljaeger.WithHTTPClient(config.JaegerColHttpClient))
	}
	return op
}

func initAgentEndpointOptions(config *TracerProviderConfig) []oteljaeger.AgentEndpointOption {
	var op []oteljaeger.AgentEndpointOption
	if config.JaegerAgentHost != "" {
		op = append(op, oteljaeger.WithAgentHost(config.JaegerAgentHost))
	}
	if config.JaegerAgentPort != "" {
		op = append(op, oteljaeger.WithAgentPort(config.JaegerAgentPort))
	}
	return op
}

func initGRPCConfigOptions(config *TracerProviderConfig) []otlptracegrpc.Option {
	var op []otlptracegrpc.Option
	if config.OTLPGRPCEndpoint != "" {
		op = append(op, otlptracegrpc.WithEndpoint(config.OTLPGRPCEndpoint))
	}
	if config.OTLPGRPCInsecure {
		op = append(op, otlptracegrpc.WithInsecure())
	}
	return op
}

func initHTTPConfigOptions(config *TracerProviderConfig) []otlptracehttp.Option {
	var op []otlptracehttp.Option
	if config.OTLPHTTPEndpoint != "" {
		op = append(op, otlptracehttp.WithEndpoint(config.OTLPHTTPEndpoint))
	}
	if config.OTLPHTTPInsecure {
		op = append(op, otlptracehttp.WithInsecure())
	}
	return op
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
	if t == string(Jaeger) || t == string(Zipkin) || t == string(OTLP_GRPC) || t == string(OTLP_HTTP) {
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

func handleErr(err error, message string) {
	if err != nil {
		blog.Errorf("%s: %v", message, err)
	}
}
