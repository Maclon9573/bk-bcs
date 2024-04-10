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

package tasks

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"

	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/actions"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/azure/api"
	providerutils "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/utils"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/common"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/remote/loop"
)

// CreateAKSClusterTask call azure interface to create cluster
func CreateAKSClusterTask(taskID string, stepName string) error {
	start := time.Now()

	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("CreateAKSClusterTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("CreateAKSClusterTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]
	nodeGroupIDs := step.Params[cloudprovider.NodeGroupIDKey.String()]

	// get dependent basic info
	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("CreateAKSClusterTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error()) // nolint
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	nodeGroups := make([]*proto.NodeGroup, 0)
	for _, ngID := range strings.Split(nodeGroupIDs, ",") {
		nodeGroup, errGet := actions.GetNodeGroupByGroupID(cloudprovider.GetStorageModel(), ngID)
		if errGet != nil {
			blog.Errorf("CreateAKSClusterTask[%s]: GetNodeGroupByGroupID for cluster %s in task %s "+
				"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
			retErr := fmt.Errorf("get nodegroup information failed, %s", err.Error())
			_ = state.UpdateStepFailure(start, stepName, retErr)
			return retErr
		}
		nodeGroups = append(nodeGroups, nodeGroup)
	}

	// inject taskID
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)

	// create cluster task
	clsId, err := createAKSCluster(ctx, dependInfo, nodeGroups)
	if err != nil {
		blog.Errorf("CreateAKSClusterTask[%s] createAKSCluster for cluster[%s] failed, %s",
			taskID, clusterID, err.Error())
		retErr := fmt.Errorf("createAKSCluster err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)

		_ = cloudprovider.UpdateClusterErrMessage(clusterID, fmt.Sprintf("submit createCluster[%s] failed: %v",
			dependInfo.Cluster.GetClusterID(), err))
		return retErr
	}

	// update response information to task common params
	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}
	state.Task.CommonParams[cloudprovider.CloudSystemID.String()] = clsId

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CreateAKSClusterTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}
	return nil
}

func createAKSCluster(ctx context.Context, info *cloudprovider.CloudDependBasicInfo, groups []*proto.NodeGroup) (string, error) {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	req, err := generateCreateClusterRequest(info, groups)
	if err != nil {
		return "", err
	}

	client, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
	if err != nil {
		return "", fmt.Errorf("create AksService failed")
	}

	rgName, ok := info.Cluster.ExtraInfo[common.ClusterResourceGroup]
	if !ok {
		return "", fmt.Errorf("createAKSCluster[%s] %s failed, empty clusterResourceGroup",
			taskID, info.Cluster.ClusterID)
	}

	aksCluster, err := client.CreateCluster(ctx, rgName, info.Cluster.ClusterName, *req)
	if err != nil {
		blog.Errorf("createAKSCluster aks client CreateCluster failed, %v", err)
		return "", err
	}

	info.Cluster.SystemID = *aksCluster.Name
	info.Cluster.ExtraInfo[common.NodeResourceGroup] = *aksCluster.Properties.NodeResourceGroup
	err = cloudprovider.UpdateCluster(info.Cluster)
	if err != nil {
		blog.Errorf("createAKSCluster[%s] updateClusterSystemID[%s] failed %s",
			taskID, info.Cluster.ClusterID, err.Error())
		retErr := fmt.Errorf("call createAKSCluster updateClusterSystemID[%s] api err: %s",
			info.Cluster.ClusterID, err.Error())
		return "", retErr
	}
	blog.Infof("createAKSCluster[%s] call createAKSCluster UpdateClusterSystemID successful", taskID)

	return *aksCluster.Name, nil
}

func generateCreateClusterRequest(info *cloudprovider.CloudDependBasicInfo, groups []*proto.NodeGroup) (
	*armcontainerservice.ManagedCluster, error) {
	cluster := info.Cluster
	if cluster.NetworkSettings == nil {
		return nil, fmt.Errorf("generateCreateClusterRequest empty NetworkSettings for cluster %s", cluster.ClusterID)
	}

	var adminUserName string
	agentPools := make([]*armcontainerservice.ManagedClusterAgentPoolProfile, 0)
	for _, ng := range groups {
		if ng.LaunchTemplate == nil {
			return nil, fmt.Errorf("generateCreateClusterRequest empty LaunchTemplate for nodegroup %s", ng.Name)
		}
		adminUserName = ng.LaunchTemplate.InitLoginUsername
		sysDiskSize, _ := strconv.Atoi(ng.LaunchTemplate.SystemDisk.DiskSize)
		agentPools = append(agentPools, &armcontainerservice.ManagedClusterAgentPoolProfile{
			AvailabilityZones: func(zones []string) []*string {
				az := make([]*string, 0)
				for _, v := range zones {
					az = append(az, to.Ptr(v))
				}
				return az
			}(ng.AutoScaling.Zones),
			Count: to.Ptr(int32(ng.AutoScaling.DesiredSize)),
			EnableNodePublicIP: to.Ptr(func(group *proto.NodeGroup) bool {
				if ng.LaunchTemplate.InternetAccess != nil {
					return ng.LaunchTemplate.InternetAccess.PublicIPAssigned
				}
				return false
			}(ng)),
			Mode:          to.Ptr(armcontainerservice.AgentPoolMode(ng.NodeGroupType)),
			Name:          to.Ptr(ng.Name),
			OSDiskSizeGB:  to.Ptr(int32(sysDiskSize)),
			OSType:        to.Ptr(armcontainerservice.OSTypeLinux),
			ScaleDownMode: to.Ptr(armcontainerservice.ScaleDownModeDelete),
			Type:          to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
			VMSize:        to.Ptr(ng.LaunchTemplate.InstanceType),
		})
	}
	req := &armcontainerservice.ManagedCluster{
		Location: to.Ptr(cluster.Region),
		Name:     to.Ptr(cluster.ClusterName),
		Tags: func() map[string]*string {
			tags := make(map[string]*string)
			for k, v := range cluster.ClusterBasicSettings.ClusterTags {
				tags[k] = to.Ptr(v)
			}
			return tags
		}(),
		Properties: &armcontainerservice.ManagedClusterProperties{
			AgentPoolProfiles: agentPools,
			KubernetesVersion: to.Ptr(cluster.ClusterBasicSettings.Version),
			LinuxProfile: &armcontainerservice.LinuxProfile{
				AdminUsername: to.Ptr(adminUserName),
			},
			NetworkProfile: &armcontainerservice.NetworkProfile{
				ServiceCidr:  to.Ptr(cluster.NetworkSettings.ServiceIPv4CIDR),
				ServiceCidrs: []*string{to.Ptr(cluster.NetworkSettings.ServiceIPv4CIDR)},
			},
		},
	}

	return req, nil
}

// CheckAKSClusterStatusTask check cluster create status
func CheckAKSClusterStatusTask(taskID string, stepName string) error {
	start := time.Now()
	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("CheckAKSClusterStatusTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("CheckAKSClusterStatusTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]

	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("CheckAKSClusterStatusTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// check cluster status
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	err = checkClusterStatus(ctx, dependInfo)
	if err != nil {
		blog.Errorf("CheckAKSClusterStatusTask[%s] checkClusterStatus[%s] failed: %v",
			taskID, clusterID, err)
		retErr := fmt.Errorf("checkClusterStatus[%s] timeout|abnormal", clusterID)
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CheckAKSClusterStatusTask[%s] task %s %s update to storage fatal",
			taskID, taskID, stepName)
		return err
	}

	return nil
}

// checkClusterStatus check cluster status
func checkClusterStatus(ctx context.Context, info *cloudprovider.CloudDependBasicInfo) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	// get azureCloud client
	cli, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] get aks client failed: %s", taskID, err.Error())
		retErr := fmt.Errorf("get cloud aks client err, %s", err.Error())
		return retErr
	}

	var (
		failed = false
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// loop cluster status
	err = loop.LoopDoFunc(ctx, func() error {
		cluster, errGet := cli.GetCluster(ctx, info, info.Cluster.ExtraInfo[common.ClusterResourceGroup])
		if errGet != nil {
			blog.Errorf("checkClusterStatus[%s] failed: %v", taskID, errGet)
			return nil
		}

		blog.Infof("checkClusterStatus[%s] cluster[%s] current status[%s]", taskID,
			info.Cluster.ClusterID, cluster.Properties.ProvisioningState)

		switch *cluster.Properties.ProvisioningState {
		case api.ManagedClusterPodIdentityProvisioningStateSucceeded:
			return loop.EndLoop
		case api.ManagedClusterPodIdentityProvisioningStateFailed:
			failed = true
			return loop.EndLoop
		}

		return nil
	}, loop.LoopInterval(10*time.Second))
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] cluster[%s] failed: %v", taskID, info.Cluster.ClusterID, err)
		return err
	}

	if failed {
		blog.Errorf("checkClusterStatus[%s] GetCluster[%s] failed: abnormal", taskID, info.Cluster.ClusterID)
		retErr := fmt.Errorf("cluster[%s] status abnormal", info.Cluster.ClusterID)
		return retErr
	}

	return nil
}

// CheckAKSNodeGroupsStatusTask check cluster nodes status
func CheckAKSNodeGroupsStatusTask(taskID string, stepName string) error {
	start := time.Now()
	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("CheckAKSNodeGroupsStatusTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("CheckAKSNodeGroupsStatusTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]
	nodeGroupIDs := cloudprovider.ParseNodeIpOrIdFromCommonMap(step.Params,
		cloudprovider.NodeGroupIDKey.String(), ",")
	systemID := state.Task.CommonParams[cloudprovider.CloudSystemID.String()]

	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("CheckAKSNodeGroupsStatusTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// check cluster status
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	addSuccessNodeGroups, addFailureNodeGroups, err := checkNodesGroupStatus(ctx, dependInfo, systemID, nodeGroupIDs)
	if err != nil {
		blog.Errorf("CheckAKSNodeGroupsStatusTask[%s] checkNodesGroupStatus[%s] failed: %v",
			taskID, clusterID, err)
		retErr := fmt.Errorf("CheckAKSNodeGroupsStatusTask[%s] timeout|abnormal", clusterID)
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update response information to task common params
	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}
	if len(addFailureNodeGroups) > 0 {
		state.Task.CommonParams[cloudprovider.FailedNodeGroupIDsKey.String()] =
			strings.Join(addFailureNodeGroups, ",")
	}

	if len(addSuccessNodeGroups) == 0 {
		blog.Errorf("CheckAKSNodeGroupsStatusTask[%s] nodegroups init failed", taskID)
		retErr := fmt.Errorf("节点池初始化失败, 请联系管理员")
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	state.Task.CommonParams[cloudprovider.SuccessNodeGroupIDsKey.String()] =
		strings.Join(addSuccessNodeGroups, ",")

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CheckAKSNodeGroupsStatusTask[%s] task %s %s update to storage fatal",
			taskID, taskID, stepName)
		return err
	}

	return nil
}

func checkNodesGroupStatus(ctx context.Context, info *cloudprovider.CloudDependBasicInfo,
	systemID string, nodeGroupIDs []string) ([]string, []string, error) {

	var (
		addSuccessNodeGroups = make([]string, 0)
		addFailureNodeGroups = make([]string, 0)
	)

	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	nodeGroups := make([]*proto.NodeGroup, 0)
	for _, ngID := range nodeGroupIDs {
		nodeGroup, err := actions.GetNodeGroupByGroupID(cloudprovider.GetStorageModel(), ngID)
		if err != nil {
			return nil, nil, fmt.Errorf("checkNodesGroupStatus GetNodeGroupByGroupID failed, %s", err.Error())
		}
		nodeGroups = append(nodeGroups, nodeGroup)
	}
	// get azureCloud client
	cli, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
	if err != nil {
		blog.Errorf("checkNodesGroupStatus[%s] get aks client failed: %s", taskID, err.Error())
		retErr := fmt.Errorf("get cloud aks client err, %s", err.Error())
		return nil, nil, retErr
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	running, failure := make([]string, 0), make([]string, 0)
	// loop cluster status
	err = loop.LoopDoFunc(ctx, func() error {
		index := 0
		for _, ng := range nodeGroups {
			aksAgentPool, errQuery := cli.GetPoolAndReturn(ctx, info.Cluster.ExtraInfo[common.ClusterResourceGroup],
				systemID, ng.Name)
			if errQuery != nil {
				blog.Errorf("checkNodesGroupStatus[%s] failed: %v", taskID, err)
				return nil
			}
			if aksAgentPool == nil {
				blog.Errorf("checkNodesGroupStatus[%s] GetPoolAndReturn[%s] not found", taskID, ng.NodeGroupID)
				return nil
			}

			blog.Infof("checkNodesGroupStatus[%s] nodeGroup[%s] status %s",
				taskID, ng.NodeGroupID, *aksAgentPool.Properties.ProvisioningState)

			switch *aksAgentPool.Properties.ProvisioningState {
			case api.AgentPoolPodIdentityProvisioningStateSucceeded:
				running = append(running, ng.NodeGroupID)
				index++
			case api.AgentPoolPodIdentityProvisioningStateFailed:
				failure = append(failure, ng.NodeGroupID)
				index++
			}
		}
		if index == len(nodeGroups) {
			addSuccessNodeGroups = running
			addFailureNodeGroups = failure
			return loop.EndLoop
		}

		return nil
	}, loop.LoopInterval(10*time.Second))
	if err != nil {
		blog.Errorf("checkNodesGroupStatus[%s] cluster[%s] failed: %v", taskID, info.Cluster.ClusterID, err)
		return nil, nil, err
	}

	blog.Infof("checkNodesGroupStatus[%s] success[%v] failure[%v]",
		taskID, addSuccessNodeGroups, addFailureNodeGroups)

	return addSuccessNodeGroups, addFailureNodeGroups, nil
}

// UpdateAKSNodesGroupToDBTask update AKS nodepool
func UpdateAKSNodesGroupToDBTask(taskID string, stepName string) error {
	start := time.Now()
	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("UpdateAKSNodesGroupToDBTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("UpdateAKSNodesGroupToDBTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]
	addSuccessNodeGroupIDs := cloudprovider.ParseNodeIpOrIdFromCommonMap(state.Task.CommonParams,
		cloudprovider.SuccessNodeGroupIDsKey.String(), ",")
	addFailedNodeGroupIDs := cloudprovider.ParseNodeIpOrIdFromCommonMap(state.Task.CommonParams,
		cloudprovider.FailedNodeGroupIDsKey.String(), ",")

	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("UpdateAKSNodesGroupToDBTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// check cluster status
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	err = updateNodeGroups(ctx, dependInfo, addFailedNodeGroupIDs, addSuccessNodeGroupIDs)
	if err != nil {
		blog.Errorf("UpdateAKSNodesGroupToDBTask[%s] checkNodesGroupStatus[%s] failed: %v",
			taskID, clusterID, err)
		retErr := fmt.Errorf("UpdateAKSNodesGroupToDBTask[%s] timeout|abnormal", clusterID)
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("UpdateAKSNodesGroupToDBTask[%s] task %s %s update to storage fatal",
			taskID, taskID, stepName)
		return err
	}

	return nil
}

func updateNodeGroups(ctx context.Context, info *cloudprovider.CloudDependBasicInfo,
	addFailedNodeGroupIDs, addSuccessNodeGroupIDs []string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	if len(addFailedNodeGroupIDs) > 0 {
		for _, ngID := range addFailedNodeGroupIDs {
			err := cloudprovider.UpdateNodeGroupStatus(ngID, common.StatusCreateNodeGroupFailed)
			if err != nil {
				return fmt.Errorf("updateNodeGroups updateNodeGroupStatus[%s] failed, %v", ngID, err)
			}
		}
	}

	for _, ngID := range addSuccessNodeGroupIDs {
		nodeGroup, err := actions.GetNodeGroupByGroupID(cloudprovider.GetStorageModel(), ngID)
		if err != nil {
			return fmt.Errorf("updateNodeGroups GetNodeGroupByGroupID failed, %s", err.Error())
		}

		// get azureCloud client
		cli, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
		if err != nil {
			blog.Errorf("updateNodeGroups[%s] get aks client failed: %s", taskID, err.Error())
			return fmt.Errorf("get cloud aks client err, %s", err.Error())
		}

		aksAgentPool, err := cli.GetPoolAndReturn(ctx, info.Cluster.ExtraInfo[common.ClusterResourceGroup],
			info.Cluster.SystemID, nodeGroup.Name)
		if err != nil {
			blog.Errorf("updateNodeGroups[%s] ListNodePool failed: %v", taskID, err)
			return nil
		}

		nodeGroup.CloudNodeGroupID = *aksAgentPool.Name
		nodeGroup.Status = common.StatusRunning
		set, err := cli.MatchNodeGroup(ctx, info.Cluster.ExtraInfo[common.NodeResourceGroup], nodeGroup.CloudNodeGroupID)
		if err != nil {
			return fmt.Errorf("checkNodeGroup[%s]: call MatchNodeGroup[%s][%s] falied, %v", taskID,
				info.Cluster.ClusterID, nodeGroup.CloudNodeGroupID, err)
		}
		// 字段对齐
		_ = cli.SetToNodeGroup(set, nodeGroup)
		_ = cli.AgentPoolToNodeGroup(aksAgentPool, nodeGroup)

		err = cloudprovider.GetStorageModel().UpdateNodeGroup(context.Background(), nodeGroup)
		if err != nil {
			return fmt.Errorf("updateNodeGroups UpdateNodeGroup[%s] failed, %s", nodeGroup.Name, err.Error())
		}
	}

	return nil
}

// CheckAKSClusterNodesStatusTask check cluster nodes status
func CheckAKSClusterNodesStatusTask(taskID string, stepName string) error {
	start := time.Now()
	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("CheckAKSClusterNodesStatusTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("CheckAKSClusterNodesStatusTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]
	nodeGroupIDs := cloudprovider.ParseNodeIpOrIdFromCommonMap(state.Task.CommonParams,
		cloudprovider.SuccessNodeGroupIDsKey.String(), ",")

	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("CheckAKSClusterNodesStatusTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// check cluster status
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	addSuccessNodes, addFailureNodes, err := checkClusterNodesStatus(ctx, dependInfo, nodeGroupIDs)
	if err != nil {
		blog.Errorf("CheckAKSClusterStatusTask[%s] checkClusterStatus[%s] failed: %v",
			taskID, clusterID, err)
		retErr := fmt.Errorf("checkClusterStatus[%s] timeout|abnormal", clusterID)
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update response information to task common params
	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}
	if len(addFailureNodes) > 0 {
		state.Task.CommonParams[cloudprovider.FailedClusterNodeIDsKey.String()] = strings.Join(addFailureNodes, ",")
	}
	if len(addSuccessNodes) == 0 {
		blog.Errorf("CheckCreateClusterNodeStatusTask[%s] nodes init failed", taskID)
		retErr := fmt.Errorf("节点初始化失败, 请联系管理员")
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	state.Task.CommonParams[cloudprovider.SuccessClusterNodeIDsKey.String()] = strings.Join(addSuccessNodes, ",")

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CheckCreateClusterNodeStatusTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}

	return nil
}

func checkClusterNodesStatus(ctx context.Context, info *cloudprovider.CloudDependBasicInfo, nodeGroupIDs []string) (
	[]string, []string, error) {
	var (
		totalNodesNum   uint32
		addSuccessNodes = make([]string, 0)
		addFailureNodes = make([]string, 0)
	)

	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	nodePoolList := make([]string, 0)
	for _, ngID := range nodeGroupIDs {
		nodeGroup, err := actions.GetNodeGroupByGroupID(cloudprovider.GetStorageModel(), ngID)
		if err != nil {
			return nil, nil, fmt.Errorf("get nodegroup information failed, %s", err.Error())
		}
		totalNodesNum += nodeGroup.AutoScaling.DesiredSize
		nodePoolList = append(nodePoolList, nodeGroup.CloudNodeGroupID)
	}

	// get azureCloud client
	cli, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
	if err != nil {
		blog.Errorf("checkClusterNodesStatus[%s] get aks client failed: %s", taskID, err.Error())
		return nil, nil, fmt.Errorf("get cloud aks client err, %s", err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// loop cluster status
	err = loop.LoopDoFunc(ctx, func() error {
		index := 0
		running, failure := make([]string, 0), make([]string, 0)
		for _, pool := range nodePoolList {
			instances, errGet := cli.ListInstanceAndReturn(ctx, info.Cluster.ExtraInfo[common.NodeResourceGroup],
				fmt.Sprintf("%s-%s", pool, "vmss"))
			if errGet != nil {
				blog.Errorf("checkClusterNodesStatus[%s] failed: %v", taskID, errGet)
				return nil
			}

			blog.Infof("checkClusterNodesStatus[%s] nodeGroup[%s], current instances %d ",
				taskID, pool, len(instances))

			for _, instance := range instances {
				blog.Infof("checkClusterNodesStatus[%s] node[%s] state %s", taskID, *instance.Name,
					*instance.Properties.ProvisioningState)
				switch *instance.Properties.ProvisioningState {
				case api.VMProvisioningStateSucceeded:
					index++
					running = append(running, *instance.Name)
				case api.VMProvisioningStateFailed:
					failure = append(failure, *instance.Name)
					index++
				}
			}
		}

		if index == int(totalNodesNum) {
			addSuccessNodes = running
			addFailureNodes = failure
			return loop.EndLoop
		}

		return nil
	}, loop.LoopInterval(10*time.Second))
	// other error
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		blog.Errorf("checkClusterNodesStatus[%s] ListNodes failed: %v", taskID, err)
		return nil, nil, err
	}
	// timeout error
	if errors.Is(err, context.DeadlineExceeded) {
		blog.Errorf("checkClusterNodesStatus[%s] ListNodes failed: %v", taskID, err)

		running, failure := make([]string, 0), make([]string, 0)
		for _, pool := range nodePoolList {
			instances, errGet := cli.ListInstanceAndReturn(ctx, info.Cluster.ExtraInfo[common.NodeResourceGroup],
				fmt.Sprintf("%s-%s", pool, "vmss"))
			if errGet != nil {
				blog.Errorf("checkClusterNodesStatus[%s] failed: %v", taskID, errGet)
				return nil, nil, errGet
			}

			for _, instance := range instances {
				blog.Infof("checkClusterNodesStatus[%s] node[%s] state %s", taskID, *instance.Name,
					*instance.Properties.ProvisioningState)
				switch *instance.Properties.ProvisioningState {
				case api.VMProvisioningStateSucceeded:
					running = append(running, *instance.Name)
				default:
					failure = append(failure, *instance.Name)
				}
			}
		}

		addSuccessNodes = running
		addFailureNodes = failure
	}
	blog.Infof("checkClusterNodesStatus[%s] success[%v] failure[%v]", taskID, addSuccessNodes, addFailureNodes)

	// set cluster node status
	for _, n := range addFailureNodes {
		err = cloudprovider.UpdateNodeStatus(false, n, common.StatusAddNodesFailed)
		if err != nil {
			blog.Errorf("checkClusterNodesStatus[%s] UpdateNodeStatus[%s] failed: %v", taskID, n, err)
		}
	}

	return addSuccessNodes, addFailureNodes, nil
}

// UpdateAKSNodesToDBTask update AKS nodes
func UpdateAKSNodesToDBTask(taskID string, stepName string) error {
	start := time.Now()
	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("UpdateNodesToDBTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("UpdateNodesToDBTask[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]
	nodes := cloudprovider.ParseNodeIpOrIdFromCommonMap(state.Task.CommonParams,
		cloudprovider.SuccessClusterNodeIDsKey.String(), ",")
	nodeGroupIDs := cloudprovider.ParseNodeIpOrIdFromCommonMap(state.Task.CommonParams,
		cloudprovider.SuccessNodeGroupIDsKey.String(), ",")

	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("UpdateNodesToDBTask[%s]: GetClusterDependBasicInfo for cluster %s in task %s "+
			"step %s failed, %s", taskID, clusterID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// check cluster status
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	err = updateNodeToDB(ctx, dependInfo, nodes, nodeGroupIDs)
	if err != nil {
		blog.Errorf("UpdateNodesToDBTask[%s] checkNodesGroupStatus[%s] failed: %v",
			taskID, clusterID, err)
		retErr := fmt.Errorf("UpdateNodesToDBTask[%s] timeout|abnormal", clusterID)
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// sync clusterData to pass-cc
	providerutils.SyncClusterInfoToPassCC(taskID, dependInfo.Cluster)

	// sync cluster perms
	providerutils.AuthClusterResourceCreatorPerm(ctx, dependInfo.Cluster.ClusterID,
		dependInfo.Cluster.ClusterName, dependInfo.Cluster.Creator)

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("UpdateNodesToDBTask[%s] task %s %s update to storage fatal",
			taskID, taskID, stepName)
		return err
	}

	return nil
}

func updateNodeToDB(ctx context.Context, info *cloudprovider.CloudDependBasicInfo, nodes, nodeGroupIDs []string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	// get azureCloud client
	cli, err := api.NewAksServiceImplWithCommonOption(info.CmOption)
	if err != nil {
		blog.Errorf("updateNodeToDB[%s] get aks client failed: %s", taskID, err.Error())
		return fmt.Errorf("updateNodeToDB get aks client err, %s", err.Error())
	}

	nodeGroups := make([]*proto.NodeGroup, 0)
	for _, ngID := range nodeGroupIDs {
		nodeGroup, err := actions.GetNodeGroupByGroupID(cloudprovider.GetStorageModel(), ngID)
		if err != nil {
			return fmt.Errorf("updateNodeToDB GetNodeGroupByGroupID information failed, %s", err.Error())
		}
		nodeGroups = append(nodeGroups, nodeGroup)
	}

	for _, n := range nodes {
		setInfo := strings.Split(n, "-")
		if len(setInfo) != 4 {
			return fmt.Errorf("updateNodeToDB get azure scale set failed")
		}

		setName := strings.Join(setInfo[:3], "-") + "-vmss"
		node, err := cli.GetInstanceWithName(ctx, info.Cluster.ExtraInfo[common.NodeResourceGroup], setName, n)
		if err != nil {
			return fmt.Errorf("updateNodeToDB GetInstanceWithName failed, %s", err.Error())
		}
		for _, ng := range nodeGroups {
			if ng.CloudNodeGroupID == setInfo[1] {
				node.NodeGroupID = ng.NodeGroupID
				node.InstanceType = ng.LaunchTemplate.InstanceType
			}
		}

		err = cloudprovider.GetStorageModel().CreateNode(context.Background(), node)
		if err != nil {
			return fmt.Errorf("updateNodeToDB CreateNode[%s] failed, %v", node.NodeName, err)
		}

	}

	return nil
}

// RegisterAKSClusterKubeConfigTask register cluster kubeconfig
func RegisterAKSClusterKubeConfigTask(taskID string, stepName string) error {
	start := time.Now()

	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("RegisterAKSClusterKubeConfigTask[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("RegisterAKSClusterKubeConfigTask[%s] task %s run current step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// inject taskID
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)

	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]

	// handler logic
	dependInfo, err := cloudprovider.GetClusterDependBasicInfo(cloudprovider.GetBasicInfoReq{
		ClusterID: clusterID,
		CloudID:   cloudID,
	})
	if err != nil {
		blog.Errorf("RegisterAKSClusterKubeConfigTask[%s] GetClusterDependBasicInfo in task %s step %s failed, %s",
			taskID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/project information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	err = importClusterCredential(ctx, dependInfo)
	if err != nil {
		blog.Errorf("RegisterAKSClusterKubeConfigTask[%s] importClusterCredential failed: %s", taskID, err.Error())
		retErr := fmt.Errorf("importClusterCredential failed %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	blog.Infof("RegisterAKSClusterKubeConfigTask[%s] importClusterCredential success", taskID)

	// update step
	if err = state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("RegisterAKSClusterKubeConfigTask[%s:%s] update to storage fatal", taskID, stepName)
		return err
	}

	return nil
}

//func importClusterCredential(ctx context.Context, info *cloudprovider.CloudDependBasicInfo) error {
//	taskID := cloudprovider.GetTaskIDFromContext(ctx)
//
//	// get azureCloud client
//	cli, err := api.NewCTClient(info.CmOption)
//	if err != nil {
//		blog.Errorf("importClusterCredential[%s] get aks client failed: %s", taskID, err.Error())
//		return fmt.Errorf("importClusterCredential get cloud aks client err, %s", err.Error())
//	}
//
//	result, err := cli.GetKubeConfig(info.Cluster.SystemID)
//	if err != nil {
//		return fmt.Errorf("importClusterCredential[%s] GetKubeConfig failed, %v", taskID, err)
//	}
//
//	if result.ExternalKubeConfig == nil {
//		return fmt.Errorf("importClusterCredential[%s] GetKubeConfig failed, empty ExternalKubeConfig", taskID)
//	}
//
//	kubeConfig, err := types.GetKubeConfigFromYAMLBody(false, types.YamlInput{
//		FileName:    "",
//		YamlContent: result.ExternalKubeConfig.Content,
//	})
//	if err != nil {
//		return fmt.Errorf("importClusterCredential[%s] GetKubeConfigFromYAMLBody failed, %v", taskID, err)
//	}
//	err = cloudprovider.UpdateClusterCredentialByConfig(info.Cluster.ClusterID, kubeConfig)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
