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
	"errors"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteljaeger "go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/jaeger"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/resource"
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
	// defaultCollectorEndpoint sets default collector endpoint
	defaultCollectorEndpoint = "http://localhost:14268/api/traces"
)

// Options set options for different tracing systems
type Options struct {
	TracingSwitch string `json:"tracingSwitch" value:"off" usage:"tracing switch"`
	TracingType   string `json:"tracingType" value:"jaeger" usage:"tracing type(default jaeger)"`
	ServiceName   string `json:"serviceName" value:"bcs-common/pkg/otel" usage:"tracing serviceName"`
	// Jaeger exporter endpoint
	JaegerConfig *jaeger.EndpointConfig `json:"jaegerConfig"`

	ResourceAttrs []attribute.KeyValue `json:"resourceAttrs" value:"" usage:"attributes of traced service"`

	// Sampler type
	AlwaysOnSampler   bool
	AlwaysOffSampler  bool
	RatioBasedSampler float64
	DefaultOnSampler  bool
	DefaultOffSampler bool
	// ParentBasedSampler bool
}

// Option for init Options
type Option func(f *Options)

// InitTracerProvider initialize an OTLP tracer provider
func InitTracerProvider(serviceName string, opt ...Option) (*sdktrace.TracerProvider, error) {
	defaultOptions := &Options{
		TracingSwitch: defaultSwitchType,
		TracingType:   defaultTracingType,
		ServiceName:   serviceName,
	}

	for _, o := range opt {
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
	switch defaultOptions.TracingType {
	case "jaeger":
		err := validateJaegerEndpoint(defaultOptions)
		if err != nil {
			blog.Errorf("failed to create jaeger exporter:", err.Error())
			return nil, err
		}
		blog.Info("Using jaeger exporter")
		if defaultOptions.JaegerConfig.CollectorEndpointConfig != nil {
			defaultOptions = &Options{
				TracingSwitch: "on",
				TracingType:   "jaeger",
				ServiceName:   serviceName,
				JaegerConfig: &jaeger.EndpointConfig{
					CollectorEndpointConfig: &jaeger.CollectorEndpointConfig{
						CollectorEndpoint: "http://localhost:14268/api/traces",
					},
				},
			}
			opts := initCollectorEndpoint(defaultOptions.JaegerConfig.CollectorEndpointConfig)
			jaegerExporter, err := jaeger.New(jaeger.WithCollectorEndpoint(opts...))
			if err != nil {
				blog.Errorf("failed to create jaeger exporter:", err.Error())
				return nil, err
			}
			sampler := initSampler(defaultOptions)
			return newTracerProvider(jaegerExporter, defaultOptions.ServiceName, sampler)
		} else {
			opts := initAgentEndpoint(defaultOptions.JaegerConfig.AgentEndpointConfig)
			jaegerExporter, err := jaeger.New(jaeger.WithAgentEndpoint(opts...))
			if err != nil {
				blog.Errorf("failed to create jaeger exporter:", err.Error())
				return nil, err
			}
			sampler := initSampler(defaultOptions)
			return newTracerProvider(jaegerExporter, defaultOptions.ServiceName, sampler)
		}
	case "otlpgrpc":
		blog.Info("Using otlpgrpc exporter")

	case "otlphttp":
		blog.Info("Using otlphttp exporter")

	case "zipkin":
	}
	return &sdktrace.TracerProvider{}, nil
}

func newTracerProvider(exporter sdktrace.SpanExporter, serviceName string, sampler sdktrace.Sampler) (*sdktrace.TracerProvider, error) {
	processor := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		// Always be sure to batch in production.
		sdktrace.WithSpanProcessor(processor),
		// Record information about this application in an Resource.
		sdktrace.WithResource(resource.New(serviceName)),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func validateTracingOptions(opt *Options) error {
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

func validateJaegerEndpoint(op *Options) error {
	switch {
	case op.JaegerConfig.CollectorEndpointConfig == nil && op.JaegerConfig.AgentEndpointConfig == nil:
		return errors.New("neither a jaeger collector nor a jaeger agent endpoint is configured")
	case op.JaegerConfig.CollectorEndpointConfig != nil && op.JaegerConfig.AgentEndpointConfig != nil:
		return errors.New("a jaeger collector can't be configured with a jaeger agent endpoint at the same time")
	}
	return nil
}

func initSampler(op *Options) sdktrace.Sampler {
	if op.AlwaysOnSampler {
		return sdktrace.AlwaysSample()
	}
	if op.AlwaysOffSampler {
		return sdktrace.NeverSample()
	}
	if strconv.Itoa(int(op.RatioBasedSampler)) != "" {
		return sdktrace.TraceIDRatioBased(op.RatioBasedSampler)
	}
	if op.DefaultOnSampler {
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
	if op.DefaultOffSampler {
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
