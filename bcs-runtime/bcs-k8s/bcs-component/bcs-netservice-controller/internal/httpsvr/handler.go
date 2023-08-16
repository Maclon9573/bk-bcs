package httpsvr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/internal/constant"
	"github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/internal/utils"
	coreV1 "k8s.io/api/core/v1"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	v1 "github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/emicklei/go-restful"
)

// HttpServerClient http server client
type HttpServerClient struct {
	K8SClient client.Client
}

// NetIPAllocateRequest represents allocate BCSNetIP request
type NetIPAllocateRequest struct {
	Host         string `json:"host"`
	ContainerID  string `json:"containerID"`
	IPAddr       string `json:"ipAddr,omitempty"`
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

// NetIPDeleteRequest represents delete BCSNetIP request
type NetIPDeleteRequest struct {
	Host         string `json:"host"`
	ContainerID  string `json:"containerID"`
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

// NetIPResponse represents allocate/delete BCSNetIP response
type NetIPResponse struct {
	Code      uint        `json:"code"`
	Message   string      `json:"message"`
	Result    bool        `json:"result"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
}

func responseData(code uint, m string, result bool, reqID string, data interface{}) *NetIPResponse {
	return &NetIPResponse{
		Code:      code,
		Message:   m,
		Result:    result,
		RequestID: reqID,
		Data:      data,
	}
}

// InitRouters init router
func InitRouters(ws *restful.WebService, httpServerClient *HttpServerClient) {
	ws.Route(ws.POST("/v1/allocator").To(httpServerClient.allocateIP))
	ws.Route(ws.DELETE("/v1/allocator").To(httpServerClient.deleteIP))
}

func (c *HttpServerClient) allocateIP(request *restful.Request, response *restful.Response) {
	requestID := request.Request.Header.Get("X-Request-Id")
	netIPReq := &NetIPAllocateRequest{}
	if err := request.ReadEntity(netIPReq); err != nil {
		blog.Errorf("decode json request failed, %s", err.Error())
		response.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAllocateNetIPReq(netIPReq); err != nil {
		response.WriteEntity(responseData(1, err.Error(), false, requestID, nil))
		return
	}

	if netIPReq.IPAddr != "" {
		netIP, err := c.getIPFromAddr(netIPReq)
		if err != nil {
			response.WriteEntity(responseData(2, err.Error(), false, requestID, nil))
		}
		if err := c.K8SClient.Status().Update(context.Background(), netIP); err != nil {
			message := fmt.Sprintf("update IP [%s] status failed", netIPReq.IPAddr)
			blog.Errorf(message)
			response.WriteEntity(responseData(2, message, false, requestID, nil))
			return
		}
		message := fmt.Sprintf("allocate ip [%s] for container %s success", netIPReq.IPAddr, netIPReq.ContainerID)
		blog.Infof(message)
		response.WriteEntity(responseData(0, message, true, requestID, netIPReq))
		return
	}

	// ip address not exists in request
	netPoolList := &v1.BCSNetPoolList{}
	if err := c.K8SClient.List(context.Background(), netPoolList); err != nil {
		message := fmt.Sprintf("get BCSNetPool list failed, %s", err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}

	availableFixedIP, availableUnfixedIP, err := c.getAvailableIPs(netPoolList, netIPReq)
	if err != nil {
		response.WriteEntity(responseData(2, err.Error(), false, requestID, nil))
		return
	}

	// get primaryKey from pod annotations
	primaryKey, err := c.getPrimaryKey(netIPReq.PodNamespace, netIPReq.PodName)
	if err != nil {
		message := fmt.Sprintf("get BCSNetIP [%s] primaryKey failed, %s", netIPReq.IPAddr, err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}

	// get fix IP info from pod annotations
	fixed, expired, err := c.isFixedIP(netIPReq.PodNamespace, netIPReq.PodName)
	if err != nil {
		message := fmt.Sprintf("check BCSNetIP [%s] fixed status failed, %s", netIPReq.IPAddr, err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}

	// match IP by primaryKey or podName and podNamespace. If not matched, get IP from available unfixed IP
	if fixed {
		ip, err := c.getUnexpiredFixedIP(primaryKey, expired, fixed, netIPReq, availableFixedIP)
		if err != nil {
			response.WriteEntity(responseData(2, err.Error(), false, requestID, nil))
		}
		if ip != nil {
			data := netIPReq
			data.IPAddr = ip.Name
			message := fmt.Sprintf("allocate IP [%s] for Host %s success", ip.Name, netIPReq.Host)
			blog.Infof(message)
			response.WriteEntity(responseData(0, message, true, requestID, data))
			return
		}
	}

	if len(availableUnfixedIP) == 0 {
		message := fmt.Sprintf("no available IP for pod %s/%s", netIPReq.PodNamespace, netIPReq.PodName)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	if err := c.updateIPStatus(availableUnfixedIP[0], netIPReq, primaryKey, expired, fixed); err != nil {
		response.WriteEntity(responseData(2, err.Error(), false, requestID, nil))
		return
	}
	message := fmt.Sprintf("allocate IP [%s] for Host %s success", availableUnfixedIP[0].Name, netIPReq.Host)
	blog.Infof(message)
	data := netIPReq
	data.IPAddr = availableUnfixedIP[0].Name
	response.WriteEntity(responseData(0, message, true, requestID, data))
}

func (c *HttpServerClient) getUnexpiredFixedIP(primaryKey, expired string, fixed bool, netIPReq *NetIPAllocateRequest,
	availableFixedIP []*v1.BCSNetIP) (*v1.BCSNetIP, error) {
	if primaryKey != "" {
		for _, ip := range availableFixedIP {
			if ip.Status.PodPrimaryKey == primaryKey {
				if err := c.updateIPStatus(ip, netIPReq, primaryKey, expired, fixed); err != nil {
					return nil, err
				}
				return ip, nil
			}
		}
	}
	for _, ip := range availableFixedIP {
		if ip.Status.PodNamespace == netIPReq.PodNamespace && ip.Status.PodName == netIPReq.PodName {
			if err := c.updateIPStatus(ip, netIPReq, "", expired, fixed); err != nil {
				return nil, err
			}
			return ip, nil
		}
	}
	return nil, nil
}

func (c *HttpServerClient) updateIPStatus(ip *v1.BCSNetIP, netIPReq *NetIPAllocateRequest, primaryKey, expired string,
	fixed bool) error {
	ip.Status = v1.BCSNetIPStatus{
		Status:        constant.ActiveStatus,
		Host:          netIPReq.Host,
		ContainerID:   netIPReq.ContainerID,
		Fixed:         fixed,
		PodPrimaryKey: primaryKey,
		PodNamespace:  netIPReq.PodNamespace,
		PodName:       netIPReq.PodName,
		UpdateTime:    metav1.Now(),
		KeepDuration:  expired,
	}
	if err := c.K8SClient.Status().Update(context.Background(), ip); err != nil {
		message := fmt.Sprintf("update IP [%s] status failed", netIPReq.IPAddr)
		blog.Errorf(message)
		return errors.New(message)
	}
	if fixed {
		ip.Labels[constant.FixIPLabel] = "true"
		if err := c.K8SClient.Update(context.Background(), ip); err != nil {
			message := fmt.Sprintf("set IP [%s] label failed", netIPReq.IPAddr)
			blog.Errorf(message)
			return errors.New(message)
		}
	}
	return nil
}

func (c *HttpServerClient) getAvailableIPs(netPoolList *v1.BCSNetPoolList, netIPReq *NetIPAllocateRequest) (
	[]*v1.BCSNetIP, []*v1.BCSNetIP, error) {
	var availableFixedIP, availableUnfixedIP []*v1.BCSNetIP
	found := false
	for _, pool := range netPoolList.Items {
		if utils.StringInSlice(pool.Spec.Hosts, netIPReq.Host) {
			found = true
			for _, v := range pool.Spec.AvailableIPs {
				netIP := &v1.BCSNetIP{}
				if err := c.K8SClient.Get(context.Background(), types.NamespacedName{Name: v}, netIP); err != nil {
					blog.Warnf("get BCSNetIP [%s] failed, %s", v, err.Error())
					continue
				}
				if netIP.Status.Status == constant.AvailableStatus {
					if !netIP.Status.Fixed {
						availableUnfixedIP = append(availableUnfixedIP, netIP)
					} else {
						availableFixedIP = append(availableFixedIP, netIP)
					}
				}
			}
		}
	}
	if !found {
		message := fmt.Sprintf("host %s does not exist in pools", netIPReq.Host)
		blog.Errorf(message)
		return nil, nil, errors.New(message)
	}
	//if len(availableFixedIP) == 0 {
	//	message := fmt.Sprintf("no available ip in pools for host %s", netIPReq.Host)
	//	blog.Errorf(message)
	//	return nil, nil, errors.New(message)
	//}
	return availableFixedIP, availableUnfixedIP, nil
}

func (c *HttpServerClient) getIPFromAddr(netIPReq *NetIPAllocateRequest) (*v1.BCSNetIP, error) {
	netIP := &v1.BCSNetIP{}
	if err := c.K8SClient.Get(context.Background(), types.NamespacedName{Name: netIPReq.IPAddr}, netIP); err != nil {
		message := fmt.Sprintf("get BCSNetIP [%s] failed, %s", netIPReq.IPAddr, err.Error())
		blog.Errorf(message)
		return nil, errors.New(message)
	}
	if netIP.Status.Status == constant.ActiveStatus {
		message := fmt.Sprintf("the requested IP [%s] is in use", netIPReq.IPAddr)
		blog.Errorf(message)
		return nil, errors.New(message)
	}
	fixed, keepDuration, err := c.isFixedIP(netIPReq.PodNamespace, netIPReq.PodName)
	if err != nil {
		message := fmt.Sprintf("check BCSNetIP [%s] fixed status failed, %s", netIPReq.IPAddr, err.Error())
		blog.Errorf(message)
		return nil, errors.New(message)
	}
	primaryKey, err := c.getPrimaryKey(netIPReq.PodNamespace, netIPReq.PodName)
	if err != nil {
		message := fmt.Sprintf("get BCSNetIP [%s] primaryKey failed, %s", netIPReq.IPAddr, err.Error())
		blog.Errorf(message)
		return nil, errors.New(message)
	}
	netIP.Status = v1.BCSNetIPStatus{
		Status:        constant.ActiveStatus,
		Host:          netIPReq.Host,
		ContainerID:   netIPReq.ContainerID,
		PodPrimaryKey: primaryKey,
		PodNamespace:  netIPReq.PodNamespace,
		PodName:       netIPReq.PodName,
		Fixed:         fixed,
		UpdateTime:    metav1.Now(),
		KeepDuration:  keepDuration,
	}
	return netIP, nil
}

func (c *HttpServerClient) deleteIP(request *restful.Request, response *restful.Response) {
	requestID := request.Request.Header.Get("X-Request-Id")
	netIPReq := &NetIPDeleteRequest{}
	if err := request.ReadEntity(netIPReq); err != nil {
		blog.Errorf("decode json request failed, %s", err.Error())
		response.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDeleteNetIPReq(netIPReq); err != nil {
		response.WriteEntity(responseData(1, err.Error(), false, requestID, nil))
		return
	}

	netIPList := &v1.BCSNetIPList{}
	if err := c.K8SClient.List(context.Background(), netIPList); err != nil {
		message := fmt.Sprintf("get BCSNetIP list failed, %s", err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	var netIP *v1.BCSNetIP
	for _, ip := range netIPList.Items {
		if ip.Status.ContainerID == netIPReq.ContainerID && ip.Status.PodNamespace == netIPReq.PodNamespace &&
			ip.Status.PodName == netIPReq.PodName {
			netIP = &ip
			break
		}
	}
	if netIP == nil {
		message := fmt.Sprintf("didn't find related BCSNetIP instance for container %s", netIPReq.ContainerID)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	fixed, _, err := c.isFixedIP(netIPReq.PodNamespace, netIPReq.PodName)
	if err != nil {
		message := fmt.Sprintf("check BCSNetIP [%s] fixed status failed, %s", netIP.Name, err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	if fixed {
		netIP.Status.Status = constant.AvailableStatus
		netIP.Status.UpdateTime = metav1.Now()
	} else {
		netIP.Status = v1.BCSNetIPStatus{
			Status:     constant.AvailableStatus,
			UpdateTime: metav1.Now(),
		}
	}

	if err := c.K8SClient.Status().Update(context.Background(), netIP); err != nil {
		message := fmt.Sprintf("update IP [%s] status failed", netIP.Name)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	message := fmt.Sprintf("deactive IP [%s] success, it's available now", netIP.Name)
	blog.Errorf(message)
	response.WriteEntity(responseData(0, message, true, requestID, netIPReq))
}

func (c *HttpServerClient) isFixedIP(namespace, name string) (bool, string, error) {
	pod := &coreV1.Pod{}
	err := c.K8SClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, pod)
	if err != nil {
		return false, "", err
	}

	fixedIPValue, ok := pod.ObjectMeta.Annotations[constant.PodAnnotationKeyFixIP]
	expiredValue, ok2 := pod.ObjectMeta.Annotations[constant.PodAnnotationKeyForExpiredDurationSeconds]

	if !ok {
		return false, "", nil
	}
	if fixedIPValue != constant.PodAnnotationValueFixIP {
		return false, "", fmt.Errorf("invalid fix ip value %s for pod %s/%s", fixedIPValue, namespace, name)
	}
	if !ok2 {
		return true, constant.DefaultFixedIPKeepDurationStr, nil
	}
	duration, err := time.ParseDuration(expiredValue)
	if err != nil {
		blog.Errorf("invalid fixed ip keep duration %s for pod %s/%s", expiredValue, namespace, name)
		return true, "", err
	}
	if duration > constant.MaxFixedIPKeepDuration {
		blog.Errorf("fixed ip keep duration %s for pod %s/%s is bigger than %f hours", expiredValue,
			namespace, name, constant.MaxFixedIPKeepDuration)
		return true, "", err
	}
	return true, expiredValue, nil
}

func (c *HttpServerClient) getPrimaryKey(namespace, name string) (string, error) {
	pod := &coreV1.Pod{}
	err := c.K8SClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, pod)
	if err != nil {
		return "", err
	}
	value, ok := pod.ObjectMeta.Annotations[constant.PodAnnotationKeyForPrimaryKey]
	if !ok {
		return "", nil
	}
	return value, nil
}

func validateAllocateNetIPReq(netIPReq *NetIPAllocateRequest) error {
	var message string
	if netIPReq == nil {
		message = "lost request body for allocating ip"
		blog.Errorf(message)
		return errors.New(message)
	}
	if netIPReq.Host == "" || netIPReq.ContainerID == "" {
		message = "lost Host/ContainerID info in request"
		blog.Errorf(message)
		return errors.New(message)
	}
	if netIPReq.PodNamespace == "" || netIPReq.PodName == "" {
		message = "lost PodNamespace/PodName info in request"
		blog.Errorf(message)
		return errors.New(message)
	}
	return nil
}

func validateDeleteNetIPReq(netIPReq *NetIPDeleteRequest) error {
	var message string
	if netIPReq == nil {
		message = "lost request body for allocating ip"
		blog.Errorf(message)
		return errors.New(message)
	}
	if netIPReq.Host == "" || netIPReq.ContainerID == "" {
		message = "lost Host/ContainerID info in request"
		blog.Errorf(message)
		return errors.New(message)
	}
	if netIPReq.PodNamespace == "" || netIPReq.PodName == "" {
		message = "lost PodNamespace/PodName info in request"
		blog.Errorf(message)
		return errors.New(message)
	}
	return nil
}
