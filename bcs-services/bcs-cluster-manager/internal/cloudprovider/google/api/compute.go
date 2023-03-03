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

package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"

	"golang.org/x/oauth2"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

const (
	locationTypeZones   = "zones"
	locationTypeRegions = "regions"
)

// ComputeServiceClient compute service client
type ComputeServiceClient struct {
	gkeProjectID         string
	location             string
	computeServiceClient *compute.Service
}

// NewComputeServiceClient create a client for google compute service
func NewComputeServiceClient(opt *cloudprovider.CommonOption) (*ComputeServiceClient, error) {
	if opt == nil || opt.Account == nil {
		return nil, cloudprovider.ErrCloudCredentialLost
	}
	if len(opt.Account.ServiceAccountSecret) == 0 || opt.Account.GkeProjectID == "" {
		return nil, cloudprovider.ErrCloudCredentialLost
	}
	computeServiceClient, err := getComputeServiceClient(context.Background(), opt.Account.ServiceAccountSecret)
	if err != nil {
		return nil, cloudprovider.ErrCloudInitFailed
	}
	return &ComputeServiceClient{
		gkeProjectID:         opt.Account.GkeProjectID,
		location:             opt.Region,
		computeServiceClient: computeServiceClient,
	}, nil
}

func getComputeServiceClient(ctx context.Context, credentialContent string) (*compute.Service, error) {
	ts, err := GetTokenSource(ctx, credentialContent)
	if err != nil {
		return nil, fmt.Errorf("getComputeServiceClient failed: %v", err)
	}

	service, err := compute.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, ts)))
	if err != nil {
		return nil, fmt.Errorf("getComputeServiceClient failed: %v", err)
	}

	return service, nil
}

// ListRegions list regions
func (c *ComputeServiceClient) ListRegions(ctx context.Context) ([]*proto.RegionInfo, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ListRegions failed: gkeProjectId is required")
	}

	regions, err := c.computeServiceClient.Regions.List(c.gkeProjectID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	result := make([]*proto.RegionInfo, 0)
	for _, v := range regions.Items {
		if v.Name != "" && v.Description != "" {
			result = append(result, &proto.RegionInfo{
				Region:      v.Name,
				RegionName:  v.Description,
				RegionState: v.Status,
			})
		}
	}
	return result, nil
}

// ListZones list zones
func (c *ComputeServiceClient) ListZones(ctx context.Context) ([]*proto.ZoneInfo, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ListZones failed: gkeProjectId is required")
	}

	zones, err := c.computeServiceClient.Zones.List(c.gkeProjectID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("ListZones failed: %v", err)
	}

	result := make([]*proto.ZoneInfo, 0)
	for _, v := range zones.Items {
		if v.Name != "" && v.Description != "" {
			result = append(result, &proto.ZoneInfo{
				ZoneID:    strconv.FormatUint(v.Id, 10),
				Zone:      v.Name,
				ZoneName:  v.Description,
				ZoneState: v.Status,
			})
		}
	}
	return result, nil
}

// GetZone list zones
func (c *ComputeServiceClient) GetZone(ctx context.Context, name string) (*proto.ZoneInfo, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ListZones failed: gkeProjectId is required")
	}

	zone, err := c.computeServiceClient.Zones.Get(c.gkeProjectID, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("ListZones failed: %v", err)
	}
	result := &proto.ZoneInfo{
		ZoneID:    strconv.FormatUint(zone.Id, 10),
		Zone:      zone.Name,
		ZoneName:  zone.Description,
		ZoneState: zone.Status,
	}
	return result, nil
}

// GetInstanceGroupManager get instanceGroupManager
func (c *ComputeServiceClient) GetInstanceGroupManager(ctx context.Context, locationType, name string) (
	*compute.InstanceGroupManager, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("GetZoneInstanceGroupManager failed: gkeProjectId is required")
	}
	if c.location == "" {
		return nil, fmt.Errorf("GetZoneInstanceGroupManager failed: location is required")
	}

	var igm *compute.InstanceGroupManager
	var err error
	switch locationType {
	case locationTypeZones:
		igm, err = c.computeServiceClient.InstanceGroupManagers.Get(c.gkeProjectID, c.location, name).Context(ctx).Do()
	case locationTypeRegions:
		igm, err = c.computeServiceClient.RegionInstanceGroupManagers.Get(c.gkeProjectID, c.location, name).Context(ctx).Do()
	default:
		return nil, fmt.Errorf("GetZoneInstanceGroupManager failed: location type is neither zones nor regions")
	}
	if err != nil {
		return nil, fmt.Errorf("GetZoneInstanceGroupManager failed: %v", err)
	}

	return igm, nil
}

// PatchInstanceGroupManager update zonal instanceGroupManager
func (c *ComputeServiceClient) PatchInstanceGroupManager(ctx context.Context, locationType, name string,
	igm *compute.InstanceGroupManager) (*compute.Operation, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("UpdateZoneInstanceGroupManager failed: gkeProjectId is required")
	}
	if c.location == "" {
		return nil, fmt.Errorf("UpdateZoneInstanceGroupManager failed: location is required")
	}

	var operation *compute.Operation
	var err error
	switch locationType {
	case locationTypeZones:
		operation, err = c.computeServiceClient.InstanceGroupManagers.Patch(c.gkeProjectID, c.location, name, igm).
			Context(ctx).Do()
	case locationTypeRegions:
		operation, err = c.computeServiceClient.RegionInstanceGroupManagers.Patch(c.gkeProjectID, c.location, name, igm).
			Context(ctx).Do()
	default:
		return nil, fmt.Errorf("UpdateZoneInstanceGroupManager failed: location type is neither zones nor regions")
	}
	if err != nil {
		return nil, fmt.Errorf("UpdateZoneInstanceGroupManager failed: %v", err)
	}

	blog.Infof("UpdateZoneInstanceGroupManager operation ID: %s", operation.SelfLink)
	return operation, nil
}

// ResizeInstanceGroupManager set instanceGroupManager size
func (c *ComputeServiceClient) ResizeInstanceGroupManager(ctx context.Context, locationType, name string, size int64) (
	*compute.Operation, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ResizeZoneInstanceGroupManager failed: gkeProjectId is required")
	}
	if c.location == "" {
		return nil, fmt.Errorf("ResizeZoneInstanceGroupManager failed: location is required")
	}

	var operation *compute.Operation
	var err error
	switch locationType {
	case locationTypeZones:
		operation, err = c.computeServiceClient.InstanceGroupManagers.
			Resize(c.gkeProjectID, c.location, name, size).Context(ctx).Do()
	case locationTypeRegions:
		operation, err = c.computeServiceClient.RegionInstanceGroupManagers.
			Resize(c.gkeProjectID, c.location, name, size).Context(ctx).Do()
	default:
		return nil, fmt.Errorf("ResizeZoneInstanceGroupManager failed: location type is neither zones nor regions")
	}
	if err != nil {
		return nil, fmt.Errorf("ResizeZoneInstanceGroupManager failed: %v", err)
	}

	blog.Infof("ResizeZoneInstanceGroupManager operation ID: %s", operation.SelfLink)
	return operation, nil
}

// GetInstanceTemplate get the instanceTemplate
func (c *ComputeServiceClient) GetInstanceTemplate(ctx context.Context, name string) (
	*compute.InstanceTemplate, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("GetInstanceTemplate failed: gkeProjectId is required")
	}
	it, err := c.computeServiceClient.InstanceTemplates.Get(c.gkeProjectID, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("GetInstanceTemplate failed: %v", err)
	}

	return it, nil
}

// CreateInstanceTemplate create a instanceTemplate
func (c *ComputeServiceClient) CreateInstanceTemplate(ctx context.Context, name string, it *compute.InstanceTemplate) (
	*compute.Operation, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("CreateInstanceTemplate failed: gkeProjectId is required")
	}
	operation, err := c.computeServiceClient.InstanceTemplates.Insert(c.gkeProjectID, it).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("CreateInstanceTemplate failed: %v", err)
	}

	blog.Infof("CreateInstanceTemplate operation ID: %s", operation.SelfLink)
	return operation, nil
}

// GetOperation get zonal operation
func (c *ComputeServiceClient) GetOperation(ctx context.Context, locationType, name string) (
	*compute.Operation, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("GetOperation failed: gkeProjectId is required")
	}
	if c.location == "" {
		return nil, fmt.Errorf("GetOperation failed: location is required")
	}

	var operation *compute.Operation
	var err error
	switch locationType {
	case locationTypeZones:
		operation, err = c.computeServiceClient.ZoneOperations.Get(c.gkeProjectID, c.location, name).Context(ctx).Do()
	case locationTypeRegions:
		operation, err = c.computeServiceClient.RegionOperations.Get(c.gkeProjectID, c.location, name).Context(ctx).Do()
	case "global":
		operation, err = c.computeServiceClient.GlobalOperations.Get(c.gkeProjectID, name).Context(ctx).Do()
	default:
		return nil, fmt.Errorf("GetOperation failed: location type is not in [zones,regions,global]")
	}
	if err != nil {
		return nil, fmt.Errorf("GetOperation failed: %v", err)
	}

	blog.Infof("GetOperation operation ID: %s", operation.SelfLink)
	return operation, nil
}

// ListInstanceGroupsInstances list instances of instance group
func (c *ComputeServiceClient) ListInstanceGroupsInstances(ctx context.Context, locationType, name string) (
	[]*compute.InstanceWithNamedPorts, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ListInstanceGroupsInstances failed: gkeProjectId is required")
	}
	if c.location == "" {
		return nil, fmt.Errorf("ListInstanceGroupsInstances failed: location is required")
	}

	var (
		zoneInstance   *compute.InstanceGroupsListInstances
		regionInstance *compute.RegionInstanceGroupsListInstances
		insts          []*compute.InstanceWithNamedPorts
		err            error
	)
	switch locationType {
	case locationTypeZones:
		req := &compute.InstanceGroupsListInstancesRequest{}
		zoneInstance, err = c.computeServiceClient.InstanceGroups.ListInstances(c.gkeProjectID, c.location, name, req).
			Context(ctx).Do()
		insts = zoneInstance.Items
	case locationTypeRegions:
		req := &compute.RegionInstanceGroupsListInstancesRequest{}
		regionInstance, err = c.computeServiceClient.RegionInstanceGroups.
			ListInstances(c.gkeProjectID, c.location, name, req).Context(ctx).Do()
		insts = regionInstance.Items
	default:
		return nil, fmt.Errorf("ListInstanceGroupsInstances failed: location type is neither zones nor regions")
	}
	if err != nil {
		return nil, fmt.Errorf("ListInstanceGroupsInstances failed: %v", err)
	}

	return insts, nil
}

// GetZoneInstance get zonal instance
func (c *ComputeServiceClient) GetZoneInstance(ctx context.Context, location, name string) (
	*compute.Instance, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("GetZoneInstance failed: gkeProjectId is required")
	}
	instance, err := c.computeServiceClient.Instances.Get(c.gkeProjectID, location, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("GetZoneInstance failed: %v", err)
	}

	return instance, nil
}

// InstanceNameFilter filter instances by name
func InstanceNameFilter(nameList []string) string {
	cond := make([]string, 0)
	for _, n := range nameList {
		n = "(name = " + n + ")"
		cond = append(cond, n)
	}
	return strings.Join(cond, " OR ")
}

// ListZoneInstanceWithFilter list filtered zonal instances
func (c *ComputeServiceClient) ListZoneInstanceWithFilter(ctx context.Context, location, filter string) (
	*compute.InstanceList, error) {
	if c.gkeProjectID == "" {
		return nil, fmt.Errorf("ListZoneInstanceWithFilter failed: gkeProjectId is required")
	}
	req := c.computeServiceClient.Instances.List(c.gkeProjectID, location)
	instanceList, err := req.Filter(filter).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("ListZoneInstanceWithFilter failed: %v", err)
	}

	return instanceList, nil
}

// RemoveNodeFromMIG remove nodes from MIG, but the nodes still in cluster
func (c *ComputeServiceClient) RemoveNodeFromMIG(ctx context.Context, location, name string, nodes []string) error {
	if c.gkeProjectID == "" {
		return fmt.Errorf("RemoveNodeFromMIG failed: gkeProjectId is required")
	}
	instances := make([]string, 0)
	for _, ins := range nodes {
		instances = append(instances, fmt.Sprintf("zones/%s/instances/%s", location, ins))
	}
	req := &compute.InstanceGroupManagersAbandonInstancesRequest{
		Instances: instances,
	}
	operation, err := c.computeServiceClient.InstanceGroupManagers.
		AbandonInstances(c.gkeProjectID, location, name, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("RemoveNodeFromMIG failed: %v", err)
	}

	blog.Infof("RemoveNodeFromMIG operation ID: %s", operation.SelfLink)
	return nil
}

// DeleteInstancesInMIG delete instances from MIG
func (c *ComputeServiceClient) DeleteInstancesInMIG(ctx context.Context, location, name string, nodes []string) error {
	if c.gkeProjectID == "" {
		return fmt.Errorf("DeleteInstancesInMIG failed: gkeProjectId is required")
	}
	instances := make([]string, 0)
	for _, ins := range nodes {
		instances = append(instances, fmt.Sprintf("zones/%s/instances/%s", location, ins))
	}
	req := &compute.InstanceGroupManagersDeleteInstancesRequest{
		Instances: instances,
	}
	operation, err := c.computeServiceClient.InstanceGroupManagers.
		DeleteInstances(c.gkeProjectID, location, name, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("DeleteInstancesInMIG failed: %v", err)
	}

	blog.Infof("DeleteInstancesInMIG operation ID: %s", operation.SelfLink)
	return nil
}
