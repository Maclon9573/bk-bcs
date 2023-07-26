/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BCSNetPoolSpec defines the desired state of BCSNetPool
type BCSNetPoolSpec struct {
	// 网段掩码
	Mask int `json:"mask,omitempty"`
	// 网段网关
	Gateway string `json:"gateway,omitempty"`
	// 对应主机列表
	Hosts []string `json:"hosts,omitempty"`
	// 可用 IP
	AvailableIPs []string `json:"availableIPs,omitempty"`
}

// BCSNetPoolStatus defines the observed state of BCSNetPool
type BCSNetPoolStatus struct {
	// Init --初始化中，Normal --正常
	Status         string `json:"status,omitempty"`
	CreateTime     string `json:"createTime,omitempty"`
	UpdateTime     string `json:"updateTime,omitempty"`
	AvailableIPNum int    `json:"availableIPNum,omitempty"`
	// 已使用IP
	ActiveIPs   []string `json:"activeIPs,omitempty"`
	ActiveIPNum int      `json:"activeIPNum,omitempty"`
	// 保留的IP
	ReservedIPs   []string `json:"reservedIPs,omitempty"`
	ReservedIPNum int      `json:"reservedIPNum,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BCSNetPool is the Schema for the bcsnetpools API
type BCSNetPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BCSNetPoolSpec   `json:"spec,omitempty"`
	Status BCSNetPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BCSNetPoolList contains a list of BCSNetPool
type BCSNetPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BCSNetPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BCSNetPool{}, &BCSNetPoolList{})
}
