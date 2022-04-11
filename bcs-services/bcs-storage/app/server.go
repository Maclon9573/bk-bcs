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

package app

import (
	"context"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common"
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-storage/app/options"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-storage/storage"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName = "bcs-storage"
)

// Run the bcs-storage
func Run(op *options.StorageOptions) error {
	setConfig(op)

	// init tracing
	traceOpts := trace.ValidateTracerProviderOption(&op.Tracing)
	traceOpts = append(traceOpts, trace.WithIDGenerator(trace.NewCustomIDGenerator()))
	ctx, tp, err := trace.InitTracerProvider(op.Tracing.ServiceName, traceOpts...)
	if err != nil {
		blog.Error("failed to create tracer provider. err:%s", err.Error())
	}
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			blog.Error("failed to shutdown the span processors. err:%s", err.Error())
		}
	}(ctx)

	storage, err := bcsstorage.NewStorageServer(op)
	if err != nil {
		blog.Error("fail to create storage server. err:%s", err.Error())
		return err
	}

	if err := common.SavePid(op.ProcessConfig); err != nil {
		blog.Warn("fail to save pid. err:%s", err.Error())
	}

	return storage.Start()
}

func setConfig(op *options.StorageOptions) {
	op.ServerCert.CertFile = op.ServerCertFile
	op.ServerCert.KeyFile = op.ServerKeyFile
	op.ServerCert.CAFile = op.CAFile

	if op.ServerCert.CertFile != "" && op.ServerCert.KeyFile != "" {
		op.ServerCert.IsSSL = true
	}
}
