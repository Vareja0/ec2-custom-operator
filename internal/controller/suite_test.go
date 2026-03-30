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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	computev1 "github.com/vareja0/operator-repo/api/v1"
	// +kubebuilder:scaffold:imports
)

var (
	ctx                 context.Context
	cancel              context.CancelFunc
	testEnv             *envtest.Environment
	cfg                 *rest.Config
	k8sClient           client.Client
	localstackContainer *localstack.LocalStackContainer
	awsEndpoint         string
	testSubnetID        string
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("starting LocalStack container")
	var err error
	localstackContainer, err = localstack.Run(ctx, "localstack/localstack:3.0",
		testcontainers.WithEnv(map[string]string{"SERVICES": "ec2"}),
	)
	Expect(err).NotTo(HaveOccurred())

	host, err := localstackContainer.Host(ctx)
	Expect(err).NotTo(HaveOccurred())
	port, err := localstackContainer.MappedPort(ctx, "4566/tcp")
	Expect(err).NotTo(HaveOccurred())
	awsEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())

	Expect(os.Setenv("AWS_ACCESS_KEY_ID", "test")).To(Succeed())
	Expect(os.Setenv("AWS_SECRET_ACCESS_KEY", "test")).To(Succeed())

	By("creating VPC and subnet in LocalStack")
	ec2Client, err := newEC2Client(ctx, "us-east-1", awsEndpoint)
	Expect(err).NotTo(HaveOccurred())

	vpcOut, err := ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	Expect(err).NotTo(HaveOccurred())

	subnetOut, err := ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            vpcOut.Vpc.VpcId,
		CidrBlock:        aws.String("10.0.1.0/24"),
		AvailabilityZone: aws.String("us-east-1a"),
	})
	Expect(err).NotTo(HaveOccurred())
	testSubnetID = aws.ToString(subnetOut.Subnet.SubnetId)

	err = computev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Eventually(func() error {
		return testEnv.Stop()
	}, time.Minute, time.Second).Should(Succeed())

	if localstackContainer != nil {
		Expect(testcontainers.TerminateContainer(localstackContainer)).To(Succeed())
	}

	Expect(os.Unsetenv("AWS_ACCESS_KEY_ID")).To(Succeed())
	Expect(os.Unsetenv("AWS_SECRET_ACCESS_KEY")).To(Succeed())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}
