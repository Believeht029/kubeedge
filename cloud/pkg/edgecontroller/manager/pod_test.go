/*
Copyright 2021 The KubeEdge Authors.

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

package manager

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/kubeedge/api/apis/componentconfig/cloudcore/v1alpha1"
	"github.com/kubeedge/kubeedge/cloud/pkg/common/client"
	"github.com/kubeedge/kubeedge/cloud/pkg/common/informers"
)

func TestIsPodUpdated(t *testing.T) {
	type args struct {
		old *v1.Pod
		new *v1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"TestPodManager_isPodUpdated(): Case 1: check different pod",
			args{
				TestOldPodObject,
				TestNewPodObject,
			},
			true,
		},
		{
			"TestPodManager_isPodUpdated(): Case 2: check same pod",
			args{
				TestOldPodObject,
				TestOldPodObject,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodUpdated(*tt.args.old, *tt.args.new); got != tt.want {
				t.Errorf("PodManager.isPodUpdated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodManager_merge(t *testing.T) {
	type fields struct {
		realEvents   chan watch.Event
		mergedEvents chan watch.Event
	}
	type args struct {
		event watch.Event
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"TestPodManager_merge(): Case 1: Add pod, deletiontimestamp is nil",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Added, Object: TestOldPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 2: Add pod, deletiontimestamp is not nil",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Added, Object: TestDeletingPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 3: Modified pod",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Modified, Object: TestNewPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 4: Modified pod, new pod is same with old pod",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Modified, Object: TestOldPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 5: Modified pod, pod not exist",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Modified, Object: TestNewPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 6: Deleted pod",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: watch.Deleted, Object: TestOldPodObject},
			},
		},
		{
			"TestPodManager_merge(): Case 7: Invalid event type",
			fields{
				realEvents:   make(chan watch.Event, 1),
				mergedEvents: make(chan watch.Event, 1),
			},
			args{
				event: watch.Event{Type: "InvalidType", Object: TestOldPodObject},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PodManager{
				realEvents:   tt.fields.realEvents,
				mergedEvents: tt.fields.mergedEvents,
			}

			tt.fields.realEvents <- tt.args.event
			go pm.merge()
		})
	}
}

func TestPodManager_Events(t *testing.T) {
	type fields struct {
		realEvents   chan watch.Event
		mergedEvents chan watch.Event
	}
	mergeEventsCh := make(chan watch.Event, 1)
	tests := []struct {
		name   string
		fields fields
		want   chan watch.Event
	}{
		{
			"TestPodManager_Events(): Case 1",
			fields{
				make(chan watch.Event, 1),
				mergeEventsCh,
			},
			mergeEventsCh,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PodManager{
				realEvents:   tt.fields.realEvents,
				mergedEvents: tt.fields.mergedEvents,
			}
			if got := pm.Events(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PodManager.Events() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPodManager(t *testing.T) {
	type args struct {
		informer cache.SharedIndexInformer
	}

	config := &v1alpha1.EdgeController{
		Buffer: &v1alpha1.EdgeControllerBuffer{
			ConfigMapEvent: 1024,
		},
	}

	tmpfile, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tmpfile.Name())
	if err := os.WriteFile(tmpfile.Name(), []byte(mockKubeConfigContent), 0666); err != nil {
		t.Error(err)
	}
	client.InitKubeEdgeClient(&v1alpha1.KubeAPIConfig{
		KubeConfig:  tmpfile.Name(),
		QPS:         100,
		Burst:       200,
		ContentType: "application/vnd.kubernetes.protobuf",
	}, false)

	client.DefaultGetRestMapper = func() (mapper meta.RESTMapper, err error) { return nil, nil }

	tests := []struct {
		name string
		args args
	}{
		{
			"TestNewPodManager(): Case 1: with nodename",
			args{
				informers.GetInformersManager().GetKubeInformerFactory().Core().V1().Pods().Informer(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = NewPodManager(config, tt.args.informer)
			assert.NoError(t, err)
		})
	}
}
