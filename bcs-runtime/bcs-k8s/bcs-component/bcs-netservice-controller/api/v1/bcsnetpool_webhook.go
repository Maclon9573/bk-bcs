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

package v1

import (
	"context"
	"errors"
	"fmt"
	"net"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type bcsNetPoolClient struct {
	client client.Client
}

func (r *BCSNetPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(&bcsNetPoolClient{client: mgr.GetClient()}).
		WithValidator(&bcsNetPoolClient{client: mgr.GetClient()}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-netservice-bkbcs-tencent-com-v1-bcsnetpool,mutating=true,failurePolicy=fail,sideEffects=None,groups=netservice.bkbcs.tencent.com,resources=bcsnetpools,verbs=create;update,versions=v1,name=mbcsnetpool.kb.io,admissionReviewVersions=v1

var _ admission.CustomDefaulter = &bcsNetPoolClient{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (c *bcsNetPoolClient) Default(ctx context.Context, obj runtime.Object) error {
	// TODO(user): fill in your defaulting logic.
	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-netservice-bkbcs-tencent-com-v1-bcsnetpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=netservice.bkbcs.tencent.com,resources=bcsnetpools,verbs=create;update;delete,versions=v1,name=vbcsnetpool.kb.io,admissionReviewVersions=v1

var _ admission.CustomValidator = &bcsNetPoolClient{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *bcsNetPoolClient) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	pool, ok := obj.(*BCSNetPool)
	if !ok {
		return errors.New("object is not BCSNetPool")
	}

	blog.Infof("validate create pool %s", pool.Name)
	if net.ParseIP(pool.Spec.Net) == nil {
		return errors.New(fmt.Sprintf("spec.net %s is not valid when creating bcsnetpool %s", pool.Spec.Net, pool.Name))
	}
	if net.ParseIP(pool.Spec.Gateway) == nil {
		return errors.New(fmt.Sprintf("spec.gateway is not valid %s when creating bcsnetpool %s", pool.Spec.Gateway, pool.Name))
	}
	for _, ip := range pool.Spec.AvailableIPs {
		if net.ParseIP(ip) == nil {
			return errors.New(fmt.Sprintf("%s in spec.availableIPs is not valid when creating bcsnetpool %s", ip, pool.Name))
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *bcsNetPoolClient) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) error {
	pool, ok := newObj.(*BCSNetPool)
	if !ok {
		return errors.New("object is not BCSNetPool")
	}
	oldPool, ok := oldObj.(*BCSNetPool)
	if !ok {
		return errors.New("object is not BCSNetPool")
	}

	blog.Infof("validate update pool %s", pool.Name)
	if net.ParseIP(pool.Spec.Net) == nil {
		return errors.New(fmt.Sprintf("spec.net %s is not valid when updating bcsnetpool %s", pool.Spec.Net, pool.Name))
	}
	if net.ParseIP(pool.Spec.Gateway) == nil {
		return errors.New(fmt.Sprintf("spec.gateway is not valid %s when updating bcsnetpool %s", pool.Spec.Gateway, pool.Name))
	}
	for _, ip := range pool.Spec.AvailableIPs {
		if net.ParseIP(ip) == nil {
			return errors.New(fmt.Sprintf("%s in spec.availableIPs is not valid when updating bcsnetpool %s", ip, pool.Name))
		}
	}

	// 找出更新操作中要删除的已存在的IP
	var delIPList []string
	newIPMap := make(map[string]bool)
	for _, v := range pool.Spec.AvailableIPs {
		newIPMap[v] = true
	}

	for _, v := range oldPool.Spec.AvailableIPs {
		if _, exists := newIPMap[v]; !exists {
			delIPList = append(delIPList, v)
		}
	}

	if err := c.checkActiveIP(ctx, delIPList, pool); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *bcsNetPoolClient) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	pool, ok := obj.(*BCSNetPool)
	if !ok {
		return errors.New("object is not BCSNetPool")
	}
	blog.Infof("validate delete pool %s", pool.Name)

	if err := c.checkActiveIP(ctx, pool.Spec.AvailableIPs, pool); err != nil {
		return err
	}
	return nil
}

func (c *bcsNetPoolClient) checkActiveIP(ctx context.Context, s []string, pool *BCSNetPool) error {
	for _, ip := range s {
		netIP := &BCSNetIP{}
		if err := c.client.Get(ctx, types.NamespacedName{Name: ip}, netIP); err != nil {
			if k8serrors.IsNotFound(err) {
				blog.Warnf("BCSNetIP %s missing in pool %s", ip, pool.Name)
				continue
			}
			return err
		}
		if netIP.Status.Status == ActiveStatus {
			return errors.New(fmt.Sprintf("can not delete pool %s, active IP %s exists", pool.Name, ip))
		}
	}
	return nil
}

func (c *bcsNetPoolClient) ValidateVerification() error {
	return nil
}
