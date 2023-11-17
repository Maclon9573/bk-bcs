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

package aws

import (
	"fmt"
	"sync"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/aws/api"
)

const (
	limit = 100
)

var nodeMgr sync.Once

func init() {
	nodeMgr.Do(func() {
		// init Node
		cloudprovider.InitNodeManager("aws", &NodeManager{})
	})
}

// NodeManager CVM relative API management
type NodeManager struct {
}

// GetNodeByIP get specified Node by innerIP address
func (nm *NodeManager) GetNodeByIP(ip string, opt *cloudprovider.GetNodeOption) (*proto.Node, error) {
	return nil, nil
}

// ListNodesByIP list node by IP set
func (nm *NodeManager) ListNodesByIP(ips []string, opt *cloudprovider.ListNodesOption) ([]*proto.Node, error) {
	return nil, nil
}

// GetCVMImageIDByImageName get imageID by imageName
func (nm *NodeManager) GetCVMImageIDByImageName(imageName string, opt *cloudprovider.CommonOption) (string, error) {
	return "", nil
}

// GetCloudRegions get cloud regions
func (nm *NodeManager) GetCloudRegions(opt *cloudprovider.CommonOption) ([]*proto.RegionInfo, error) {
	client, err := api.GetEc2Client(opt)
	if err != nil {
		blog.Errorf("create ec2 client when GetRegionsInfo failed: %v", err)
		return nil, err
	}

	input := &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
	}
	output, err := client.DescribeRegions(input)
	if err != nil {
		blog.Errorf("ec2 client DescribeRegions failed: %v", err)
		return nil, err
	}

	regions := make([]*proto.RegionInfo, 0)
	for _, v := range output.Regions {
		regions = append(regions, &proto.RegionInfo{
			Region:      *v.Endpoint,
			RegionName:  *v.RegionName,
			RegionState: *v.OptInStatus,
		})
	}

	return regions, nil
}

// GetZoneList get zoneList by region
func (nm *NodeManager) GetZoneList(opt *cloudprovider.CommonOption) ([]*proto.ZoneInfo, error) {
	client, err := api.NewEC2Client(opt)
	if err != nil {
		return nil, fmt.Errorf("create ec2 client failed, err %s", err.Error())
	}
	zones, err := client.DescribeAvailabilityZones(
		&ec2.DescribeAvailabilityZonesInput{AllAvailabilityZones: aws.Bool(true)})
	if err != nil {
		return nil, fmt.Errorf("list regions failed, err %s", err.Error())
	}
	var zonesInfo []*proto.ZoneInfo
	for _, z := range zones {
		zonesInfo = append(zonesInfo, &proto.ZoneInfo{
			ZoneID:    *z.ZoneId,
			Zone:      *z.ZoneId,
			ZoneName:  *z.ZoneName,
			ZoneState: *z.State,
		})
	}

	return zonesInfo, nil
}

// ListNodeInstanceType list node type by zone and node family
func (nm *NodeManager) ListNodeInstanceType(info cloudprovider.InstanceInfo, opt *cloudprovider.CommonOption) (
	[]*proto.InstanceType, error) {
	return nil, cloudprovider.ErrCloudNotImplemented
}

// GetExternalNodeByIP get specified Node by innerIP address
func (nm *NodeManager) GetExternalNodeByIP(ip string, opt *cloudprovider.GetNodeOption) (*proto.Node, error) {
	return nil, cloudprovider.ErrCloudNotImplemented
}

// ListExternalNodesByIP list node by IP set
func (nm *NodeManager) ListExternalNodesByIP(ips []string, opt *cloudprovider.ListNodesOption) ([]*proto.Node, error) {
	return nil, cloudprovider.ErrCloudNotImplemented
}

// ListOsImage get osimage list
func (nm *NodeManager) ListOsImage(provider string, opt *cloudprovider.CommonOption) ([]*proto.OsImage, error) {
	return nil, nil
}

// ListKeyPairs keyPairs list
func (nm *NodeManager) ListKeyPairs(opt *cloudprovider.CommonOption) ([]*proto.KeyPair, error) {
	return nil, cloudprovider.ErrCloudNotImplemented
}
