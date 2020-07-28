// Copyright 2020-2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package clusterstore_test

import (
	"fmt"
	"testing"

	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/sclevine/spec"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pivotal/build-service-cli/pkg/commands/clusterstore"
	"github.com/pivotal/build-service-cli/pkg/testhelpers"
)

func TestStatusCommand(t *testing.T) {
	spec.Run(t, "TestStatusCommand", testStatusCommand)
}

func testStatusCommand(t *testing.T, when spec.G, it spec.S) {
	cmdFunc := func(clientSet *fake.Clientset) *cobra.Command {
		clientSetProvider := testhelpers.GetFakeKpackClusterProvider(clientSet)
		return clusterstore.NewStatusCommand(clientSetProvider)
	}

	when("the store exists", func() {
		const storeName = "some-store-name"
		store := &expv1alpha1.ClusterStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: storeName,
			},
			Status: expv1alpha1.ClusterStoreStatus{
				Buildpacks: []expv1alpha1.StoreBuildpack{
					{
						BuildpackInfo: expv1alpha1.BuildpackInfo{
							Id:      "meta",
							Version: "1",
						},
						Buildpackage: expv1alpha1.BuildpackageInfo{
							Id:       "meta",
							Version:  "1",
							Homepage: "meta-1-buildpackage-homepage",
						},
						StoreImage: expv1alpha1.StoreImage{
							Image: "some-meta-image",
						},
						Homepage: "meta-homepage",
						Order: []expv1alpha1.OrderEntry{
							{
								Group: []expv1alpha1.BuildpackRef{
									{
										BuildpackInfo: expv1alpha1.BuildpackInfo{
											Id:      "nested-buildpack",
											Version: "2",
										},
										Optional: true,
									},
								},
							},
						},
					},
					{
						BuildpackInfo: expv1alpha1.BuildpackInfo{
							Id:      "nested-buildpack",
							Version: "2",
						},
						Buildpackage: expv1alpha1.BuildpackageInfo{
							Id:       "meta",
							Version:  "1",
							Homepage: "meta-1-buildpackage-homepage",
						},
						StoreImage: expv1alpha1.StoreImage{
							Image: "some-meta-image",
						},
						Homepage: "nested-buildpack-homepage",
					},
					{
						BuildpackInfo: expv1alpha1.BuildpackInfo{
							Id:      "simple-buildpack",
							Version: "3",
						},
						Buildpackage: expv1alpha1.BuildpackageInfo{
							Id:       "simple-buildpack",
							Version:  "3",
							Homepage: "simple-3-buildpackage-homepage",
						},
						StoreImage: expv1alpha1.StoreImage{
							Image: "simple-buildpackage",
						},
						Homepage: "simple-buildpack-homepage",
					},
				},
			},
		}

		it("returns store details", func() {
			const expectedOutput = `BUILDPACKAGE ID     VERSION    HOMEPAGE
meta                1          meta-1-buildpackage-homepage
simple-buildpack    3          simple-3-buildpackage-homepage

`
			testhelpers.CommandTest{
				Objects:        append([]runtime.Object{store}),
				Args:           []string{storeName},
				ExpectedOutput: expectedOutput,
			}.TestKpack(t, cmdFunc)
		})

		it("includes buildpacks and detection order when --verbose flag is used", func() {
			const expectedOutput = `Buildpackage:    meta@1
Image:           some-meta-image
Homepage:        meta-homepage

BUILDPACK ID        VERSION    HOMEPAGE
nested-buildpack    2          nested-buildpack-homepage

DETECTION ORDER       
Group #1              
  nested-buildpack    (Optional)


Buildpackage:    simple-buildpack@3
Image:           simple-buildpackage
Homepage:        simple-buildpack-homepage

BUILDPACK ID    VERSION    HOMEPAGE

DETECTION ORDER    

`
			testhelpers.CommandTest{
				Objects:        append([]runtime.Object{store}),
				Args:           []string{storeName, "--verbose"},
				ExpectedOutput: expectedOutput,
			}.TestKpack(t, cmdFunc)
		})
	})

	when("the store does not exist", func() {
		it("returns a message that there is no store", func() {
			const storeName = "non-existent-store"
			testhelpers.CommandTest{
				Args:           []string{storeName},
				ExpectErr:      true,
				ExpectedOutput: fmt.Sprintf("Error: clusterstores.experimental.kpack.pivotal.io %q not found\n", storeName),
			}.TestKpack(t, cmdFunc)
		})
	})
}
