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

package otlpgrpctrace

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/otlp"
)

// New constructs a new Exporter and starts it.
func New(ctx context.Context, opts ...otlptracegrpc.Option) (*otlp.Exporter, error) {
	return otlp.New(ctx, NewClient(opts...))
}

// NewUnstarted constructs a new Exporter and does not start it.
func NewUnstarted(opts ...otlptracegrpc.Option) *otlp.Exporter {
	return otlp.NewUnstarted(NewClient(opts...))
}
