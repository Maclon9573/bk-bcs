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

package main

import (
	"runtime/debug"

	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/cmd"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/clustercheck"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/dnscheck"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/eventrecorder"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/logrecorder"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/masterpodcheck"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/netcheck"
	_ "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-reporter/internal/plugin/systemappcheck"

	"k8s.io/klog"
)

func main() {
	// See SetMemoryLimit for more details.
	debug.SetGCPercent(100)

	cmd.Execute()
	// Flush flushes all pending log I/O.
	defer klog.Flush()
}
