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

package session

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Tencent/bk-bcs/bcs-common/common/blog"
	traceconst "github.com/Tencent/bk-bcs/bcs-common/pkg/otel/trace/constants"
	"github.com/Tencent/bk-bcs/bcs-scenarios/bcs-gitops-manager/pkg/common"
	"github.com/Tencent/bk-bcs/bcs-scenarios/bcs-gitops-manager/pkg/proxy"
	"github.com/Tencent/bk-bcs/bcs-scenarios/bcs-gitops-manager/pkg/utils"
)

// MonitorSession defines the instance that to proxy to monitor server
type MonitorSession struct {
	op *proxy.MonitorOption
}

// NewMonitorSession create the session of monitor
func NewMonitorSession(op *proxy.MonitorOption) *MonitorSession {
	return &MonitorSession{
		op: op,
	}
}

// ServeHTTP http.Handler implementation
func (s *MonitorSession) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	requestID := req.Context().Value(traceconst.RequestIDHeaderKey).(string)
	// backend real path with encoded format
	realPath := strings.TrimPrefix(req.URL.RequestURI(), common.GitOpsProxyURL)
	// !force https link
	fullPath := fmt.Sprintf("http://bcsmonitorcontroller.bcs-system.svc.cluster.local:18088%s", realPath)
	newURL, err := url.Parse(fullPath)
	if err != nil {
		err = fmt.Errorf("monitor session build new fullpath '%s' failed: %w", fullPath, err)
		rw.WriteHeader(http.StatusInternalServerError)
		blog.Errorf(err.Error())
		_, _ = rw.Write([]byte(err.Error())) // nolint
		return
	}
	reverseProxy := httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL = newURL
		},
		ErrorHandler: func(res http.ResponseWriter, request *http.Request, e error) {
			if !utils.IsContextCanceled(e) {
				// metric.ManagerSecretProxyFailed.WithLabelValues().Inc()
			}
			blog.Errorf("RequestID[%s] monitor session proxy '%s' with header '%s' failure: %s",
				requestID, fullPath, request.Header, e.Error())
			res.WriteHeader(http.StatusInternalServerError)
			_, _ = res.Write([]byte("monitor session proxy failed")) // nolint
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint
		},
		ModifyResponse: func(r *http.Response) error {
			return nil
		},
	}

	req.Header.Set(traceconst.RequestIDHeaderKey, requestID)
	blog.Infof("RequestID[%s] monitor session serve: %s/%s", requestID, req.Method, fullPath)
	reverseProxy.ServeHTTP(rw, req)
}
