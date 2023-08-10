package controllers

import (
	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	v1 "github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IPFilter filter for BCSNetIP event
type IPFilter struct {
	filterName string
}

// NewIPFilter create bcsNetIP filter
func NewIPFilter() *IPFilter {
	return &IPFilter{
		filterName: "bcsNetIP",
	}
}

var _ handler.EventHandler = &IPFilter{}

func (f *IPFilter) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	//TODO implement me
}

func (f *IPFilter) Update(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	//TODO implement me
}

func (f *IPFilter) Delete(event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	ip, ok := event.Object.(*v1.BCSNetIP)
	if !ok {
		blog.Warnf("recv delete object is not BCSNetIP, event %+v", event)
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: ip.Spec.Pool,
	}})
}

func (f *IPFilter) Generic(event event.GenericEvent, q workqueue.RateLimitingInterface) {
	//TODO implement me
}
