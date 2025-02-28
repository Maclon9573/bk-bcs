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
 */

// Package main xxx
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/utils"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

func welcomePage(w http.ResponseWriter, r *http.Request) {
	_, span := utils.Tracer("demo-http-tracer").Start(r.Context(), "WelcomePage")
	log.Printf("traceID:%v, spanID:%v",
		span.SpanContext().TraceID().String(), span.SpanContext().SpanID().String())
	defer span.End()
	w.Write([]byte("Welcome to my website!\n"))
}

func main() {
	opts := trace.Options{
		TracingSwitch: "on",
		ExporterURL:   "http://localhost:14268/api/traces",
		ResourceAttrs: []attribute.KeyValue{
			attribute.String("endpoint", "http_server"),
		},
	}
	op := []trace.Option{}
	op = append(op, trace.TracerSwitch(opts.TracingSwitch))
	op = append(op, trace.ResourceAttrs(opts.ResourceAttrs))
	op = append(op, trace.ExporterURL(opts.ExporterURL))

	err := initTracingProvider("demo-http-server", op...)
	if err != nil {
		log.Fatal(err)
	}
	wrappedHandler := otelhttp.NewHandler(http.HandlerFunc(welcomePage), "/")
	http.Handle("/", wrappedHandler)
	log.Fatal(http.ListenAndServe("localhost:9090", nil))
}

func initTracingProvider(serviceName string, opt ...trace.Option) error {
	ctx := context.Background()

	tp, err := trace.InitTracerProvider(serviceName, opt...)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	if err != nil {
		return fmt.Errorf("failed to initialize an OTLP tracer provider: %s", err.Error())
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	return tp.Shutdown(ctx)
}
