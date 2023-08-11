package httpsvr

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	v1 "github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/emicklei/go-restful"
)

// HttpServerClient http server client
type HttpServerClient struct {
	client.Client
}

type NetIPAllocateRequest struct {
	Host         string `json:"host"`
	ContainerID  string `json:"containerID"`
	IPAddr       string `json:"ipAddr,omitempty"`
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

type NetIPDeleteRequest struct {
	Host         string `json:"host"`
	ContainerID  string `json:"containerID"`
	PodName      string `json:"podName"`
	PodNamespace string `json:"podNamespace"`
}

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
	if err := validateAllocateNetIPReq(netIPReq, response, requestID); err != nil {
		return
	}

	if netIPReq.IPAddr != "" {
		netIP := &v1.BCSNetIP{}
		if err := c.Get(context.Background(), types.NamespacedName{Name: netIPReq.IPAddr}, netIP); err != nil {
			message := fmt.Sprintf("get BCSNetIP [%s] failed, %s", netIPReq.IPAddr, err.Error())
			blog.Errorf(message)
			response.WriteEntity(responseData(2, message, false, requestID, nil))
			return
		}
		if netIP.Status.Status == v1.ActiveStatus {
			message := fmt.Sprintf("the requested IP [%s] is in use", netIPReq.IPAddr)
			blog.Errorf(message)
			response.WriteEntity(responseData(2, message, false, requestID, nil))
			return
		}
		netIP.Status = v1.BCSNetIPStatus{
			Status:       v1.ActiveStatus,
			Host:         netIPReq.Host,
			ContainerID:  netIPReq.ContainerID,
			PodNamespace: netIPReq.PodNamespace,
			PodName:      netIPReq.PodName,
			UpdateTime:   metav1.Now(),
		}
		if err := c.Status().Update(context.Background(), netIP); err != nil {
			message := fmt.Sprintf("update IP [%s] status failed", netIPReq.IPAddr)
			blog.Errorf(message)
			response.WriteEntity(responseData(2, message, false, requestID, nil))
			return
		}
		message := fmt.Sprintf("allocate ip [%s] for container %s success", netIPReq.IPAddr, netIPReq.ContainerID)
		blog.Infof("allocate ip [%s] for container %s on Host %s success.", netIPReq.IPAddr, netIPReq.ContainerID,
			netIPReq.Host)
		response.WriteEntity(responseData(0, message, true, requestID, netIPReq))
		return
	}

	// if ip address not exists in request, allocate a random ip
	netPoolList := &v1.BCSNetPoolList{}
	if err := c.List(context.Background(), netPoolList); err != nil {
		message := fmt.Sprintf("get BCSNetPool list failed, %s", err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	var avilableIP []*v1.BCSNetIP
	found := false
	for _, pool := range netPoolList.Items {
		if stringInSlice(netIPReq.Host, pool.Spec.Hosts) {
			found = true
			for _, v := range pool.Spec.AvailableIPs {
				netIP := &v1.BCSNetIP{}
				if err := c.Get(context.Background(), types.NamespacedName{Name: v}, netIP); err != nil {
					blog.Warnf("get BCSNetIP [%s] failed, %s", v, err.Error())
					continue
				}
				if netIP.Status.Status == v1.AvailableStatus {
					avilableIP = append(avilableIP, netIP)
				}
			}
		}
	}
	if !found {
		message := fmt.Sprintf("host %s does not exist in pools", netIPReq.Host)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	if len(avilableIP) == 0 {
		message := fmt.Sprintf("no available ip in pools for host %s", netIPReq.Host)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	avilableIP[0].Status = v1.BCSNetIPStatus{
		Status:       v1.ActiveStatus,
		Host:         netIPReq.Host,
		ContainerID:  netIPReq.ContainerID,
		PodNamespace: netIPReq.PodNamespace,
		PodName:      netIPReq.PodName,
		UpdateTime:   metav1.Now(),
	}
	if err := c.Status().Update(context.Background(), avilableIP[0]); err != nil {
		message := fmt.Sprintf("update IP [%s] status failed", netIPReq.IPAddr)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	message := fmt.Sprintf("allocate ip [%s] for Host %s success", avilableIP[0].Name, netIPReq.Host)
	blog.Infof(message)
	data := netIPReq
	data.IPAddr = avilableIP[0].Name
	response.WriteEntity(responseData(0, message, true, requestID, data))
}

func (c *HttpServerClient) deleteIP(request *restful.Request, response *restful.Response) {
	requestID := request.Request.Header.Get("X-Request-Id")
	netIPReq := &NetIPDeleteRequest{}
	if err := request.ReadEntity(netIPReq); err != nil {
		blog.Errorf("decode json request failed, %s", err.Error())
		response.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDeleteNetIPReq(netIPReq, response, requestID); err != nil {
		return
	}

	netIPList := &v1.BCSNetIPList{}
	if err := c.List(context.Background(), netIPList); err != nil {
		message := fmt.Sprintf("get BCSNetIP list failed, %s", err.Error())
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	var netIP *v1.BCSNetIP
	for _, ip := range netIPList.Items {
		if ip.Status.ContainerID == netIPReq.ContainerID && ip.Status.Host == netIPReq.Host &&
			ip.Status.PodName == netIPReq.PodName {
			netIP = &ip
		}
	}
	if netIP == nil {
		message := fmt.Sprintf("didn't find related BCSNetIP instance for container %s", netIPReq.ContainerID)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	netIP.Status = v1.BCSNetIPStatus{
		Status:     v1.AvailableStatus,
		UpdateTime: metav1.Now(),
	}
	if err := c.Status().Update(context.Background(), netIP); err != nil {
		message := fmt.Sprintf("update IP [%s] status failed", netIP.Name)
		blog.Errorf(message)
		response.WriteEntity(responseData(2, message, false, requestID, nil))
		return
	}
	message := fmt.Sprintf("deactive IP [%s] success, it's available now", netIP.Name)
	blog.Errorf(message)
	response.WriteEntity(responseData(0, message, true, requestID, netIPReq))
}

func (c *HttpServerClient) updateIPStatus(netIPReq *NetIPAllocateRequest, netIPRes *NetIPResponse, response *restful.Response,
	netIP *v1.BCSNetIP) error {
	netIP.Status.Status = v1.ActiveStatus
	netIP.Status.Host = netIPReq.Host
	netIP.Status.ContainerID = netIPReq.ContainerID
	netIP.Status.PodNamespace = netIPReq.PodNamespace
	netIP.Status.PodName = netIPReq.PodName
	netIP.Status.UpdateTime = metav1.Now()
	if err := c.Status().Update(context.Background(), netIP); err != nil {
		blog.Errorf("update IP [%s] status failed", netIPReq.IPAddr)
		return fmt.Errorf("update IP [%s] status failed", netIPReq.IPAddr)
	}

	return nil
}

func validateAllocateNetIPReq(netIPReq *NetIPAllocateRequest, response *restful.Response, requestID string) error {
	if netIPReq == nil {
		message := "lost request body for allocating ip"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	if netIPReq.Host == "" || netIPReq.ContainerID == "" {
		message := "lost Host/ContainerID info in request"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	if netIPReq.PodNamespace == "" || netIPReq.PodName == "" {
		message := "lost PodNamespace/PodName info in request"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	return nil
}

func validateDeleteNetIPReq(netIPReq *NetIPDeleteRequest, response *restful.Response, requestID string) error {
	if netIPReq == nil {
		message := "lost request body for allocating ip"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	if netIPReq.Host == "" || netIPReq.ContainerID == "" {
		message := "lost Host/ContainerID info in request"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	if netIPReq.PodNamespace == "" || netIPReq.PodName == "" {
		message := "lost PodNamespace/PodName info in request"
		blog.Errorf(message)
		response.WriteEntity(responseData(1, message, false, requestID, nil))
		return errors.New(message)
	}
	return nil
}

// stringInSlice split string to slice
func stringInSlice(str string, strs []string) bool {
	for _, item := range strs {
		if str == item {
			return true
		}
	}
	return false
}
