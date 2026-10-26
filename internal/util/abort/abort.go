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

// Package abort makes sure the process terminates when SIGABRT is raised,
// even when it runs as PID 1 inside a container.
package abort

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/ceph/ceph-csi/internal/util/log"
)

// HandleSignal registers a handler for SIGABRT that panics, which terminates
// the process with a non-zero exit code and a traceback of all goroutines.
//
// Native libraries (librados, librbd, libcephfs) call abort() on failed
// assertions, often from threads that are not created by the Go runtime. For
// such threads the Go runtime resets SIGABRT to its default action and raises
// it again. When the process is the init process of a PID namespace (PID 1 in
// a container), the kernel discards that signal and the process hangs instead
// of getting restarted. Delivering SIGABRT through os/signal prevents the
// re-raise, and the handler terminates the process instead.
//
// This should be called as early as possible, before any native Ceph code is
// used.
func HandleSignal() {
	handle(make(chan os.Signal, 1), panicOnAbort)
}

func handle(ch chan os.Signal, onAbort func(os.Signal)) {
	signal.Notify(ch, syscall.SIGABRT)
	go func() {
		onAbort(<-ch)
	}()
}

func panicOnAbort(sig os.Signal) {
	log.ErrorLogMsg("received signal %q, possibly abort() was called by a native library, terminating", sig)
	debug.SetTraceback("all")
	panic(fmt.Sprintf("received signal %q", sig))
}
