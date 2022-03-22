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
	"go.opentelemetry.io/otel/trace"

	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/utils"
	"github.com/emicklei/go-restful"
)

// SetHTTPSpanContextInfo set restful.Request context
func SetHTTPSpanContextInfo(req *restful.Request, handler string) trace.Span {
	ctx, span := utils.Tracer("").Start(req.Request.Context(), handler)
	defer span.End()
	//utils.HTTPHandlerKey.Set(span, handler)
	req.Request = req.Request.WithContext(ctx)

	return span
}
