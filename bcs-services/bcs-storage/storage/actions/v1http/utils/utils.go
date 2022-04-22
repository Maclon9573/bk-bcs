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

package utils

import (
	"context"
	"net/http"

	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/utils"
	"github.com/emicklei/go-restful"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SetHTTPSpanContextInfo set restful.Request context
func SetHTTPSpanContextInfo(req *restful.Request, handler string) trace.Span {
	ri := req.Request.Header.Get("X-Request-Id")
	ctx := context.WithValue(req.Request.Context(), "X-Request-Id", ri)
	ctx, span := utils.Tracer(handler).Start(ctx, handler)
	defer span.End()
	setHTTPSpanAttributes(span, req.Request, handler)
	req.Request = req.Request.WithContext(ctx)

	return span
}

func setHTTPSpanAttributes(span trace.Span, request *http.Request, handler string) {
	attrs := []attribute.KeyValue{}

	attrs = append(attrs, utils.HTTPHandlerKey.String(handler))
	if request.Method != "" {
		attrs = append(attrs, utils.HTTPMethodKey.String(request.Method))
	} else {
		attrs = append(attrs, utils.HTTPMethodKey.String(http.MethodGet))
	}

	// remove any username/password info that may be in the URL
	// before adding it to the attributes
	userinfo := request.URL.User
	request.URL.User = nil

	attrs = append(attrs, utils.HTTPURLKey.String(request.URL.String()))

	// restore any username/password info that was removed
	request.URL.User = userinfo

	if request.TLS != nil {
		attrs = append(attrs, utils.HTTPSchemeKey.String("http"))
	} else {
		attrs = append(attrs, utils.HTTPSchemeKey.String("https"))
	}

	if request.Host != "" {
		attrs = append(attrs, utils.HTTPHostKey.String(request.Host))
	}

	span.SetAttributes(attrs...)
}
