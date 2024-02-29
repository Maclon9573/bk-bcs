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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/clusterops"
	"strconv"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/common/deepcopy"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/odm/drivers"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/odm/operator"
	corev1 "k8s.io/api/core/v1"

	cmproto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/common"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/lock"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/remote/cmdb"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/store"
	storeopt "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/store/options"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/taskserver"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/utils"
)

// ReimportAction action for reimport cluster
type ReimportAction struct {
	ctx        context.Context
	model      store.ClusterManagerModel
	locker     lock.DistributedLock
	cloud      *cmproto.Cloud
	task       *cmproto.Task
	cluster    *cmproto.Cluster
	oriCluster *cmproto.Cluster
	req        *cmproto.ReimportClusterReq
	resp       *cmproto.ReimportClusterResp
}

// NewReimportAction reimport cluster action
func NewReimportAction(model store.ClusterManagerModel, locker lock.DistributedLock) *ReimportAction {
	return &ReimportAction{
		model:  model,
		locker: locker,
	}
}

func (ra *ReimportAction) constructCluster() (*cmproto.Cluster, error) {
	createTime := time.Now().Format(time.RFC3339)
	cls := new(cmproto.Cluster)
	err := deepcopy.DeepCopy(cls, ra.oriCluster)
	if err != nil {
		return nil, err
	}
	cls.ClusterID = ra.req.TargetClusterID
	cls.ProjectID = ra.req.TargetProjectID
	cls.BusinessID = ra.req.TargetBusinessID
	cls.Updater = ra.req.Updater
	cls.CreateTime = createTime
	cls.UpdateTime = createTime

	return cls, nil
}

func (ra *ReimportAction) syncClusterInfoToDB(cls *cmproto.Cluster) error {
	// generate ClusterID
	err := ra.generateClusterID(cls)
	if err != nil {
		return err
	}

	// update imported cluster status
	cls.Status = common.StatusInitialization
	ra.cluster = cls

	err = ra.model.CreateCluster(ra.ctx, cls)
	if err != nil {
		blog.Errorf("save Cluster %s information to store failed, %s", cls.ClusterID, err.Error())
		if errors.Is(err, drivers.ErrTableRecordDuplicateKey) {
			return err
		}
		return err
	}

	ra.oriCluster.Status = common.StatusMigrating
	err = ra.model.UpdateCluster(ra.ctx, ra.oriCluster)
	if err != nil {
		blog.Errorf("save Cluster %s information to store failed, %s", ra.oriCluster.ClusterID, err.Error())
		return err
	}

	return nil
}

func (ra *ReimportAction) setResp(code uint32, msg string) {
	ra.resp.Code = code
	ra.resp.Message = msg
	ra.resp.Result = (code == common.BcsErrClusterManagerSuccess)
	ra.setResponseData(ra.resp.Result)
}

func (ra *ReimportAction) setResponseData(result bool) {
	if !result {
		return
	}

	ra.cluster.KubeConfig = ""
	respData := map[string]interface{}{
		"cluster": ra.cluster,
		"task":    ra.task,
	}
	data, err := utils.MapToProtobufStruct(respData)
	if err != nil {
		blog.Errorf("ReimportAction[%s] trans Data failed: %v", ra.cluster.ClusterID, err)
		return
	}
	ra.resp.Data = data
}

// Handle create cluster request
func (ra *ReimportAction) Handle(ctx context.Context, req *cmproto.ReimportClusterReq, resp *cmproto.ReimportClusterResp) {
	if req == nil || resp == nil {
		blog.Errorf("import cluster failed, req or resp is empty")
		return
	}
	ra.ctx = ctx
	ra.req = req
	ra.resp = resp

	// parameters check
	if err := ra.validate(); err != nil {
		ra.setResp(common.BcsErrClusterManagerInvalidParameter, err.Error())
		return
	}

	if err := ra.getCloudInfo(ctx); err != nil {
		blog.Errorf("get cluster %s relative Cloud failed, %s", req.ClusterID, err.Error())
		ra.setResp(common.BcsErrClusterManagerDBOperation, err.Error())
		return
	}

	ra.locker.Lock(createClusterIDLockKey, []lock.LockOption{lock.LockTTL(time.Second * 10)}...) // nolint
	defer ra.locker.Unlock(createClusterIDLockKey)                                               // nolint

	// init cluster and set cloud default info
	cls, err := ra.constructCluster()
	if err != nil {
		blog.Errorf("constructCluster cluster failed, %s", err.Error())
		ra.setResp(common.BcsErrClusterManagerDBOperation, err.Error())
	}

	// create cluster save to mongoDB
	err = ra.syncClusterInfoToDB(cls)
	if err != nil {
		ra.setResp(common.BcsErrClusterManagerDBOperation, err.Error())
		return
	}

	// generate cluster task and dispatch it
	err = ra.reimportClusterTask(ctx, cls)
	if err != nil {
		return
	}
	blog.Infof("create cluster[%s] task cloud[%s] provider[%s] successfully",
		cls.ClusterName, ra.cloud.CloudID, ra.cloud.CloudProvider)

	// import cluster info to extra system
	importClusterExtraOperation(cls)

	// build operationLog
	err = ra.model.CreateOperationLog(ctx, &cmproto.OperationLog{
		ResourceType: common.Cluster.String(),
		ResourceID:   cls.ClusterID,
		TaskID:       ra.task.TaskID,
		Message:      fmt.Sprintf("导入%s集群%s", cls.Provider, cls.ClusterID),
		OpUser:       cls.Creator,
		CreateTime:   time.Now().String(),
		ClusterID:    ra.cluster.ClusterID,
		ProjectID:    ra.cluster.ProjectID,
	})
	if err != nil {
		blog.Errorf("import cluster[%s] CreateOperationLog failed: %v", cls.ClusterID, err)
	}

	ra.setResp(common.BcsErrClusterManagerSuccess, common.BcsErrClusterManagerSuccessStr)
}

func (ra *ReimportAction) validate() error {
	if ra.req.ProjectID == ra.req.TargetProjectID {
		return fmt.Errorf("projectID and targetProjectID can not be same")
	}

	if err := ra.getOriginClusterInfo(); err != nil {
		return err
	}

	if ra.req.ProjectID != ra.oriCluster.ProjectID {
		return fmt.Errorf("wrong projectID %s", ra.req.ProjectID)
	}

	if ra.req.TargetProjectID == ra.oriCluster.ProjectID {
		return fmt.Errorf("cluster[%s] is already in project %s", ra.req.ClusterID, ra.req.TargetProjectID)
	}

	if err := ra.req.Validate(); err != nil {
		return err
	}

	return nil
}

func (ra *ReimportAction) getOriginClusterInfo() error {
	cluster, err := ra.model.GetCluster(ra.ctx, ra.req.ClusterID)
	if err != nil {
		blog.Errorf("get cluster %s in store failed, %s", ra.oriCluster.ClusterID, err.Error())
		return err
	}
	ra.oriCluster = cluster

	return nil
}

func (ra *ReimportAction) getCloudInfo(ctx context.Context) error {
	cloud, err := ra.model.GetCloud(ctx, ra.oriCluster.Provider)
	if err != nil {
		blog.Errorf("get cluster %s relative Cloud %s failed, %s",
			ra.oriCluster.ClusterID, ra.oriCluster.Provider, err.Error())
		return err
	}
	ra.cloud = cloud

	return nil
}

func (ra *ReimportAction) reimportClusterTask(ctx context.Context, cls *cmproto.Cluster) error {
	// call cloud provider cluster_manager feature to reimport cluster task
	provider, err := cloudprovider.GetClusterMgr(ra.cloud.CloudProvider)
	if err != nil {
		blog.Errorf("get cluster %s relative cloud provider %s failed, %s",
			ra.req.ClusterID, ra.cloud.CloudProvider, err.Error())
		ra.setResp(common.BcsErrClusterManagerCloudProviderErr, err.Error())
		return err
	}

	// first, get cloud credentialInfo from project; second, get from cloud provider when failed to obtain
	coption, err := cloudprovider.GetCredential(&cloudprovider.CredentialData{
		Cloud:     ra.cloud,
		AccountID: cls.CloudAccountID,
	})
	if err != nil {
		blog.Errorf("Get Credential failed from Cloud %s: %s",
			ra.cloud.CloudID, err.Error())
		ra.setResp(common.BcsErrClusterManagerCloudProviderErr, err.Error())
		return err
	}
	coption.Region = cls.Region

	// create cluster task by task manager
	task, err := provider.ReimportCluster(cls, &cloudprovider.ReimportClusterOption{
		CommonOption:    *coption,
		Cloud:           ra.cloud,
		OriginClusterID: ra.oriCluster.ClusterID,
		Operator:        cls.Updater,
	})
	if err != nil {
		blog.Errorf("migrate Cluster %s by Cloud %s with provider %s failed, %s",
			ra.req.ClusterID, ra.cloud.CloudID, ra.cloud.CloudProvider, err.Error())
		ra.setResp(common.BcsErrClusterManagerCloudProviderErr, err.Error())
		return err
	}

	// create task and dispatch task
	if err := ra.model.CreateTask(ctx, task); err != nil {
		blog.Errorf("save create cluster task for cluster %s failed, %s",
			cls.ClusterName, err.Error(),
		)
		ra.setResp(common.BcsErrClusterManagerDBOperation, err.Error())
		return err
	}
	if err := taskserver.GetTaskServer().Dispatch(task); err != nil {
		blog.Errorf("dispatch create cluster task for cluster %s failed, %s",
			cls.ClusterName, err.Error(),
		)
		ra.setResp(common.BcsErrClusterManagerTaskErr, err.Error())
		return err
	}

	ra.task = task
	return nil
}

func (ra *ReimportAction) generateClusterID(cls *cmproto.Cluster) error {
	if cls.ClusterID == "" {
		clusterID, clusterNum, err := generateClusterID(cls, ra.model)
		if err != nil {
			blog.Errorf("generate clusterId failed when import cluster")
			return err
		}

		blog.Infof("generate clusterID[%v:%s] successful when impport cluster", clusterNum, clusterID)
		cls.ClusterID = clusterID
	}

	return nil
}

// ReimportClusterValidateAction action for reimport cluster validate
type ReimportClusterValidateAction struct {
	ctx   context.Context
	model store.ClusterManagerModel
	nodes []*cmproto.ClusterNode
	k8sOp *clusterops.K8SOperator
	req   *cmproto.ReimportClusterValidateReq
	resp  *cmproto.ReimportClusterValidateResp
}

// NewReimportClusterValidateAction reimport cluster action validate
func NewReimportClusterValidateAction(model store.ClusterManagerModel, k8sOp *clusterops.K8SOperator) *ReimportClusterValidateAction {
	return &ReimportClusterValidateAction{
		model: model,
		k8sOp: k8sOp,
	}
}

func (ra *ReimportClusterValidateAction) Handle(ctx context.Context, req *cmproto.ReimportClusterValidateReq,
	resp *cmproto.ReimportClusterValidateResp) {
	ra.ctx = ctx
	ra.req = req
	ra.resp = resp

	if err := ra.req.Validate(); err != nil {
		ra.setResp(err.Error())
		return
	}

	if err := ra.getClusterNodes(); err != nil {
		ra.setResp(err.Error())
		return
	}

	bizID, _ := strconv.Atoi(req.TargetBusinessID)
	if err := ra.hostExistInBiz(bizID); err != nil {
		ra.setResp(err.Error())
		return
	}

	blog.Infof("ReimportClusterValidateAction run successfully")
	ra.setResp(common.BcsErrClusterManagerSuccessStr)
}

func (ra *ReimportClusterValidateAction) setResp(msg string) {
	ra.resp.Message = msg
	if msg == common.BcsErrClusterManagerSuccessStr {
		ra.resp.Result = true
	} else {
		ra.resp.Result = false
	}
}

func (ra *ReimportClusterValidateAction) hostExistInBiz(bizID int) error {
	hostData, err := cmdb.GetCmdbClient().FetchAllHostsByBizID(bizID, false)
	if err != nil {
		return fmt.Errorf("fetch cmdb hosts failed, %v", err)
	}

	nonExistIPs := make([]string, 0)
	for _, node := range ra.nodes {
		exist := false
		for _, h := range hostData {
			if (node.InnerIP != "" && node.InnerIP == h.BKHostInnerIP) ||
				(node.InnerIPv6 != "" && node.InnerIPv6 == h.BKHostInnerIPV6) {
				exist = true
				blog.Infof("host %s[%s] exists in biz %d", node.InnerIP, node.InnerIPv6, bizID)
				break
			}
		}
		blog.Infof("host %s exist %v", node.InnerIP, exist)
		if !exist {
			if node.InnerIP != "" {
				nonExistIPs = append(nonExistIPs, node.InnerIP)
			} else {
				nonExistIPs = append(nonExistIPs, node.InnerIPv6)
			}
		}
	}

	if len(nonExistIPs) > 0 {
		return fmt.Errorf("hosts %v not migrated to target business", nonExistIPs)
	}

	return nil
}

func (ra *ReimportClusterValidateAction) getClusterNodes() error {
	condM := make(operator.M)
	condM["clusterid"] = ra.req.ClusterID

	cond := operator.NewLeafCondition(operator.Eq, condM)
	nodes, err := ra.model.ListNode(ra.ctx, cond, &storeopt.ListOption{})
	if err != nil {
		blog.Errorf("list nodes in cluster %s failed, %s", ra.req.ClusterID, err.Error())
		return err
	}
	cmNodes := make([]*cmproto.ClusterNode, 0)
	for i := range nodes {
		cmNodes = append(cmNodes, transNodeToClusterNode(ra.model, nodes[i]))
	}

	k8sNodes, err := ra.getK8sNodes(cmNodes)
	if err != nil {
		return err
	}

	// get cluster nodes
	ra.nodes = mergeClusterNodes(ra.req.ClusterID, cmNodes, k8sNodes)

	return nil
}

func (ra *ReimportClusterValidateAction) getK8sNodes(cmNodes []*cmproto.ClusterNode) ([]*corev1.Node, error) {
	if len(cmNodes) == 0 {
		blog.Infof("cluster[%s] nodes not exist in storage", ra.req.ClusterID)
		return nil, fmt.Errorf("cluster[%s] nodes not exist in storage", ra.req.ClusterID)
	}

	k8sNodes, err := ra.k8sOp.ListClusterNodes(ra.ctx, ra.req.ClusterID)
	if err != nil {
		blog.Warnf("ListClusterNodes[%s] failed, %s", ra.req.ClusterID, err.Error())
		return nil, fmt.Errorf("ListClusterNodes[%s] failed, %s", ra.req.ClusterID, err.Error())
	}

	return k8sNodes, nil
}
