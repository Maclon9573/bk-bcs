/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package constant

import "time"

const (
	// InitializingStatus for BCSNetPool Initializing status
	InitializingStatus = "Initializing"
	// NormalStatus for BCSNetPool Normal status
	NormalStatus = "Normal"
	// TerminatingStatus for BCSNetPool Terminating status
	TerminatingStatus = "Terminating"

	// ActiveStatus for BCSNetIP Active status
	ActiveStatus = "Active"
	// AvailableStatus for BCSNetIP Available status
	AvailableStatus = "Available"
	// ReservedStatus for BCSNetIP Reserved status
	ReservedStatus = "Reserved"

	// PodAnnotationKeyFixIP pod annotation key for fix ip
	PodAnnotationKeyFixIP = "netservicecontroller.bkbcs.tencent.com"
	// PodAnnotationValueFixIP pod annotation value for fix ip
	PodAnnotationValueFixIP = "fixed-ip"
	// PodAnnotationKeyForExpiredDurationSeconds pod annotation key for fix ip keep duration
	PodAnnotationKeyForExpiredDurationSeconds = "keepduration.netservicecontroller.bkbcs.tencent.com"
	// PodAnnotationKeyForPrimaryKey pod annotation key for primary key
	PodAnnotationKeyForPrimaryKey = "primarykey.netservicecontroller.bkbcs.tencent.com"

	// FixIPLabel label key for fix ip
	FixIPLabel = "fixed-ip"

	// DefaultFixedIPKeepDurationStr string of default fixed ip keep time
	DefaultFixedIPKeepDurationStr = "48h"
	// MaxFixedIPKeepDuration max keep duration for fixed ip
	MaxFixedIPKeepDuration = 500 * time.Hour

	// FinalizerNameBcsNetserviceController finalizer name of bcs netservice controller
	FinalizerNameBcsNetserviceController = "netservicecontroller.bkbcs.tencent.com"
)
