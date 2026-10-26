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

package abort

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

const helperEnv = "CEPHCSI_ABORT_TEST_HELPER"

func TestHandleDeliversSIGABRT(t *testing.T) {
	t.Parallel()

	ch := make(chan os.Signal, 1)
	received := make(chan os.Signal, 1)
	handle(ch, func(sig os.Signal) {
		received <- sig
	})
	defer signal.Stop(ch)

	if err := syscall.Kill(os.Getpid(), syscall.SIGABRT); err != nil {
		t.Fatalf("failed to send SIGABRT: %v", err)
	}

	select {
	case sig := <-received:
		if sig != syscall.SIGABRT {
			t.Errorf("expected SIGABRT, got %v", sig)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SIGABRT was not delivered to the handler")
	}
}

// TestHandleSignalTerminates runs the test binary again with HandleSignal()
// registered, sends SIGABRT and verifies that the process exits by itself.
func TestHandleSignalTerminates(t *testing.T) {
	t.Parallel()

	if os.Getenv(helperEnv) == "1" {
		HandleSignal()
		_ = syscall.Kill(os.Getpid(), syscall.SIGABRT)
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestHandleSignalTerminates$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected the process to exit with an error, got %v, output:\n%s", err, out)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("expected exit code 2 from panic, got %d, output:\n%s", exitErr.ExitCode(), out)
	}
	if !strings.Contains(string(out), "panic: received signal") {
		t.Errorf("expected panic message in output:\n%s", out)
	}
}
