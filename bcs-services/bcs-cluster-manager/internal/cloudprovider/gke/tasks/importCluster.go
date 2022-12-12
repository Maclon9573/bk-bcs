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

package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	gkeapi "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/gke/api"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// ImportClusterNodesTask call gkeInterface or kubeConfig import cluster nodes
func ImportClusterNodesTask(taskID string, stepName string) error {
	start := time.Now()
	// get task information and validate
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	if step == nil {
		return nil
	}

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]

	basicInfo, err := cloudprovider.GetClusterDependBasicInfo(clusterID, cloudID, "")
	if err != nil {
		blog.Errorf("ImportClusterNodesTask[%s]: getClusterDependBasicInfo failed: %v", taskID, err)
		retErr := fmt.Errorf("getClusterDependBasicInfo failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// import cluster instances
	err = importClusterInstances(basicInfo)
	if err != nil {
		blog.Errorf("ImportClusterNodesTask[%s]: importClusterInstances failed: %v", taskID, err)
		retErr := fmt.Errorf("importClusterInstances failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update cluster masterNodes info
	cloudprovider.GetStorageModel().UpdateCluster(context.Background(), basicInfo.Cluster)

	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("ImportClusterNodesTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}
	return nil
}

// RegisterClusterKubeConfigTask register cluster kubeConfig connection
func RegisterClusterKubeConfigTask(taskID string, stepName string) error {
	start := time.Now()
	// get task information and validate
	state, step, err := cloudprovider.GetTaskStateAndCurrentStep(taskID, stepName)
	if err != nil {
		return err
	}
	if step == nil {
		return nil
	}

	// step login started here
	clusterID := step.Params[cloudprovider.ClusterIDKey.String()]
	cloudID := step.Params[cloudprovider.CloudIDKey.String()]

	basicInfo, err := cloudprovider.GetClusterDependBasicInfo(clusterID, cloudID, "")
	if err != nil {
		blog.Errorf("RegisterClusterKubeConfigTask[%s]: getClusterDependBasicInfo failed: %v", taskID, err)
		retErr := fmt.Errorf("getClusterDependBasicInfo failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	ctx := cloudprovider.WithTaskIDForContext(context.Background(), taskID)

	err = importClusterCredential(ctx, basicInfo)
	if err != nil {
		blog.Errorf("RegisterClusterKubeConfigTask[%s]: importClusterCredential failed: %v", taskID, err)
		retErr := fmt.Errorf("importClusterCredential failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("RegisterClusterKubeConfigTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}
	return nil
}

func importClusterCredential(ctx context.Context, data *cloudprovider.CloudDependBasicInfo) error {
	saSecret := data.Cloud.CloudCredential.ServiceAccountSecret
	gkeProjectID := data.Cloud.CloudCredential.GkeProjectID
	client, err := gkeapi.GetContainerServiceClient(ctx, saSecret)
	if err != nil {
		return err
	}

	// Get the kube cluster in given project.
	parent := "projects/" + gkeProjectID + "/locations/" + data.Cluster.Region + "/clusters/" + data.Cluster.ClusterName
	gkeCluster, err := client.Projects.Locations.Clusters.Get(parent).Do()
	if err != nil {
		return fmt.Errorf("clusters list project=%s: %w", gkeProjectID, err)
	}
	name := fmt.Sprintf("%s_%s_%s", gkeProjectID, gkeCluster.Location, gkeCluster.Name)
	cert, err := base64.StdEncoding.DecodeString(gkeCluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return fmt.Errorf("invalid certificate cluster=%s: %w", name, err)
	}

	restConfig := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			CAData: cert,
		},
		Host: "https://" + gkeCluster.Endpoint,
		AuthProvider: &api.AuthProviderConfig{
			Name: gkeapi.GoogleAuthPlugin,
			Config: map[string]string{
				"scopes":      "https://www.googleapis.com/auth/cloud-platform",
				"credentials": saSecret,
			},
		},
	}

	var saToken string
	saToken, err = GenerateSAToken(restConfig)
	if err != nil {
		return fmt.Errorf("failed to generate k8s serviceaccount token project=%s cluster=%s: %w",
			gkeProjectID, data.Cluster.ClusterName, err)
	}
	typesConfig := &types.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []types.NamedCluster{
			{
				Name: name,
				Cluster: types.ClusterInfo{
					Server:                   "https://" + gkeCluster.Endpoint,
					CertificateAuthorityData: cert,
				},
			},
		},
		AuthInfos: []types.NamedAuthInfo{
			{
				Name: name,
				AuthInfo: types.AuthInfo{
					Token: saToken,
				},
			},
		},
		Contexts: []types.NamedContext{
			{
				Name: name,
				Context: types.Context{
					Cluster:  name,
					AuthInfo: name,
				},
			},
		},
		CurrentContext: name,
	}

	configByte, err := json.Marshal(typesConfig)
	if err != nil {
		return fmt.Errorf("failed to marsh kubeconfig, %v", err)
	}
	data.Cluster.KubeConfig = base64.StdEncoding.EncodeToString(configByte)
	cloudprovider.UpdateCluster(data.Cluster)

	err = cloudprovider.UpdateClusterCredentialByConfig(data.Cluster.ClusterID, typesConfig)
	if err != nil {
		return err
	}

	return nil
}

func importClusterInstances(data *cloudprovider.CloudDependBasicInfo) error {
	kubeConfigByte, err := base64.StdEncoding.DecodeString(data.Cluster.KubeConfig)
	if err != nil {
		return fmt.Errorf("decode kube config failed: %v", err)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigByte)
	if err != nil {
		return fmt.Errorf("build rest config failed: %v", err)
	}

	config.Burst = 200
	config.QPS = 100
	kubeCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("build kube client failed: %s", err)
	}

	nodes, err := kubeCli.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list nodes failed, %s", err.Error())
	}

	err = cloudprovider.ImportClusterNodesToCM(context.Background(), nodes.Items, data.Cluster.ClusterID)
	if err != nil {
		return err
	}

	return nil
}
