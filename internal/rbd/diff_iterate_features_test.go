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

package rbd

import (
	"testing"
)

func TestHasDiffIterateFeatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features string
		want     bool
	}{
		{"empty", "", false},
		{"layering only", "layering", false},
		{"missing fast-diff", "layering,exclusive-lock,object-map", false},
		{"missing exclusive-lock", "layering,object-map,fast-diff", false},
		{"all required", "exclusive-lock,object-map,fast-diff", true},
		{"all required with others", "layering,deep-flatten,exclusive-lock,object-map,fast-diff", true},
		{"all required with spaces", "layering, exclusive-lock, object-map, fast-diff", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasDiffIterateFeatures(tt.features); got != tt.want {
				t.Errorf("HasDiffIterateFeatures(%q) = %t, want %t", tt.features, got, tt.want)
			}
		})
	}
}

func TestImageChainLinkString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		link imageChainLink
		want string
	}{
		{
			name: "image head",
			link: imageChainLink{pool: "rbd", imageName: "csi-vol-1", imageID: "abc"},
			want: "rbd/csi-vol-1",
		},
		{
			name: "image head in namespace",
			link: imageChainLink{pool: "rbd", namespace: "ns", imageName: "csi-vol-1", imageID: "abc"},
			want: "rbd/ns/csi-vol-1",
		},
		{
			name: "snapshot",
			link: imageChainLink{
				pool: "rbd", imageName: "csi-vol-1-temp", imageID: "abc",
				isSnap: true, snapID: 4, snapName: "csi-vol-1",
			},
			want: "rbd/csi-vol-1-temp@csi-vol-1<id:4>",
		},
		{
			name: "image without name",
			link: imageChainLink{pool: "rbd", imageID: "abc", isSnap: true, snapID: 7},
			want: "rbd/<id:abc>@<id:7>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.link.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestObjectMapLinks(t *testing.T) {
	t.Parallel()

	child := &imageChainLink{pool: "pool", imageName: "csi-vol-1", imageID: "c1"}
	parent := &imageChainLink{
		pool:      "pool",
		imageName: "csi-vol-1-temp",
		imageID:   "p1",
		isSnap:    true,
		snapID:    4,
		snapName:  "csi-vol-1",
	}

	links := objectMapLinks(child, nil)
	if len(links) != 1 || links[0] != child {
		t.Errorf("objectMapLinks() without parent = %v, want [%v]", links, child)
	}

	links = objectMapLinks(child, parent)
	want := []string{"pool/csi-vol-1-temp@csi-vol-1<id:4>", "pool/csi-vol-1-temp", "pool/csi-vol-1"}
	if len(links) != len(want) {
		t.Fatalf("objectMapLinks() = %v, want %v", links, want)
	}
	for i := range want {
		if links[i].String() != want[i] {
			t.Errorf("objectMapLinks()[%d] = %s, want %s", i, links[i], want[i])
		}
	}
	if !parent.isSnap {
		t.Error("objectMapLinks() modified the parent link")
	}
}
