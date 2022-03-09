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
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	oteljaeger "go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
)

// TraceType tracing type
type TraceType string

const (
	// Jaeger show jaeger system
	Jaeger TraceType = "jaeger"
	// OTLP_GRPC show otlpgrpc system
	OTLP_GRPC TraceType = "otlpgrpc"
	// OTLP_HTTP show otlphttp system
	OTLP_HTTP TraceType = "otlphttp"
	// Zipkin show zipkin system
	Zipkin TraceType = "zipkin"
)

const (
	// DefaultJaegerCollectorEndpoint sets default jaeger collector endpoint
	DefaultJaegerCollectorEndpoint = "http://localhost:14268/api/traces"
	// DefaultJaegerAgentEndpointHost sets default jaeger agent endpoint host
	DefaultJaegerAgentEndpointHost = "localhost"
	// DefaultJaegerAgentEndpointPort sets default jaeger agent endpoint host
	DefaultJaegerAgentEndpointPort = "6831"
	// DefaultOTLPCollectorPort is the port the Exporter will attempt connect to
	// if no collector port is provided.
	DefaultOTLPCollectorPort uint16 = 4317
	// DefaultOTLPCollectorHost is the host address the Exporter will attempt
	// connect to if no collector address is provided.
	DefaultOTLPCollectorHost string = "localhost"
	// OTELGrpcTracesPath is a default URL path for endpoint that
	// receives spans.
	OTELGrpcTracesPath string = "/v1/traces"
)

// TracerProviderOption for init TracerProviderConfig
type TracerProviderOption func(f *TracerProviderConfig)

// TracerSwitch sets a factory tracing switch: on or off
func TracerSwitch(s string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.TracingSwitch = s
	}
}

// TracerType sets a factory tracing type
func TracerType(t string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.TracingType = t
	}
}

// ServiceName sets a service name for a tracing system
func ServiceName(sn string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.ServiceName = sn
	}
}

// JaegerAgentHost sets the jaeger agent host for tracing system
func JaegerAgentHost(host string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerAgentHost = host
	}
}

// JaegerAgentPort sets the jaeger agent host for tracing system
func JaegerAgentPort(port string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerAgentPort = port
	}
}

// JaegerAgentOptions imports oteljaeger.AgentEndpointOption
func JaegerAgentOptions(option oteljaeger.AgentEndpointOption) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerAgentOptions = append(o.JaegerAgentOptions, option)
	}
}

// JaegerCollectorEndpoint sets the endpoint url for tracing system
func JaegerCollectorEndpoint(ep string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerColEndpoint = ep
	}
}

// JaegerCollectorUsername sets the username url for tracing system
func JaegerCollectorUsername(name string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerColUsername = name
	}
}

// JaegerCollectorPassword sets the password url for tracing system
func JaegerCollectorPassword(password string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerColPassword = password
	}
}

// JaegerCollectorHttpClient sets the http client for tracing system
func JaegerCollectorHttpClient(client *http.Client) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerColHttpClient = client
	}
}

// JaegerCollectorOptions imports oteljaeger.CollectorEndpointOption
func JaegerCollectorOptions(option oteljaeger.CollectorEndpointOption) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerColOptions = append(o.JaegerColOptions, option)
	}
}

// WithOTLPGRPCEndpoint sets OTLP GRPC endpoint
func WithOTLPGRPCEndpoint(endpoint string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.OTLPGRPCEndpoint = endpoint
	}
}

// WithOTLPGRPCInsecure sets OTLP GRPC endpoint
func WithOTLPGRPCInsecure() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.OTLPGRPCInsecure = true
	}
}

// WithGRPCOption imports otlptracegrpc.Option
func WithGRPCOption(option otlptracegrpc.Option) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.GRPCOptions = append(o.GRPCOptions, option)
	}
}

// WithOTLPHTTPEndpoint sets OTLP HTTP endpoint
func WithOTLPHTTPEndpoint(endpoint string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.OTLPHTTPEndpoint = endpoint
	}
}

// WithOTLPHTTPInsecure sets OTLP HTTP endpoint
func WithOTLPHTTPInsecure() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.OTLPHTTPInsecure = true
	}
}

// WithHTTPOption imports otlptracegrpc.Option
func WithHTTPOption(option otlptracehttp.Option) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.HTTPOptions = append(o.HTTPOptions, option)
	}
}

// WithResourceOption imports otelresource.Option
func WithResourceOption(option otelresource.Option) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.ResourceOptions = append(o.ResourceOptions, option)
	}
}

// ResourceAttrs sets resource attributes
func ResourceAttrs(ra []attribute.KeyValue) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.ResourceAttrs = append(o.ResourceAttrs, ra...)
	}
}

// WithAlwaysOnSampler sets a always on Sampler
func WithAlwaysOnSampler() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.AlwaysOnSampler = true
	}
}

// WithAlwaysOffSampler sets a always off Sampler
func WithAlwaysOffSampler() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.AlwaysOffSampler = true
	}
}

// WithRatioBasedSampler sets a ratio based Sampler
func WithRatioBasedSampler(r float64) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.RatioBasedSampler = r
	}
}

// WithDefaultOnSampler sets a default on Sampler if parent span is not sampled
func WithDefaultOnSampler() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.DefaultOnSampler = true
	}
}

// WithDefaultOffSampler sets a default off Sampler if parent span is not sampled
func WithDefaultOffSampler() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.DefaultOffSampler = true
	}
}
