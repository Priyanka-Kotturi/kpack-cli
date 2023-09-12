// Copyright 2020-Present VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/kmeta"
)

func TestWaiter(t *testing.T) {
	spec.Run(t, "Waiter", testWaiter)
}

func init() {
	v1alpha2.AddToScheme(scheme.Scheme)
}

func testWaiter(t *testing.T, when spec.G, it spec.S) {
	var (
		watcher       *TestWatcher
		generation    int64 = 2
		dynamicClient       = dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		waiter              = NewWaiter(dynamicClient, 2*time.Second)
	)

	when("Wait", func() {
		var resourceToWatch *v1alpha2.Builder

		it.Before(func() {
			resourceToWatch = &v1alpha2.Builder{
				TypeMeta: v1.TypeMeta{
					Kind: "Builder",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:            "some-name",
					Namespace:       "some-namespace",
					ResourceVersion: "1",
					Generation:      generation,
				},
			}
			watcher = &TestWatcher{
				events:           make(chan watch.Event, 100),
				expectedResource: resourceToWatch,
			}
			dynamicClient.PrependWatchReactor("builders", watcher.watchReactor)
		})

		it("returns no error when resource is already ready", func() {
			resourceToWatch.Status = v1alpha2.BuilderStatus{
				Status: conditionReady(corev1.ConditionTrue, generation),
			}

			require.NoError(t, waiter.Wait(context.Background(), resourceToWatch))
		})

		it("returns an error when resource is already failed", func() {
			resourceToWatch.Status = v1alpha2.BuilderStatus{
				Status: conditionReady(corev1.ConditionFalse, generation),
			}

			require.EqualError(t, waiter.Wait(context.Background(), resourceToWatch), "Builder \"some-name\" not ready: some-message")
		})

		it("waits for the correct generation", func() {
			resourceToWatch.Status = v1alpha2.BuilderStatus{
				Status: conditionReady(corev1.ConditionFalse, generation-1),
			}

			builderObj := &v1alpha2.Builder{
				TypeMeta:   resourceToWatch.TypeMeta,
				ObjectMeta: resourceToWatch.ObjectMeta,
				Status:     v1alpha2.BuilderStatus{Status: conditionReady(corev1.ConditionTrue, generation)},
			}

			content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(builderObj)
			if err != nil {
				panic(err)
			}
			watcher.addEvent(watch.Event{
				Type:   watch.Modified,
				Object: &unstructured.Unstructured{Object: content},
			})

			require.NoError(t, waiter.Wait(context.Background(), resourceToWatch))
		})

		it("runs extra condition checks", func() {
			fakeConditionChecker := fakeConditionChecker{}
			resourceToWatch.Status = v1alpha2.BuilderStatus{
				Status: conditionReady(corev1.ConditionFalse, generation-1),
			}

			builderObj := &v1alpha2.Builder{
				TypeMeta:   resourceToWatch.TypeMeta,
				ObjectMeta: resourceToWatch.ObjectMeta,
				Status:     v1alpha2.BuilderStatus{Status: conditionReady(corev1.ConditionTrue, generation)},
			}

			content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(builderObj)
			if err != nil {
				panic(err)
			}
			watcher.addEvent(watch.Event{
				Type:   watch.Modified,
				Object: &unstructured.Unstructured{Object: content},
			})

			require.NoError(t, waiter.Wait(context.Background(), resourceToWatch, fakeConditionChecker.conditionCheck))
			require.True(t, fakeConditionChecker.called)
		})

		it("recovers from too old resource version error", func() {
			watcher.addEvent(watch.Event{
				Type: watch.Error,
				Object: &v1.Status{
					TypeMeta: v1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Status",
					},
					Status:  "Failure",
					Message: "too old resource version: 23358 (23360)",
					Reason:  "Expired",
					Code:    410,
				},
			})

			builderObj := &v1alpha2.Builder{
				TypeMeta:   resourceToWatch.TypeMeta,
				ObjectMeta: resourceToWatch.ObjectMeta,
				Status:     v1alpha2.BuilderStatus{Status: conditionReady(corev1.ConditionTrue, generation)},
			}
			content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(builderObj)
			if err != nil {
				panic(err)
			}
			watcher.addEvent(watch.Event{
				Type:   watch.Modified,
				Object: &unstructured.Unstructured{Object: content},
			})

			require.NoError(t, waiter.Wait(context.Background(), resourceToWatch))
		})
	})
}

type fakeConditionChecker struct {
	called bool
}

func (cc *fakeConditionChecker) conditionCheck(_ watch.Event) (bool, error) {
	cc.called = true
	return true, nil
}

func conditionReady(status corev1.ConditionStatus, generation int64) corev1alpha1.Status {
	return corev1alpha1.Status{
		ObservedGeneration: generation,
		Conditions: []corev1alpha1.Condition{
			{
				Type:    corev1alpha1.ConditionReady,
				Status:  status,
				Message: "some-message",
			},
		},
	}
}

type TestWatcher struct {
	events           chan watch.Event
	expectedResource kmeta.OwnerRefable
}

func (t *TestWatcher) addEvent(event watch.Event) {
	t.events <- event
}

func (t *TestWatcher) Stop() {
}

func (t *TestWatcher) ResultChan() <-chan watch.Event {
	return t.events
}

func (t *TestWatcher) watchReactor(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
	if t.expectedResource == nil {
		return true, nil, errors.New("test watcher must be configured with an expected resource to be used")
	}

	watchAction := action.(clientgotesting.WatchAction)
	if watchAction.GetNamespace() != t.expectedResource.GetObjectMeta().GetNamespace() {
		return true, nil, errors.New("expected watch on namespace")
	}

	match, found := watchAction.GetWatchRestrictions().Fields.RequiresExactMatch("metadata.name")
	if !found {
		return true, nil, errors.New("expected watch on name")
	}
	if match != t.expectedResource.GetObjectMeta().GetName() {
		return true, nil, errors.New("expected watch on name")
	}

	return true, t, nil
}
