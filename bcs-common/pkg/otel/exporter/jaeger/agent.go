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

package jaeger

import (
	"log"
	"time"
)

type AgentEndpointConfig struct {
	Host                     string        `json:"host" value:"localhost" usage:"host to be used in the agent client endpoint"`
	Port                     string        `json:"port" value:"6831" usage:"port to be used in the agent client endpoint"`
	MaxPacketSize            int           `json:"maxPacketSize" usage:"maxPacketSize for transport to the Jaeger agent"`
	Logger                   *log.Logger   `json:"logger" usage:"logger to be used by agent client"`
	AttemptReconnecting      bool          `json:"attemptReconnecting" value:"false" usage:"attemptReconnecting disables reconnecting udp client"`
	AttemptReconnectInterval time.Duration `json:"attemptReconnectInterval" usage:"attemptReconnectInterval sets the interval between attempts to connect agent endpoint"`
}
