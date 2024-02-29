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
	"strings"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/odm/drivers"
	"github.com/Tencent/bk-bcs/bcs-common/pkg/odm/operator"
	"github.com/avast/retry-go"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/common"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/remote/loop"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/store/options"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/utils"
)

// DeployBCSComponents deploy BCS components
func DeployBCSComponents(taskID string, stepName string) error {
	start := time.Now()

	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("DeployBCSComponents[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("DeployBCSComponents[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	originClusterID := step.Params[cloudprovider.OriginClusterIDKey.String()]

	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	// import cluster instances
	err = deployBCSComponents(ctx, originClusterID, clusterID)
	if err != nil {
		blog.Errorf("DeployBCSComponents[%s] failed: %v", taskID, err)
		retErr := fmt.Errorf("deployBCSComponents failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}

	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("DeployBCSComponents[%s]  %s update to storage fatal", taskID, stepName)
		return err
	}
	return nil
}

func deployBCSComponents(ctx context.Context, oriClusterID, clusterID string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	clientSet, err := generateClientSet(oriClusterID)
	if err != nil {
		blog.Errorf("deployBCSComponents[%s] create clientset failed, %v", taskID, err)
		return err
	}

	deployList, err := clientSet.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	agentDeploy, watchDeploy := v1.Deployment{}, v1.Deployment{}
	for _, deploy := range deployList.Items {
		if strings.Contains(deploy.Name, "bcs-kube-agent") {
			agentDeploy = deploy
			continue
		}
		if strings.Contains(deploy.Name, "bcs-k8s-watch") {
			watchDeploy = deploy
			continue
		}
	}

	err = deployK8SWatch(ctx, clientSet, watchDeploy, clusterID)
	if err != nil {
		return err
	}

	err = deployKubeAgent(ctx, clientSet, agentDeploy, clusterID)
	if err != nil {
		return err
	}

	return nil
}

func deployK8SWatch(ctx context.Context, client *kubernetes.Clientset, deploy v1.Deployment, clusterID string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	if deploy.Name == "" {
		blog.Infof("deployK8SWatch[%s] skip for cluster[%s], not installed before",
			taskID, clusterID)
		return nil
	}

	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "bcs-k8s-watch" {
			envs := []corev1.EnvVar{
				{
					Name:  "clusterId",
					Value: clusterID,
				},
			}
			for _, v := range deploy.Spec.Template.Spec.Containers[i].Env {
				if v.Name != "clusterId" {
					envs = append(envs, v)
				}
			}
			deploy.Spec.Template.Spec.Containers[i].Env = envs
		}
	}

	_, err := client.AppsV1().Deployments(deploy.Namespace).Update(ctx, &deploy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// wait deployment state to normal
	timeCtx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	err = loop.LoopDoFunc(timeCtx, func() error {
		dep, getErr := client.AppsV1().Deployments(deploy.Namespace).Get(timeCtx, deploy.Name, metav1.GetOptions{})
		if getErr != nil {
			blog.Errorf("deployK8SWatch[%s] for cluster[%s] failed, %v, retrying...",
				taskID, clusterID, getErr)
			return nil
		}

		if len(dep.Status.Conditions) != 0 && dep.Status.Conditions[0].Type == v1.DeploymentAvailable &&
			dep.Status.Conditions[0].Status == corev1.ConditionTrue {
			blog.Infof("deployK8SWatch[%s] for cluster[%s] success", taskID, clusterID)
			return loop.EndLoop
		}

		return nil
	}, loop.LoopInterval(3*time.Second))
	if err != nil {
		blog.Errorf("deployK8SWatch[%s] for cluster[%s] failed, %v",
			taskID, clusterID, err)
		return err
	}

	return nil
}

func deployKubeAgent(ctx context.Context, client *kubernetes.Clientset, deploy v1.Deployment, clusterID string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)
	if deploy.Name == "" {
		blog.Errorf("deployKubeAgent[%s] for cluster[%s] failed, can not get kube agent",
			taskID, clusterID)
		return fmt.Errorf("deployKubeAgent[%s] for cluster[%s] failed, can not get kube agent",
			taskID, clusterID)
	}

	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "bcs-kube-agent" {
			args := []string{fmt.Sprintf("--cluster-id=%s", clusterID)}
			for _, v := range deploy.Spec.Template.Spec.Containers[i].Args {
				if !strings.Contains(v, "--cluster-id") {
					args = append(args, v)
				}
			}
			deploy.Spec.Template.Spec.Containers[i].Args = args
		}
	}
	_, err := client.AppsV1().Deployments(deploy.Namespace).Update(ctx, &deploy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	timeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err = loop.LoopDoFunc(timeCtx, func() error {
		_, found, getErr := cloudprovider.GetStorageModel().GetClusterCredential(timeCtx, clusterID)
		if getErr != nil {
			blog.Errorf("deployKubeAgent[%s] get clusterCredential[%s] failed, %v, retrying...",
				taskID, clusterID, getErr)
			return nil
		}
		if !found {
			blog.Errorf("deployKubeAgent[%s] get clusterCredential[%s] not found, retrying...", taskID, clusterID)
			return nil
		}

		blog.Infof("deployKubeAgent[%s] for cluster[%s] success", taskID, clusterID)

		return loop.EndLoop
	}, loop.LoopInterval(3*time.Second))
	if err != nil {
		blog.Errorf("deployKubeAgent[%s] for cluster[%s] failed, %v", taskID, clusterID, err)
		return err
	}

	return nil
}

// CheckClusterStatus check cluster status
func CheckClusterStatus(taskID string, stepName string) error {
	start := time.Now()

	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("DeployBCSComponents[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("DeployBCSComponents[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	originClusterID := step.Params[cloudprovider.OriginClusterIDKey.String()]
	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	// import cluster instances
	err = checkClusterStatus(ctx, clusterID, originClusterID)
	if err != nil {
		blog.Errorf("DeployBCSComponents[%s] failed: %v", taskID, err)
		retErr := fmt.Errorf("deployBCSComponents failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("DeployBCSComponents[%s]  %s update to storage fatal", taskID, stepName)
		return err
	}
	return nil
}

func checkClusterStatus(ctx context.Context, clusterID, originClusterID string) error {
	taskID := cloudprovider.GetTaskIDFromContext(ctx)

	clientSet, err := generateClientSet(clusterID)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] create clientset failed, %v", taskID, err)
		return err
	}

	err = retry.Do(func() error {
		_, errList := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if errList != nil {
			blog.Errorf("checkClusterStatus[%s] failed: %v", taskID, errList)
			return errList
		}

		blog.Infof("checkClusterStatus[%s] cluster[%s] successful", taskID, clusterID)

		return nil
	}, retry.Attempts(3))

	// update origin cluster status
	oriCluster, err := cloudprovider.GetStorageModel().GetCluster(ctx, originClusterID)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] GetCluster[%s] failed, %v", taskID, originClusterID, err)
		return err
	}
	oriCluster.Status = common.StatusDeleted
	err = cloudprovider.GetStorageModel().UpdateCluster(ctx, oriCluster)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] UpdateCluster[%s] status failed, %v", taskID, originClusterID, err)
		return err
	}

	// update new cluster status
	cluster, err := cloudprovider.GetStorageModel().GetCluster(ctx, clusterID)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] GetCluster[%s] failed, %v", taskID, clusterID, err)
		return err
	}
	cluster.Status = common.StatusRunning
	err = cloudprovider.GetStorageModel().UpdateCluster(ctx, cluster)
	if err != nil {
		blog.Errorf("checkClusterStatus[%s] UpdateCluster[%s] status failed, %v", taskID, clusterID, err)
		return err
	}

	return nil
}

// SyncClusterNodes sync cluster nodes
func SyncClusterNodes(taskID string, stepName string) error {
	start := time.Now()

	// get task and task current step
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	// previous step successful when retry task
	if step == nil {
		blog.Infof("SyncClusterNodes[%s]: current step[%s] successful and skip", taskID, stepName)
		return nil
	}
	blog.Infof("SyncClusterNodes[%s]: task %s run step %s, system: %s, old state: %s, params %v",
		taskID, taskID, stepName, step.System, step.Status, step.Params)

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	originClusterID := step.Params[cloudprovider.OriginClusterIDKey.String()]

	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)
	// import cluster instances
	err = syncClusterNodes(ctx, originClusterID, clusterID)
	if err != nil {
		blog.Errorf("SyncClusterNodes[%s] syncClusterNodes failed: %v", taskID, err)
		retErr := fmt.Errorf("SyncClusterNodes failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("SyncClusterNodes[%s]  %s update to storage fatal", taskID, stepName)
		return err
	}
	return nil
}

func syncClusterNodes(ctx context.Context, originClusterID, clusterID string) error {
	err := deleteOriNodes(ctx, originClusterID)
	if err != nil {
		return err
	}

	clientSet, err := generateClientSet(clusterID)
	if err != nil {
		return err
	}
	nodes, err := clientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		blog.Errorf("syncClusterNodes[%s] listNode failed, %v", clusterID, err)
		return err
	}

	for _, n := range nodes.Items {
		ipv4, ipv6 := utils.GetNodeIPAddress(&n)
		node := &proto.Node{
			InnerIP: utils.SliceToString(ipv4),
			Status: func(n corev1.Node) string {
				if utils.CheckNodeIfReady(&n) {
					return common.StatusRunning
				}
				return common.StatusNodeUnknown
			}(n),
			ClusterID: clusterID,
			Region:    "default",
			NodeName:  n.Name,
			InnerIPv6: utils.SliceToString(ipv6),
		}
		err = cloudprovider.GetStorageModel().CreateNode(ctx, node)
		if err != nil {
			if errors.Is(err, drivers.ErrTableRecordDuplicateKey) {
				continue
			}
			blog.Errorf("syncClusterNodes[%s] createNode[%s] failed, %v", clusterID, n.Name, err)
			return err
		}
	}

	return nil
}

func deleteOriNodes(ctx context.Context, originClusterID string) error {
	condM := make(operator.M)
	condM["clusterid"] = originClusterID
	cond := &operator.Condition{
		Op:    operator.Eq,
		Value: condM,
	}
	oriNodes, err := cloudprovider.GetStorageModel().ListNode(ctx, cond, &options.ListOption{})
	if err != nil {
		blog.Errorf("deleteOriNodes[%s] ListNode failed, %v", originClusterID, err)
		return err
	}
	for _, n := range oriNodes {
		err = cloudprovider.GetStorageModel().DeleteNodeByIP(ctx, n.GetInnerIP())
		if err != nil {
			blog.Errorf("deleteOriNodes[%s] delete node[%s] failed, %v", originClusterID, n.GetInnerIP(), err)
			return err
		}
	}

	return nil
}
