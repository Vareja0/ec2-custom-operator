/*
Copyright 2026.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	computev1 "github.com/vareja0/operator-repo/api/v1"
)

var _ = Describe("EC2instance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		testCtx := context.Background()

		namespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the EC2instance custom resource")
			existing := &computev1.EC2instance{}
			err := k8sClient.Get(testCtx, namespacedName, existing)
			if err != nil && errors.IsNotFound(err) {
				resource := &computev1.EC2instance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: computev1.EC2instanceSpec{
						InstanceName:      resourceName,
						Type:              "t3.micro",
						Region:            "us-east-1",
						AvaibilityZone:    "us-east-1a",
						AmiID:             "ami-12345678",
						Subnet:            "subnet-12345678",
						SshKey:            "test-key",
						AssociatePublicIP: false,
						Storage: computev1.StorageConfig{
							RootVoume: computev1.VolumeConfig{
								DeviceName: "/dev/xvda",
								Size:       8,
								Type:       "gp3",
								Encrypted:  true,
							},
						},
					},
				}
				Expect(k8sClient.Create(testCtx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("cleaning up the EC2instance resource")
			resource := &computev1.EC2instance{}
			err := k8sClient.Get(testCtx, namespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(testCtx, resource)).To(Succeed())
		})

		It("should create an EC2 instance and populate status", func() {
			By("reconciling the created resource")
			reconciler := &EC2instanceReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				AWSEndpoint: awsEndpoint,
			}

			// First reconcile adds the finalizer
			_, err := reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile creates the EC2 instance
			_, err = reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("verifying the status was populated with instance details")
			updated := &computev1.EC2instance{}
			Expect(k8sClient.Get(testCtx, namespacedName, updated)).To(Succeed())
			Expect(updated.Status.InstanceID).NotTo(BeEmpty(), "instanceID should be set after reconcile")
			Expect(updated.Status.Status).To(Equal("running"))
		})

		It("should be idempotent when reconciling a running instance", func() {
			reconciler := &EC2instanceReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				AWSEndpoint: awsEndpoint,
			}

			By("running the full creation flow")
			_, err := reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			instance := &computev1.EC2instance{}
			Expect(k8sClient.Get(testCtx, namespacedName, instance)).To(Succeed())
			firstInstanceID := instance.Status.InstanceID
			Expect(firstInstanceID).NotTo(BeEmpty())

			By("reconciling again and expecting no replacement")
			_, err = reconciler.Reconcile(testCtx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(testCtx, namespacedName, instance)).To(Succeed())
			Expect(instance.Status.InstanceID).To(Equal(firstInstanceID), "instance should not be replaced when spec is unchanged")
		})
	})
})
