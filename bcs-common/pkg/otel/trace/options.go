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
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// TraceType tracing type
type TraceType string

const (
	// Jaeger show jaeger system
	Jaeger TraceType = "jaeger"
	// OTLPGrpc show otlpgrpc system
	OTLPGrpc TraceType = "otlpgrpc"
	// OTLPHttp show otlphttp system
	OTLPHttp TraceType = "otlphttp"
	// Zipkin show zipkin system
	Zipkin TraceType = "zipkin"
	// NullTrace show opentracing NoopTracer
	NullTrace TraceType = "null"
)

const (
	// DefaultCollectorPort is the port the Exporter will attempt connect to
	// if no collector port is provided.
	DefaultCollectorPort uint16 = 4317
	// DefaultCollectorHost is the host address the Exporter will attempt
	// connect to if no collector address is provided.
	DefaultCollectorHost string = "localhost"
	// DefaultTracesPath is a default URL path for endpoint that
	// receives spans.
	DefaultTracesPath string = "/v1/traces"
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
		o.TracingType = string(t)
	}
}

// ServiceName sets a service name for a tracing system
func ServiceName(sn string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.ServiceName = sn
	}
}

// JaegerCollectorEndpoint sets the endpoint url for tracing system
func JaegerCollectorEndpoint(ep string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.CollectorEndpointConfig.CollectorEndpoint = ep
	}
}

// JaegerCollectorUsername sets the username url for tracing system
func JaegerCollectorUsername(name string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.CollectorEndpointConfig.Username = name
	}
}

// JaegerCollectorPassword sets the password url for tracing system
func JaegerCollectorPassword(password string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.CollectorEndpointConfig.Password = password
	}
}

// JaegerCollectorHttpClient sets the http client for tracing system
func JaegerCollectorHttpClient(client *http.Client) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.CollectorEndpointConfig.HttpClient = client
	}
}

// JaegerAgentHost sets the jaeger agent host for tracing system
func JaegerAgentHost(host string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.Host = host
	}
}

// JaegerAgentPort sets the jaeger agent host for tracing system
func JaegerAgentPort(port string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.Port = port
	}
}

// JaegerAgentMaxPacketSize sets the jaeger agent max package size for tracing system
func JaegerAgentMaxPacketSize(size int) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.MaxPacketSize = size
	}
}

// JaegerAgentLogger sets the jaeger agent logger for tracing system
func JaegerAgentLogger(l *log.Logger) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.Logger = l
	}
}

// JaegerAgentDisableReconnecting disables reconnecting udp client
func JaegerAgentDisableReconnecting() TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.AttemptReconnecting = false
	}
}

// JaegerAgentReconnectInterval the interval between attempts to connect agent endpoint
func JaegerAgentReconnectInterval(t time.Duration) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.JaegerConfig.AgentEndpointConfig.AttemptReconnectInterval = t
	}
}

// WithGrpcEndpoint sets grpc endpoint
func WithGrpcEndpoint(endpoint string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.GrpcConfig.Endpoint = endpoint
	}
}

// WithGrpcURLPath sets grpc url path
func WithGrpcURLPath(path string) TracerProviderOption {
	return func(o *TracerProviderConfig) {
		o.GrpcConfig.URLPath = path
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
