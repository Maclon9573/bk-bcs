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
	"go.opentelemetry.io/otel/attribute"
	"log"
	"net/http"
	"time"
)

// TraceType tracing type
type TraceType string

const (
	// Jaeger show jaeger system
	Jaeger TraceType = "jaeger"
	// Zipkin show zipkin system
	Zipkin TraceType = "zipkin"
	// NullTrace show opentracing NoopTracer
	NullTrace TraceType = "null"
)

// TracerSwitch sets a factory tracing switch: on or off
func TracerSwitch(s string) Option {
	return func(o *Options) {
		o.TracingSwitch = s
	}
}

// TracerType sets a factory tracing type
func TracerType(t string) Option {
	return func(o *Options) {
		o.TracingType = string(t)
	}
}

// ServiceName sets a service name for a tracing system
func ServiceName(sn string) Option {
	return func(o *Options) {
		o.ServiceName = sn
	}
}

// JaegerCollectorEndpoint sets the Endpoint url for tracing system
func JaegerCollectorEndpoint(ep string) Option {
	return func(o *Options) {
		o.JaegerConfig.CollectorEndpointConfig.CollectorEndpoint = ep
	}
}

// JaegerCollectorUsername sets the username url for tracing system
func JaegerCollectorUsername(name string) Option {
	return func(o *Options) {
		o.JaegerConfig.CollectorEndpointConfig.Username = name
	}
}

// JaegerCollectorPassword sets the password url for tracing system
func JaegerCollectorPassword(password string) Option {
	return func(o *Options) {
		o.JaegerConfig.CollectorEndpointConfig.Password = password
	}
}

// JaegerCollectorHttpClient sets the http client for tracing system
func JaegerCollectorHttpClient(client *http.Client) Option {
	return func(o *Options) {
		o.JaegerConfig.CollectorEndpointConfig.HttpClient = client
	}
}

// JaegerAgentHost sets the jaeger agent host for tracing system
func JaegerAgentHost(host string) Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.Host = host
	}
}

// JaegerAgentPort sets the jaeger agent host for tracing system
func JaegerAgentPort(port string) Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.Port = port
	}
}

// JaegerAgentMaxPacketSize sets the jaeger agent max package size for tracing system
func JaegerAgentMaxPacketSize(size int) Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.MaxPacketSize = size
	}
}

// JaegerAgentLogger sets the jaeger agent logger for tracing system
func JaegerAgentLogger(l *log.Logger) Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.Logger = l
	}
}

// JaegerAgentDisableReconnecting disables reconnecting udp client
func JaegerAgentDisableReconnecting() Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.AttemptReconnecting = false
	}
}

// JaegerAgentReconnectInterval the interval between attempts to connect agent endpoint
func JaegerAgentReconnectInterval(t time.Duration) Option {
	return func(o *Options) {
		o.JaegerConfig.AgentEndpointConfig.AttemptReconnectInterval = t
	}
}

// ResourceAttrs sets resource attributes
func ResourceAttrs(ra []attribute.KeyValue) Option {
	return func(o *Options) {
		o.ResourceAttrs = append(o.ResourceAttrs, ra...)
	}
}

// WithAlwaysOnSampler sets a always on Sampler
func WithAlwaysOnSampler(s bool) Option {
	return func(o *Options) {
		o.AlwaysOnSampler = true
	}
}

// WithAlwaysOffSampler sets a always off Sampler
func WithAlwaysOffSampler(s bool) Option {
	return func(o *Options) {
		o.AlwaysOffSampler = true
	}
}

// todo
// WithParentBasedSampler sets a parent based Sampler
//func WithParentBasedSampler(s bool) Option {
//	return func(o *Options) {
//		o.ParentBasedSampler = true
//	}
//}

// WithRatioBasedSampler sets a ratio based Sampler
func WithRatioBasedSampler(r float64) Option {
	return func(o *Options) {
		o.RatioBasedSampler = r
	}
}

// WithDefaultOnSampler sets a default on Sampler if parent span is not sampled
func WithDefaultOnSampler(s bool) Option {
	return func(o *Options) {
		o.DefaultOnSampler = true
	}
}

// WithDefaultOffSampler sets a default off Sampler if parent span is not sampled
func WithDefaultOffSampler(s bool) Option {
	return func(o *Options) {
		o.DefaultOffSampler = true
	}
}
