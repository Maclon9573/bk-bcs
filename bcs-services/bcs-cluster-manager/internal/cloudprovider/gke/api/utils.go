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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"

	errs "github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/container/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	bcsNamespace              = "bcs-system"
	clusterAdmin              = "cluster-admin"
	bcsCusterManager          = "bcs-cluster-manager"
	newClusterRoleBindingName = "bcs-system-cm-clusterRoleBinding"
)

// GetTokenSource gets token source from provided sa credential
func GetTokenSource(ctx context.Context, credential string) (oauth2.TokenSource, error) {
	ts, err := google.CredentialsFromJSON(ctx, []byte(credential), container.CloudPlatformScope)
	if err != nil {
		return nil, err
	}
	return ts.TokenSource, nil
}

func GenerateSAToken(restConfig *rest.Config) (string, error) {
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", fmt.Errorf("error creating clientset: %v", err)
	}

	return GenerateServiceAccountToken(clientSet)
}

// GenerateServiceAccountToken generate a serviceAccountToken for clusterAdmin given a rest clientset
func GenerateServiceAccountToken(clientset kubernetes.Interface) (string, error) {
	_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: bcsNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}

	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: bcsCusterManager,
		},
	}

	_, err = clientset.CoreV1().ServiceAccounts(bcsNamespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating service account: %v", err)
	}

	adminRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdmin,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	}
	clusterAdminRole, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), clusterAdmin, metav1.GetOptions{})
	if err != nil {
		clusterAdminRole, err = clientset.RbacV1().ClusterRoles().Create(context.TODO(), adminRole, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("error creating admin role: %v", err)
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: newClusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: bcsNamespace,
				APIGroup:  v1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterAdminRole.Name,
			APIGroup: rbacv1.GroupName,
		},
	}
	if _, err = clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("error creating role bindings: %v", err)
	}

	start := time.Millisecond * 250
	for i := 0; i < 5; i++ {
		time.Sleep(start)
		if serviceAccount, err = clientset.CoreV1().ServiceAccounts(bcsNamespace).Get(context.TODO(), serviceAccount.Name, metav1.GetOptions{}); err != nil {
			return "", fmt.Errorf("error getting service account: %v", err)
		}
		secret, err := CreateSecretForServiceAccount(context.TODO(), clientset, serviceAccount)
		if err != nil {
			return "", fmt.Errorf("error creating secret for service account: %v", err)
		}
		if token, ok := secret.Data["token"]; ok {
			return string(token), nil
		}
		start = start * 2
	}

	return "", errs.New("failed to fetch serviceAccountToken")
}

// CreateSecretForServiceAccount creates a service-account-token Secret for the provided Service Account.
// If the secret already exists, the existing one is returned.
func CreateSecretForServiceAccount(ctx context.Context, clientSet kubernetes.Interface, sa *v1.ServiceAccount) (*v1.Secret, error) {
	secretName := ServiceAccountSecretName(sa)
	secretClient := clientSet.CoreV1().Secrets(sa.Namespace)
	secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !apierror.IsNotFound(err) {
			return nil, err
		}
		sc := SecretTemplate(sa)
		secret, err = secretClient.Create(ctx, sc, metav1.CreateOptions{})
		if err != nil {
			if !apierror.IsAlreadyExists(err) {
				return nil, err
			}
			secret, err = secretClient.Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
		}
	}
	if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
		return secret, nil
	}
	blog.Errorf("createSecretForServiceAccount: waiting for secret [%s] to be populated with token", secretName)
	for {
		if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
			return secret, nil
		}
		time.Sleep(2 * time.Second)
		secret, err = secretClient.Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
}

// SecretTemplate generate a template of service-account-token Secret for the provided Service Account.
func SecretTemplate(sa *v1.ServiceAccount) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountSecretName(sa),
			Namespace: sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Name:       sa.Name,
					UID:        sa.UID,
				},
			},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
	}

}

// ServiceAccountSecretName returns the secret name for the given Service Account.
func ServiceAccountSecretName(sa *v1.ServiceAccount) string {
	return SafeConcatName(sa.Name, "token")
}

func SafeConcatName(name ...string) string {
	fullPath := strings.Join(name, "-")
	if len(fullPath) < 64 {
		return fullPath
	}
	digest := sha256.Sum256([]byte(fullPath))
	// since we cut the string in the middle, the last char may not be compatible with what is expected in k8s
	// we are checking and if necessary removing the last char
	c := fullPath[56]
	if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return fullPath[0:57] + "-" + hex.EncodeToString(digest[0:])[0:5]
	}

	return fullPath[0:56] + "-" + hex.EncodeToString(digest[0:])[0:6]
}
