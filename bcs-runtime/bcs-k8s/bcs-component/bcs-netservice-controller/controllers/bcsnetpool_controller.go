/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	netservicev1 "github.com/Tencent/bk-bcs/bcs-runtime/bcs-k8s/bcs-component/bcs-netservice-controller/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BCSNetPoolReconciler reconciles a BCSNetPool object
type BCSNetPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=netservice.bkbcs.tencent.com,resources=bcsnetpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=netservice.bkbcs.tencent.com,resources=bcsnetpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=netservice.bkbcs.tencent.com,resources=bcsnetips,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=netservice.bkbcs.tencent.com,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=netservice.bkbcs.tencent.com,resources=bcsnetpools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BCSNetPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	blog.V(3).Infof("BCSNetPool %+v triggered", req.NamespacedName)
	netPool := &netservicev1.BCSNetPool{}
	if err := r.Get(ctx, req.NamespacedName, netPool); err != nil {
		if k8serrors.IsNotFound(err) {
			blog.Infof("BCSNetPool %s deleted successfully", req.Name)
			return ctrl.Result{}, nil
		}
		blog.Errorf("get BCSNetPool %s failed, err %s", req.Name, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, err
	}
	if netPool.Status.Status == "" {
		blog.Infof("initializing BCSNetPool %s", req.Name)
		netPool.Status.Status = "Initializing"
		if err := r.Status().Update(ctx, netPool); err != nil {
			blog.Errorf("update BCSNetPool %s status failed, err %s", req.Name, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 5 * time.Second,
			}, err
		}
	}

	result, err := r.syncBCSNetIP(ctx, netPool)
	if err != nil {
		return result, err
	}

	// netPool is deleted
	if netPool.DeletionTimestamp != nil {
	}

	// TODO: 更新/删除时, Pool状态的判断
	netPool.Status.Status = "Normal"
	if err := r.Status().Update(ctx, netPool); err != nil {
		blog.Errorf("update BCSNetPool %s status failed, err %s", req.Name, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, err
	}
	blog.Infof("BCSNetPool %s status update successfully", req.Name)

	return ctrl.Result{}, nil
}

func (r *BCSNetPoolReconciler) syncBCSNetIP(ctx context.Context, netPool *netservicev1.BCSNetPool) (ctrl.Result, error) {
	// create BCSNetIP based on BCSNetPool if not exists
	for _, ip := range netPool.Spec.AvailableIPs {
		netIP := &netservicev1.BCSNetIP{}
		if err := r.Get(ctx, types.NamespacedName{Name: ip}, netIP); err != nil {
			if k8serrors.IsNotFound(err) {
				newNetIP := &netservicev1.BCSNetIP{
					ObjectMeta: metav1.ObjectMeta{Name: ip},
					Spec: netservicev1.BCSNetIPSpec{
						Pool:    netPool.Spec.Net,
						Mask:    netPool.Spec.Mask,
						Gateway: netPool.Spec.Gateway,
					},
				}
				if err := r.Create(ctx, newNetIP); err != nil {
					blog.Errorf("create BCSNetIP %s failed, err %s", ip, err.Error())
					return ctrl.Result{
						Requeue:      true,
						RequeueAfter: 5 * time.Second,
					}, err
				}
				blog.Infof("BCSNetIP %s created successfully", ip)

				newNetIP.Status.Status = "Available"
				if err := r.Status().Update(ctx, newNetIP); err != nil {
					blog.Errorf("update BCSNetIP %s status failed, err %s", ip, err.Error())
					return ctrl.Result{
						Requeue:      true,
						RequeueAfter: 5 * time.Second,
					}, err
				}
				blog.Infof("BCSNetIP %s status update successfully", ip)
				continue
			}
			blog.Errorf("get BCSNetIP %s failed, err %s", ip, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: 5 * time.Second,
			}, err
		}

	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BCSNetPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netservicev1.BCSNetPool{}).
		Complete(r)
}
