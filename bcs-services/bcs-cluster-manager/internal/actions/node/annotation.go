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

// Package node xxx
package node

import (
	"context"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"

	cmproto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/clusterops"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/common"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/store"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/utils"
)

// UpdateNodeAnnotationsAction action for update node annotations
type UpdateNodeAnnotationsAction struct {
	ctx   context.Context
	model store.ClusterManagerModel
	req   *cmproto.UpdateNodeAnnotationsRequest
	resp  *cmproto.UpdateNodeAnnotationsResponse
	k8sOp *clusterops.K8SOperator
}

// NewUpdateNodeAnnotationsAction create action
func NewUpdateNodeAnnotationsAction(model store.ClusterManagerModel,
	k8sOp *clusterops.K8SOperator) *UpdateNodeAnnotationsAction {
	return &UpdateNodeAnnotationsAction{
		model: model,
		k8sOp: k8sOp,
	}
}

func (ua *UpdateNodeAnnotationsAction) validate() error {
	if err := ua.req.Validate(); err != nil {
		return err
	}

	return nil
}

func (ua *UpdateNodeAnnotationsAction) updateNodeAnnotations() error { // nolint
	successCh := make(chan *cmproto.NodeOperationStatusInfo, len(ua.req.Nodes))
	failCh := make(chan *cmproto.NodeOperationStatusInfo, len(ua.req.Nodes))

	barrier := utils.NewRoutinePool(50)
	defer barrier.Close()

	for i := range ua.req.Nodes {
		barrier.Add(1)
		go func(node *cmproto.NodeAnnotation) {
			defer barrier.Done()

			ctx, cancel := context.WithTimeout(context.Background(), clusterops.DefaultTimeout)
			defer cancel()

			err := ua.k8sOp.UpdateNodeAnnotations(ctx, ua.req.ClusterID, node.NodeName, node.Annotations, false)
			if err != nil {
				failCh <- &cmproto.NodeOperationStatusInfo{NodeName: node.NodeName, Message: err.Error()}
				blog.Errorf("updateNodeAnnotations[%s] failed in cluster %s, err %s", node.NodeName, ua.req.ClusterID,
					err.Error())
				return
			}

			successCh <- &cmproto.NodeOperationStatusInfo{NodeName: node.NodeName}
		}(ua.req.Nodes[i])
	}
	barrier.Wait()
	close(successCh)
	close(failCh)

	ua.resp.Data = &cmproto.NodeOperationStatus{
		Success: make([]*cmproto.NodeOperationStatusInfo, 0),
		Fail:    make([]*cmproto.NodeOperationStatusInfo, 0),
	}
	for v := range successCh {
		ua.resp.Data.Success = append(ua.resp.Data.Success, v)
	}
	for v := range failCh {
		ua.resp.Data.Fail = append(ua.resp.Data.Fail, v)
	}
	return nil
}

func (ua *UpdateNodeAnnotationsAction) setResp(code uint32, msg string) {
	ua.resp.Code = code
	ua.resp.Message = msg
	ua.resp.Result = (code == common.BcsErrClusterManagerSuccess)
}

// Handle handles update node labels
func (ua *UpdateNodeAnnotationsAction) Handle(ctx context.Context, req *cmproto.UpdateNodeAnnotationsRequest,
	resp *cmproto.UpdateNodeAnnotationsResponse) {
	if req == nil || resp == nil {
		blog.Errorf("update node annotations failed, req or resp is empty")
		return
	}
	ua.ctx = ctx
	ua.req = req
	ua.resp = resp

	if err := ua.validate(); err != nil {
		ua.setResp(common.BcsErrClusterManagerInvalidParameter, err.Error())
		return
	}

	if err := ua.updateNodeAnnotations(); err != nil {
		ua.setResp(common.BcsErrClusterManagerDBOperation, err.Error())
		return
	}

	ua.setResp(common.BcsErrClusterManagerSuccess, common.BcsErrClusterManagerSuccessStr)
}
