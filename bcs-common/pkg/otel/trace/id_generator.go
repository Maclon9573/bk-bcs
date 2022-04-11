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
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"math/rand"
	"net/http"
	"reflect"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type CustomIDGenerator struct {
	sync.Mutex
	requestID  string
	RandSource *rand.Rand
}

var _ sdktrace.IDGenerator = &CustomIDGenerator{}

// NewSpanID returns a non-zero span ID from a Customly-chosen sequence.
func (gen *CustomIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	gen.Lock()
	defer gen.Unlock()
	sid := trace.SpanID{}
	gen.RandSource.Read(sid[:])
	return sid
}

// NewIDs returns a non-zero trace ID and a non-zero span ID from a
// Customly-chosen sequence.
func (gen *CustomIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	gen.Lock()
	defer gen.Unlock()
	tid := trace.TraceID{}
	if gen.requestID != "" {
		var err error
		tid, err = trace.TraceIDFromHex(gen.requestID)
		if err != nil {
			blog.Error("failed to create trace id from request id. err:", err.Error())
		}
	} else {
		gen.RandSource.Read(tid[:])
	}
	sid := trace.SpanID{}
	gen.RandSource.Read(sid[:])
	return tid, sid
}

func NewCustomIDGenerator(req http.Request) (sdktrace.IDGenerator, error) {
	gen := &CustomIDGenerator{}
	requestID := "request_id"
	traceID := reflect.ValueOf(req.Context().Value(requestID)).String()
	if traceID != "" {
		if _, err := trace.TraceIDFromHex(traceID); err == nil {
			gen.requestID = requestID
			return gen, nil
		} else {
			return gen, err
		}
	}
	return gen, nil
}
