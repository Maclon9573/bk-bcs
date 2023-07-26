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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BCSNetIPSpec defines the desired state of BCSNetIP
type BCSNetIPSpec struct {
	// Pool/Mask/Gateway 的信息冗余进来
	Pool    string `json:"pool,omitempty"`
	Mask    int    `json:"mask,omitempty"`
	Gateway string `json:"gateway,omitempty"`

	// 容器ID
	Container string `json:"container,omitempty"`
	// 对应主机信息
	Host string `json:"host"`

	// TODO: 不确定是否能拿到
	PodName      string `json:"podName,omitempty"`
	PodNamespace string `json:"podNamespace,omitempty"`
}

// BCSNetIPStatus defines the observed state of BCSNetIP
type BCSNetIPStatus struct {
	// Active --已使用，Available --可用, Reserved --保留
	Status     string `json:"status,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
	UpdateTime string `json:"updateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BCSNetIP is the Schema for the bcsnetips API
type BCSNetIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BCSNetIPSpec   `json:"spec,omitempty"`
	Status BCSNetIPStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BCSNetIPList contains a list of BCSNetIP
type BCSNetIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BCSNetIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BCSNetIP{}, &BCSNetIPList{})
}
