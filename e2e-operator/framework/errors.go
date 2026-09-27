/*
Copyright 2026 The Ceph-CSI Authors.

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

package framework

import (
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

// IsRetryableAPIError reports whether err may be transient and safe to retry.
func IsRetryableAPIError(err error) bool {
	// These errors may indicate a transient error that we can retry in tests.
	if apierrors.IsInternalError(err) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) ||
		utilnet.IsConnectionRefused(err) {
		return true
	}

	// If the error sends the Retry-After header, we respect it as an explicit confirmation we should retry.
	if _, shouldRetry := apierrors.SuggestsClientDelay(err); shouldRetry {
		return true
	}

	// "etcdserver: request timed out" does not seem to match the timeout errors above
	if strings.Contains(err.Error(), "etcdserver: request timed out") {
		return true
	}

	// "unable to upgrade connection" happens occasionally when executing commands in Pods
	if strings.Contains(err.Error(), "unable to upgrade connection") {
		return true
	}

	// "transport is closing" is an internal gRPC err, we can not use ErrConnClosing
	if strings.Contains(err.Error(), "transport is closing") {
		return true
	}

	// "transport: missing content-type field" is an error that sometimes
	// is returned while talking to the kubernetes-api-server. There does
	// not seem to be a public error constant for this.
	if strings.Contains(err.Error(), "transport: missing content-type field") {
		return true
	}

	// "pod nfs-820 does not have a host assigned" seems to get reported
	// when a Pod is not completely started yet, or was restarted while
	// trying to access it
	return strings.Contains(err.Error(), "does not have a host assigned")
}
