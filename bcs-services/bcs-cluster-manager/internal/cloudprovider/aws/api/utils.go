package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/utils"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"

	proto "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/api/clustermanager"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider"
	cutils "github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/cloudprovider/utils"
	"github.com/Tencent/bk-bcs/bcs-services/bcs-cluster-manager/internal/types"
)

const (
	// ResourceTypeLaunchTemplate is a ResourceType enum value
	ResourceTypeLaunchTemplate = "launch-template"

	limit = 100
)

var (
	// DefaultUserDataHeader default user data header for creating launch template userdata
	DefaultUserDataHeader = "MIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=\"==MYBOUNDARY==\"\n\n" +
		"-==MYBOUNDARY==\nContent-Type: text/x-shellscript; charset=\"us-ascii\"\n\n"
	// DefaultUserDataTail default user data tail for creating launch template userdata
	DefaultUserDataTail = "\n\n--==MYBOUNDARY==--"
)

var (
	// DeviceName device name list
	DeviceName = []string{"/dev/xvdb", "/dev/xvdc", "/dev/xvdd", "/dev/xvde", "/dev/xvdf", "/dev/xvdg", "/dev/xvdh",
		"/dev/xvdi", "/dev/xvdj", "/dev/xvdk", "/dev/xvdl", "/dev/xvdm", "/dev/xvdn", "/dev/xvdo", "/dev/xvdp",
		"/dev/xvdq", "/dev/xvdr", "/dev/xvds", "/dev/xvdt", "/dev/xvdu", "/dev/xvdv", "/dev/xvdw", "/dev/xvdx",
		"/dev/xvdy", "/dev/xvdz", "/dev/xvdba", "/dev/xvdbb", "/dev/xvdbc", "/dev/xvdbd", "/dev/xvdbe", "/dev/xvdbf",
		"/dev/xvdbg", "/dev/xvdbh", "/dev/xvdbi", "/dev/xvdbj", "/dev/xvdbk", "/dev/xvdbl", "/dev/xvdbm", "/dev/xvdbn",
		"/dev/xvdbo", "/dev/xvdbp", "/dev/xvdbq", "/dev/xvdbr", "/dev/xvdbs", "/dev/xvdbt", "/dev/xvdbu", "/dev/xvdbv",
		"/dev/xvdbw", "/dev/xvdbx", "/dev/xvdby", "/dev/xvdbz"}
)

// AWSClientSet aws client set
type AWSClientSet struct {
	*AutoScalingClient
	*EC2Client
	*EksClient
	*IAMClient
}

// NewSession generates a new aws session
func NewSession(opt *cloudprovider.CommonOption) (*session.Session, error) {
	if opt == nil || opt.Account == nil || len(opt.Account.SecretID) == 0 || len(opt.Account.SecretKey) == 0 {
		return nil, cloudprovider.ErrCloudCredentialLost
	}
	if len(opt.Region) == 0 {
		return nil, cloudprovider.ErrCloudRegionLost
	}

	awsConf := &aws.Config{}
	awsConf.Region = aws.String(opt.Region)
	awsConf.Credentials = credentials.NewStaticCredentials(opt.Account.SecretID, opt.Account.SecretKey, "")

	sess, err := session.NewSession(awsConf)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// NewAWSClientSet creates a aws client set
func NewAWSClientSet(opt *cloudprovider.CommonOption) (*AWSClientSet, error) {
	sess, err := NewSession(opt)
	if err != nil {
		return nil, err
	}

	iam, err := NewIAMClient(opt)
	if err != nil {
		return nil, err
	}

	return &AWSClientSet{
		AutoScalingClient: &AutoScalingClient{asClient: autoscaling.New(sess)},
		EC2Client:         &EC2Client{ec2.New(sess)},
		EksClient:         &EksClient{eks.New(sess)},
		IAMClient:         &IAMClient{iam.iamClient},
	}, nil
}

// GetClusterKubeConfig constructs the cluster kubeConfig
func GetClusterKubeConfig(opt *cloudprovider.CommonOption, cluster *eks.Cluster) (string, error) {
	sess, err := NewSession(opt)
	if err != nil {
		return "", err
	}
	generator, err := token.NewGenerator(false, false)
	if err != nil {
		return "", err
	}

	awsToken, err := generator.GetWithOptions(&token.GetTokenOptions{
		Session:   sess,
		ClusterID: *cluster.Name,
	})
	if err != nil {
		return "", err
	}

	decodedCA, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return "", err
	}

	restConfig := &rest.Config{
		Host: *cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: decodedCA,
		},
		BearerToken: awsToken.Token,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	cert, err := base64.StdEncoding.DecodeString(*cluster.CertificateAuthority.Data)
	if err != nil {
		return "", fmt.Errorf("GetClusterKubeConfig invalid certificate failed, cluster=%s: %w", *cluster.Name, err)
	}

	saToken, err := cutils.GenerateSATokenByRestConfig(context.Background(), restConfig)
	if err != nil {
		return "", fmt.Errorf("getClusterKubeConfig generate k8s serviceaccount token failed,cluster=%s: %w",
			*cluster.Name, err)
	}

	typesConfig := &types.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []types.NamedCluster{
			{
				Name: *cluster.Name,
				Cluster: types.ClusterInfo{
					Server:                   "https://" + *cluster.Endpoint,
					CertificateAuthorityData: cert,
				},
			},
		},
		AuthInfos: []types.NamedAuthInfo{
			{
				Name: *cluster.Name,
				AuthInfo: types.AuthInfo{
					Token: saToken,
				},
			},
		},
		Contexts: []types.NamedContext{
			{
				Name: *cluster.Name,
				Context: types.Context{
					Cluster:  *cluster.Name,
					AuthInfo: *cluster.Name,
				},
			},
		},
		CurrentContext: *cluster.Name,
	}

	configByte, err := json.Marshal(typesConfig)
	if err != nil {
		return "", fmt.Errorf("GetClusterKubeConfig marsh kubeconfig failed, %v", err)
	}

	return base64.StdEncoding.EncodeToString(configByte), nil
}

// MapToTaints converts a map of string-string to a slice of Taint
func MapToTaints(taints []*proto.Taint) []*Taint {
	result := make([]*Taint, 0)
	for _, v := range taints {
		key := v.Key
		value := v.Value
		effect := v.Effect
		result = append(result, &Taint{Key: &key, Value: &value, Effect: &effect})
	}
	return result
}

// MapToAwsTaints converts a map of string-string to a slice of aws Taint
func MapToAwsTaints(taints []*proto.Taint) []*eks.Taint {
	result := make([]*eks.Taint, 0)
	for _, v := range taints {
		key := v.Key
		value := v.Value
		effect := v.Effect
		result = append(result, &eks.Taint{Key: &key, Value: &value, Effect: &effect})
	}
	return result
}

func generateAwsCreateNodegroupInput(input *CreateNodegroupInput) *eks.CreateNodegroupInput {
	newInput := &eks.CreateNodegroupInput{
		ClusterName:   input.ClusterName,
		NodegroupName: input.NodegroupName,
		//AmiType:       input.AmiType,
		ScalingConfig: func(c *NodegroupScalingConfig) *eks.NodegroupScalingConfig {
			if c == nil {
				return nil
			}
			return &eks.NodegroupScalingConfig{
				DesiredSize: c.DesiredSize,
				MaxSize:     c.MaxSize,
				MinSize:     c.MinSize,
			}
		}(input.ScalingConfig),
		NodeRole:     input.NodeRole,
		Labels:       input.Labels,
		Tags:         input.Tags,
		CapacityType: input.CapacityType,
	}
	newInput.Taints = make([]*eks.Taint, 0)
	for _, v := range input.Taints {
		newInput.Taints = append(newInput.Taints, &eks.Taint{
			Key:    v.Key,
			Value:  v.Value,
			Effect: v.Effect,
		})
	}
	if input.LaunchTemplate != nil {
		newInput.LaunchTemplate = &eks.LaunchTemplateSpecification{
			Id:      input.LaunchTemplate.Id,
			Name:    input.LaunchTemplate.Name,
			Version: input.LaunchTemplate.Version,
		}
	}
	return newInput
}

// CreateTagSpecs creates tag specs
func CreateTagSpecs(instanceTags map[string]*string) []*ec2.LaunchTemplateTagSpecificationRequest {
	if len(instanceTags) == 0 {
		return nil
	}

	tags := make([]*ec2.Tag, 0)
	for key, value := range instanceTags {
		tags = append(tags, &ec2.Tag{Key: aws.String(key), Value: value})
	}
	return []*ec2.LaunchTemplateTagSpecificationRequest{
		{
			ResourceType: aws.String(ec2.ResourceTypeInstance),
			Tags:         tags,
		},
	}
}

func generateAwsCreateLaunchTemplateInput(input *CreateLaunchTemplateInput) *ec2.CreateLaunchTemplateInput {
	awsInput := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: input.LaunchTemplateName,
		TagSpecifications:  generateAwsTagSpecs(input.TagSpecifications),
	}
	if input.LaunchTemplateData != nil {
		awsInput.LaunchTemplateData = &ec2.RequestLaunchTemplateData{
			ImageId:      input.LaunchTemplateData.ImageId,
			InstanceType: input.LaunchTemplateData.InstanceType,
			KeyName:      input.LaunchTemplateData.KeyName,
			UserData:     input.LaunchTemplateData.UserData,
		}
	}
	return awsInput
}

func generateAwsTagSpecs(tagSpecs []*TagSpecification) []*ec2.TagSpecification {
	if tagSpecs == nil {
		return nil
	}
	awsTagSpecs := make([]*ec2.TagSpecification, 0)
	for _, t := range tagSpecs {
		awsTagSpecs = append(awsTagSpecs, &ec2.TagSpecification{
			ResourceType: t.ResourceType,
			Tags: func(t []*Tag) []*ec2.Tag {
				awsTags := make([]*ec2.Tag, 0)
				for _, v := range t {
					awsTags = append(awsTags, &ec2.Tag{Key: v.Key, Value: v.Value})
				}
				return awsTags
			}(t.Tags),
		})
	}
	return awsTagSpecs
}

// ListNodesByInstanceID list node by instanceIDs
func ListNodesByInstanceID(ids []string, opt *cloudprovider.ListNodesOption) ([]*proto.Node, error) {
	idChunks := utils.SplitStringsChunks(ids, limit)
	nodeList := make([]*proto.Node, 0)

	blog.Infof("ListNodesByInstanceID ipChunks %+v", idChunks)
	for _, chunk := range idChunks {
		if len(chunk) > 0 {
			nodes, err := transInstanceIDsToNodes(chunk, opt)
			if err != nil {
				blog.Errorf("ListNodesByInstanceID failed: %v", err)
				return nil, err
			}
			if len(nodes) == 0 {
				continue
			}

			nodeList = append(nodeList, nodes...)
		}
	}

	return nodeList, nil
}

// transInstanceIDsToNodes trans IDList to Nodes
func transInstanceIDsToNodes(ids []string, opt *cloudprovider.ListNodesOption) ([]*proto.Node, error) {
	client, err := NewEC2Client(opt.Common)
	if err != nil {
		blog.Errorf("create ec2 client when GetNodeByIP failed, %s", err.Error())
		return nil, err
	}

	instances, err := client.DescribeInstances(&ec2.DescribeInstancesInput{InstanceIds: aws.StringSlice(ids)})
	if err != nil {
		blog.Errorf("ec2 client DescribeInstances len(%d) ip address failed, %s", len(ids), err.Error())
		return nil, err
	}
	blog.Infof("ec2 client DescribeInstances len(%d) ip response num %d", len(ids), len(instances))

	if len(instances) == 0 {
		// * no data response
		return nil, nil
	}
	if len(instances) != len(ids) {
		blog.Warnf("ec2 client DescribeInstances, expect %d, but got %d")
	}
	zoneInfo, err := client.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{AllAvailabilityZones: aws.Bool(true)})
	if err != nil {
		blog.Errorf("ec2 client DescribeAvailabilityZones failed: %v", err)
	}
	zoneMap := make(map[string]string)
	for _, z := range zoneInfo {
		zoneMap[*z.ZoneName] = *z.ZoneId
	}

	nodeMap := make(map[string]*proto.Node)
	var nodes []*proto.Node
	for _, inst := range instances {
		node := InstanceToNode(inst, zoneMap)
		// clean duplicated Node if user input multiple ip that
		// belong to one cvm instance
		if _, ok := nodeMap[node.NodeID]; ok {
			continue
		}

		nodeMap[node.NodeID] = node
		// default get first privateIP
		node.InnerIP = *inst.PrivateIpAddress
		node.Region = opt.Common.Region

		// check node vpc and cluster vpc
		if !strings.EqualFold(node.VPC, opt.ClusterVPCID) {
			return nil, fmt.Errorf(cloudprovider.ErrCloudNodeVPCDiffWithClusterResponse, node.InnerIP)
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// InstanceToNode parse Instance information in qcloud to Node in clustermanager
// @param Instance: qcloud instance information, can not be nil;
// @return Node: cluster-manager node information;
func InstanceToNode(inst *ec2.Instance, zoneInfo map[string]string) *proto.Node {
	var zoneID int
	if zoneInfo != nil {
		zoneID, _ = strconv.Atoi(zoneInfo[*inst.Placement.AvailabilityZone])
	}
	node := &proto.Node{
		NodeID:       *inst.InstanceId,
		InstanceType: *inst.InstanceType,
		CPU:          uint32(*inst.CpuOptions.CoreCount),
		GPU:          0,
		VPC:          *inst.VpcId,
		ZoneID:       *inst.Placement.AvailabilityZone,
		Zone:         uint32(zoneID),
	}
	return node
}
