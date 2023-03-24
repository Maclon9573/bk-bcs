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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/actions"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/aws/api"
	icommon "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/common"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
)

const (
	launchTemplateNameFormat = "bcs-managed-lt-%s"
	launchTemplateTagKey     = "bcs-managed-template"
	launchTemplateTagValue   = "do-not-modify-or-delete"
	defaultStorageDeviceName = "/dev/xvda"
)

// CreateCloudNodeGroupTask create cloud node group task
func CreateCloudNodeGroupTask(taskID string, stepName string) error {
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
	cloudID := step.Params["CloudID"]
	nodeGroupID := step.Params["NodeGroupID"]
	group, err := cloudprovider.GetStorageModel().GetNodeGroup(context.Background(), nodeGroupID)
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: get nodegroup for %s failed", taskID, nodeGroupID)
		retErr := fmt.Errorf("get nodegroup information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// get cloud and cluster info
	cloud, cluster, err := actions.GetCloudAndCluster(cloudprovider.GetStorageModel(), cloudID, group.ClusterID)
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: get cloud/cluster for nodegroup %s in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/cluster information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// get dependency resource for cloudprovider operation
	cmOption, err := cloudprovider.GetCredential(&cloudprovider.CredentialData{
		Cloud:     cloud,
		AccountID: cluster.CloudAccountID,
	})
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: get credential for nodegroup %s in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud credential err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	cmOption.Region = group.Region

	// create node group
	eksCli, err := api.NewEksClient(cmOption)
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: get eks client for nodegroup[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud eks client err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return err
	}

	// set default value for nodegroup
	if group.AutoScaling != nil && group.AutoScaling.VpcID == "" {
		group.AutoScaling.VpcID = cluster.VpcID
	}
	ec2Cli, err := api.NewEC2Client(cmOption)
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: get ec2 client for nodegroup[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud eks client err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return err
	}
	ng, err := eksCli.CreateNodegroup(generateCreateNodegroupInput(group, cluster, ec2Cli))
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: call CreateClusterNodePool[%s] api in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("call CreateClusterNodePool[%s] api err, %s", nodeGroupID, err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	blog.Infof("CreateCloudNodeGroupTask[%s]: call CreateClusterNodePool successful", taskID)
	group.CloudNodeGroupID = *ng.NodegroupName

	// update nodegorup cloudNodeGroupID
	err = updateNodeGroupCloudNodeGroupID(nodeGroupID, group)
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: updateNodeGroupCloudNodeGroupID[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("call CreateCloudNodeGroupTask updateNodeGroupCloudNodeGroupID[%s] api err, %s", nodeGroupID,
			err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	blog.Infof("CreateCloudNodeGroupTask[%s]: call CreateNodegroup updateNodeGroupCloudNodeGroupID successful",
		taskID)

	// update response information to task common params
	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}

	state.Task.CommonParams["CloudNodeGroupID"] = *ng.NodegroupName
	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}
	return nil
}

func generateCreateNodegroupInput(group *proto.NodeGroup, cluster *proto.Cluster, cli *api.EC2Client) *api.CreateNodegroupInput {
	if group.AutoScaling == nil || group.LaunchTemplate == nil {
		return nil
	}
	nodeGroup := api.CreateNodegroupInput{
		ClusterName:   &cluster.SystemID,
		NodegroupName: &group.NodeGroupID,
		AmiType:       aws.String(group.NodeOS),
		ScalingConfig: &api.NodegroupScalingConfig{
			DesiredSize: aws.Int64(0),
			MaxSize:     aws.Int64(int64(group.AutoScaling.MaxSize)),
			MinSize:     aws.Int64(int64(group.AutoScaling.MinSize)),
		},
		Tags:   aws.StringMap(group.Tags),
		Labels: aws.StringMap(group.Labels),
	}
	nodeGroup.CapacityType = &group.LaunchTemplate.InstanceChargeType
	if nodeGroup.CapacityType != aws.String(eks.CapacityTypesOnDemand) &&
		nodeGroup.CapacityType != aws.String(eks.CapacityTypesSpot) {
		nodeGroup.CapacityType = aws.String(eks.CapacityTypesOnDemand)
	}

	lt, err := createLaunchTemplate(group, cluster.SystemID, cli)
	if err != nil {
		blog.Errorf("create launch template failed, %v", err)
		return nil
	}

	if group.NodeTemplate != nil {
		nodeGroup.Taints = api.MapToTaints(group.NodeTemplate.Taints)
	}
	nodeGroup.LaunchTemplate, err = createNewLaunchTemplateVersion(*lt.LaunchTemplateId, group, cli)
	if err != nil {
		blog.Errorf("createNewLaunchTemplateVersion failed, %v", err)
		return nil
	}
	nodeGroup.NodeRole = &group.NodeRole
	return &nodeGroup
}

func createLaunchTemplate(group *proto.NodeGroup, clusterName string, cli *api.EC2Client) (*api.LaunchTemplate, error) {
	// Create first version of the launch template as default version. It will not be used for any node group.
	var imageID *string
	if group.NodeOS != "" {
		imageID = aws.String(group.NodeOS)
	}
	launchTemplateCreateInput := &api.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(fmt.Sprintf(launchTemplateNameFormat, clusterName)),
		LaunchTemplateData: &api.RequestLaunchTemplateData{
			ImageId:          imageID,
			InstanceType:     aws.String(group.LaunchTemplate.InstanceType),
			KeyName:          aws.String(group.LaunchTemplate.SshKey),
			UserData:         aws.String(group.LaunchTemplate.UserData),
			SecurityGroupIds: aws.StringSlice(group.LaunchTemplate.SecurityGroupIDs),
		},
		TagSpecifications: []*api.TagSpecification{
			{
				ResourceType: aws.String(api.ResourceTypeLaunchTemplate),
				Tags: []*api.Tag{
					{
						Key:   aws.String(launchTemplateTagKey),
						Value: aws.String(launchTemplateTagValue),
					},
				},
			},
		},
	}

	output, err := cli.CreateLaunchTemplate(launchTemplateCreateInput)
	if err != nil {
		return nil, err
	}

	return &api.LaunchTemplate{
		LaunchTemplateName:  output.LaunchTemplateName,
		LaunchTemplateId:    output.LaunchTemplateId,
		LatestVersionNumber: output.LatestVersionNumber,
	}, nil
}

func createNewLaunchTemplateVersion(ltID string, group *proto.NodeGroup, cli *api.EC2Client) (*api.LaunchTemplateSpecification, error) {
	ltData, err := buildLaunchTemplateData(group, cli)
	if err != nil {
		return nil, err
	}

	launchTemplateVersionInput := &ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateData: ltData,
		LaunchTemplateId:   aws.String(ltID),
	}

	output, err := cli.CreateLaunchTemplateVersion(launchTemplateVersionInput)
	if err != nil {
		return nil, err
	}
	version := strconv.Itoa(int(*output.VersionNumber))

	return &api.LaunchTemplateSpecification{
		Id:      output.LaunchTemplateName,
		Name:    output.LaunchTemplateId,
		Version: aws.String(version),
	}, nil
}

func buildLaunchTemplateData(group *proto.NodeGroup, cli *api.EC2Client) (*ec2.RequestLaunchTemplateData, error) {
	deviceName := aws.String(defaultStorageDeviceName)
	if group.NodeOS != "" {
		if rootDeviceName, err := getImageRootDeviceName([]*string{&group.NodeOS}, cli); err != nil {
			return nil, err
		} else if rootDeviceName != nil {
			deviceName = rootDeviceName
		}
	}

	userdata := group.NodeTemplate.UserScript
	if userdata != "" {
		userdata = base64.StdEncoding.EncodeToString([]byte(userdata))
	}

	volumeSize, err := strconv.Atoi(group.LaunchTemplate.SystemDisk.DiskSize)
	if err != nil {
		return nil, err
	}

	launchTemplateData := &ec2.RequestLaunchTemplateData{
		//KeyName:  group.Ec2SshKey,
		UserData: aws.String(userdata),
		BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
			{
				DeviceName: deviceName,
				Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize:          aws.Int64(int64(volumeSize)),
					DeleteOnTermination: aws.Bool(true),
					VolumeType:          aws.String(group.LaunchTemplate.SystemDisk.DiskType),
				},
			},
		},
		TagSpecifications: api.CreateTagSpecs(aws.StringMap(group.Tags)),
	}

	if len(group.LaunchTemplate.DataDisks) != 0 {
		for k, v := range group.LaunchTemplate.DataDisks {
			if k >= len(api.DeviceName) {
				return nil, fmt.Errorf("data disks counts can't larger than %d", len(api.DeviceName))
			}
			size, err := strconv.Atoi(v.DiskSize)
			if err != nil {
				return nil, err
			}
			launchTemplateData.BlockDeviceMappings = append(launchTemplateData.BlockDeviceMappings,
				&ec2.LaunchTemplateBlockDeviceMappingRequest{
					DeviceName: aws.String(api.DeviceName[k]),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						VolumeSize:          aws.Int64(int64(size)),
						DeleteOnTermination: aws.Bool(true),
						VolumeType:          aws.String(v.DiskType),
					},
				})
		}
	}
	launchTemplateData.InstanceType = aws.String(group.LaunchTemplate.InstanceType)

	return launchTemplateData, nil
}

func getImageRootDeviceName(imageID []*string, cli *api.EC2Client) (*string, error) {
	describeOutput, err := cli.DescribeImages(&ec2.DescribeImagesInput{ImageIds: imageID})
	if err != nil {
		return nil, err
	}
	return describeOutput.RootDeviceName, nil
}

// CheckCloudNodeGroupStatusTask check cloud node group status task
func CheckCloudNodeGroupStatusTask(taskID string, stepName string) error {
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
	nodeGroupID := step.Params["NodeGroupID"]
	cloudID := step.Params["CloudID"]

	group, err := cloudprovider.GetStorageModel().GetNodeGroup(context.Background(), nodeGroupID)
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get nodegroup for %s failed", taskID, nodeGroupID)
		retErr := fmt.Errorf("get nodegroup information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	cloud, cluster, err := actions.GetCloudAndCluster(cloudprovider.GetStorageModel(), cloudID, group.ClusterID)
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get cloud/cluster for nodegroup %s in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud/cluster information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// get dependency resource for cloudprovider operation
	cmOption, err := cloudprovider.GetCredential(&cloudprovider.CredentialData{
		Cloud:     cloud,
		AccountID: cluster.CloudAccountID,
	})
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get credential for nodegroup %s in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud credential err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	cmOption.Region = group.Region

	// get eks client
	eksCli, err := api.NewEksClient(cmOption)
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get eks client for nodegroup[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud eks client err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	asCli, err := api.NewAutoScalingClient(cmOption)
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get as client for nodegroup[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud as client err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	ec2Cli, err := api.NewEC2Client(cmOption)
	if err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s]: get ec2 client for nodegroup[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("get cloud as client err, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// wait node group state to normal
	ctx, cancel := context.WithTimeout(context.TODO(), 20*time.Minute)
	defer cancel()
	var asgName, ltName, ltVersion *string
	cloudNodeGroup := &eks.Nodegroup{}
	err = cloudprovider.LoopDoFunc(ctx, func() error {

		ng, err := eksCli.DescribeNodegroup(&group.CloudNodeGroupID, &cluster.SystemID)
		if err != nil {
			blog.Errorf("taskID[%s] DescribeClusterNodePoolDetail[%s/%s] failed: %v", taskID, cluster.SystemID,
				group.CloudNodeGroupID, err)
			return nil
		}
		if ng == nil {
			return nil
		}
		cloudNodeGroup = ng
		if ng.Resources != nil && ng.Resources.AutoScalingGroups != nil {
			asgName = ng.Resources.AutoScalingGroups[0].Name
		}
		if ng.LaunchTemplate != nil {
			ltName = ng.LaunchTemplate.Name
			ltVersion = ng.LaunchTemplate.Version
		}
		switch {
		case *ng.Status == api.NodeGroupStatusCreating:
			blog.Infof("taskID[%s] DescribeNodegroup[%s] still creating, status[%s]",
				taskID, group.CloudNodeGroupID, *ng.Status)
			return nil
		case *ng.Status == api.NodeGroupStatusActive:
			return cloudprovider.EndLoop
		default:
			return nil
		}
	}, cloudprovider.LoopInterval(5*time.Second))
	if err != nil {
		blog.Errorf("taskID[%s] DescribeNodegroup failed: %v", taskID, err)
		return err
	}

	// get asg info
	asgInfo, err := asCli.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{asgName}})
	if err != nil {
		blog.Errorf("taskID[%s] DescribeAutoScalingGroups[%s] failed: %v", taskID, *asgName, err)
		return err
	}

	// get launchTemplateVersion
	ltvInfo, err := ec2Cli.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateName: ltName, Versions: []*string{ltVersion}})
	if err != nil {
		blog.Errorf("taskID[%s] DescribeLaunchTemplateVersions[%s] failed: %v", taskID, *ltName, err)
		return err
	}

	err = cloudprovider.GetStorageModel().UpdateNodeGroup(context.Background(), generateNodeGroupFromAsgAndLtv(group,
		cloudNodeGroup, asgInfo[0], ltvInfo[0]))
	if err != nil {
		blog.Errorf("CreateCloudNodeGroupTask[%s]: updateNodeGroupCloudArgsID[%s] in task %s step %s failed, %s",
			taskID, nodeGroupID, taskID, stepName, err.Error())
		retErr := fmt.Errorf("call CreateCloudNodeGroupTask updateNodeGroupCloudArgsID[%s] api err, %s", nodeGroupID,
			err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}

	// update response information to task common params
	if state.Task.CommonParams == nil {
		state.Task.CommonParams = make(map[string]string)
	}

	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("CheckCloudNodeGroupStatusTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}
	return nil
}

func generateNodeGroupFromAsgAndLtv(group *proto.NodeGroup, cloudNodeGroup *eks.Nodegroup, asg *autoscaling.Group,
	ltv *ec2.LaunchTemplateVersion) *proto.NodeGroup {
	group = generateNodeGroupFromAsg(group, cloudNodeGroup, asg)
	return generateNodeGroupFromLtv(group, cloudNodeGroup, ltv)
}

func generateNodeGroupFromAsg(group *proto.NodeGroup, cloudNodeGroup *eks.Nodegroup,
	asg *autoscaling.Group) *proto.NodeGroup {
	if asg.AutoScalingGroupName != nil {
		group.AutoScaling.AutoScalingName = *asg.AutoScalingGroupName
		group.AutoScaling.AutoScalingID = *asg.AutoScalingGroupName
	}
	if asg.MaxSize != nil {
		group.AutoScaling.MinSize = uint32(*asg.MaxSize)
	}
	if asg.MinSize != nil {
		group.AutoScaling.MinSize = uint32(*asg.MinSize)
	}
	if asg.DesiredCapacity != nil {
		group.AutoScaling.DesiredSize = uint32(*asg.DesiredCapacity)
	}
	if asg.DefaultCooldown != nil {
		group.AutoScaling.DefaultCooldown = uint32(*asg.DefaultCooldown)
	}
	if asg.VPCZoneIdentifier != nil {
		subnetIDs := make([]string, 0)
		subnetIDs = strings.Split(*asg.VPCZoneIdentifier, ",")
		group.AutoScaling.SubnetIDs = subnetIDs
	}

	return group
}

func generateNodeGroupFromLtv(group *proto.NodeGroup, cloudNodeGroup *eks.Nodegroup,
	ltv *ec2.LaunchTemplateVersion) *proto.NodeGroup {
	if ltv.LaunchTemplateId != nil {
		group.LaunchTemplate.LaunchConfigurationID = *ltv.LaunchTemplateId
	}
	if ltv.LaunchTemplateName != nil {
		group.LaunchTemplate.LaunchConfigureName = *ltv.LaunchTemplateName
	}
	if ltv.LaunchTemplateData != nil {
		group.LaunchTemplate.InstanceType = *ltv.LaunchTemplateData.InstanceType
		if ltv.LaunchTemplateData.SecurityGroupIds != nil {
			group.LaunchTemplate.SecurityGroupIDs = make([]string, 0)
			for _, v := range ltv.LaunchTemplateData.SecurityGroupIds {
				group.LaunchTemplate.SecurityGroupIDs = append(group.LaunchTemplate.SecurityGroupIDs, *v)
			}
		}
		group.LaunchTemplate.ImageInfo = &proto.ImageInfo{ImageID: *ltv.LaunchTemplateData.ImageId}
		group.LaunchTemplate.UserData = *ltv.LaunchTemplateData.UserData
		if ltv.LaunchTemplateData.Monitoring != nil {
			group.LaunchTemplate.IsMonitorService = *ltv.LaunchTemplateData.Monitoring.Enabled
		}
	}

	return group
}

// UpdateCreateNodeGroupDBInfoTask update create node group db info task
func UpdateCreateNodeGroupDBInfoTask(taskID string, stepName string) error {
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
	nodeGroupID := step.Params["NodeGroupID"]

	np, err := cloudprovider.GetStorageModel().GetNodeGroup(context.Background(), nodeGroupID)
	if err != nil {
		blog.Errorf("UpdateCreateNodeGroupDBInfoTask[%s]: get cluster for %s failed", taskID, nodeGroupID)
		retErr := fmt.Errorf("get nodegroup information failed, %s", err.Error())
		_ = state.UpdateStepFailure(start, stepName, retErr)
		return retErr
	}
	np.Status = icommon.StatusRunning

	err = cloudprovider.GetStorageModel().UpdateNodeGroup(context.Background(), np)
	if err != nil {
		blog.Errorf("UpdateCreateNodeGroupDBInfoTask[%s]: update nodegroup status for %s failed", taskID, np.Status)
	}

	// update step
	if err := state.UpdateStepSucc(start, stepName); err != nil {
		blog.Errorf("UpdateCreateNodeGroupDBInfoTask[%s] task %s %s update to storage fatal", taskID, taskID, stepName)
		return err
	}

	return nil
}
