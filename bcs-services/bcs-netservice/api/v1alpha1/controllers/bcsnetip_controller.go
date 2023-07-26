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
	"fmt"
	"strings"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tkexv1alpha1 "github.com/Tencent/bk-bcs/bcs-services/bcs-netservice/api/v1alpha1"
)

// BCSNetIPReconciler reconciles a BCSNetIP object
type BCSNetIPReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tkex.tencent.com,resources=bcsnetips,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tkex.tencent.com,resources=bcsnetips/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tkex.tencent.com,resources=bcsnetips/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BCSNetIP object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BCSNetIPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	blog.Infof("Reconciling %s", fmt.Sprintf("%s/%s", req.Name, req.Namespace))
	fmt.Printf("Reconciling %s\n", fmt.Sprintf("%s/%s", req.Name, req.Namespace))
	// 获取当前的 CR，并打印
	obj := &tkexv1alpha1.BCSNetIP{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			blog.Errorf("Unable to fetch object, %v", err)
		}
	} else {
		blog.Infof("Getting BCSNetIP %s", obj.Name)
		// 初始化 CR 的 Status 为 Running
		obj.Status.Status = "Active"
		if err := r.Status().Update(ctx, obj); err != nil {
			blog.Errorf("unable to update status, %v", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BCSNetIPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tkexv1alpha1.BCSNetIP{}).
		Complete(r)
}
