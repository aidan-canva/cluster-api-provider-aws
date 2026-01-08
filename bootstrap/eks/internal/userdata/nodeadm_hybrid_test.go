/*
Copyright 2024 The Kubernetes Authors.

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

package userdata

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	eksbootstrapv1 "sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
)

func TestHybridNodeadmUserdata(t *testing.T) {
	format.TruncatedDiff = false
	g := NewWithT(t)

	type args struct {
		input *HybridNodeadmInput
	}

	tests := []struct {
		name         string
		args         args
		expectErr    bool
		errContains  string
		verifyOutput func(output string) bool
	}{
		{
			name: "basic hybrid nodeadm userdata",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "MIME-Version: 1.0") &&
					strings.Contains(output, "name: test-cluster") &&
					strings.Contains(output, "region: us-west-2") &&
					strings.Contains(output, "activationId: test-activation-id") &&
					strings.Contains(output, "activationCode: test-activation-code") &&
					strings.Contains(output, "apiVersion: node.eks.aws/v1alpha1") &&
					strings.Contains(output, "hybrid:") &&
					strings.Contains(output, "ssm:")
			},
		},
		{
			name: "hybrid userdata does NOT contain apiServerEndpoint or certificateAuthority",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				// Hybrid nodes use region-based discovery, NOT explicit endpoints
				return !strings.Contains(output, "apiServerEndpoint") &&
					!strings.Contains(output, "certificateAuthority") &&
					strings.Contains(output, "region: us-west-2")
			},
		},
		{
			name: "with kubelet flags",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					KubeletFlags: []string{
						"--node-labels=node-type=hybrid,location=datacenter-1",
						"--register-with-taints=dedicated=hybrid:NoSchedule",
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "node-type=hybrid") &&
					strings.Contains(output, "location=datacenter-1") &&
					strings.Contains(output, "register-with-taints") &&
					strings.Contains(output, "flags:")
			},
		},
		{
			name: "with kubelet config",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					KubeletConfig: &runtime.RawExtension{
						Raw: []byte(`
maxPods: 110
evictionHard:
  memory.available: "500Mi"
`),
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "maxPods: 110") &&
					strings.Contains(output, "evictionHard:") &&
					strings.Contains(output, "memory.available")
			},
		},
		{
			name: "with containerd config",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					ContainerdConfig: `[plugins."io.containerd.grpc.v1.cri".containerd]
discard_unpacked_layers = false`,
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "containerd:") &&
					strings.Contains(output, "config:") &&
					strings.Contains(output, "discard_unpacked_layers")
			},
		},
		{
			name: "with pre-nodeadm commands",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					PreNodeadmCommands: []string{
						"echo 'Starting hybrid node setup...'",
						"yum install -y nfs-utils",
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "echo 'Starting hybrid node setup...'") &&
					strings.Contains(output, "yum install -y nfs-utils") &&
					strings.Contains(output, "#!/bin/bash") &&
					strings.Contains(output, "Content-Type: text/x-shellscript")
			},
		},
		{
			name: "with NTP configuration",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					NTP: &eksbootstrapv1.NTP{
						Enabled: ptr.To(true),
						Servers: []string{"time.google.com", "time.aws.com"},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "Content-Type: text/cloud-config") &&
					strings.Contains(output, "#cloud-config") &&
					strings.Contains(output, "time.google.com") &&
					strings.Contains(output, "time.aws.com")
			},
		},
		{
			name: "with users configuration",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					Users: []eksbootstrapv1.User{
						{
							Name:              "admin",
							SSHAuthorizedKeys: []string{"ssh-rsa AAAAB3..."},
						},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "Content-Type: text/cloud-config") &&
					strings.Contains(output, "admin") &&
					strings.Contains(output, "ssh-rsa")
			},
		},
		{
			name: "with files configuration",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					Files: []eksbootstrapv1.File{
						{
							Path:        "/etc/custom-config.yaml",
							Content:     "key: value",
							Permissions: "0644",
							Owner:       "root:root",
						},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "Content-Type: text/cloud-config") &&
					strings.Contains(output, "/etc/custom-config.yaml") &&
					strings.Contains(output, "key: value")
			},
		},
		{
			name: "with disk setup and mounts",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
					DiskSetup: &eksbootstrapv1.DiskSetup{
						Filesystems: []eksbootstrapv1.Filesystem{
							{
								Device:     "/dev/sdb",
								Filesystem: "ext4",
								Label:      "data_disk",
							},
						},
					},
					Mounts: []eksbootstrapv1.MountPoints{
						{"/dev/sdb", "/mnt/data"},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "Content-Type: text/cloud-config") &&
					strings.Contains(output, "/dev/sdb") &&
					strings.Contains(output, "ext4") &&
					strings.Contains(output, "/mnt/data")
			},
		},
		{
			name: "boundary verification - all parts with custom boundary",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:        "test-cluster",
					Region:             "us-west-2",
					ActivationID:       "test-activation-id",
					ActivationCode:     "test-activation-code",
					Boundary:           "HYBRIDBOUNDARY123",
					PreNodeadmCommands: []string{"echo 'test'"},
					NTP: &eksbootstrapv1.NTP{
						Enabled: ptr.To(true),
						Servers: []string{"time.google.com"},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				boundary := "HYBRIDBOUNDARY123"
				return strings.Contains(output, fmt.Sprintf(`boundary=%q`, boundary)) &&
					strings.Contains(output, fmt.Sprintf("--%s", boundary)) &&
					strings.Contains(output, fmt.Sprintf("--%s--", boundary)) &&
					strings.Contains(output, "Content-Type: application/node.eks.aws") &&
					strings.Contains(output, "Content-Type: text/x-shellscript") &&
					strings.Contains(output, "Content-Type: text/cloud-config")
			},
		},
		{
			name: "boundary verification - only node config part with default boundary",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				boundary := "//" // default boundary
				return strings.Contains(output, fmt.Sprintf(`boundary=%q`, boundary)) &&
					strings.Contains(output, fmt.Sprintf("--%s", boundary)) &&
					strings.Contains(output, fmt.Sprintf("--%s--", boundary)) &&
					strings.Contains(output, "Content-Type: application/node.eks.aws") &&
					!strings.Contains(output, "Content-Type: text/x-shellscript") &&
					!strings.Contains(output, "Content-Type: text/cloud-config")
			},
		},
		{
			name: "full configuration - all options",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "production-cluster",
					Region:         "eu-west-1",
					ActivationID:   "prod-activation-id",
					ActivationCode: "prod-activation-code",
					KubeletFlags: []string{
						"--node-labels=env=production",
					},
					KubeletConfig: &runtime.RawExtension{
						Raw: []byte(`maxPods: 250`),
					},
					ContainerdConfig: `[plugins]
  config = "value"`,
					PreNodeadmCommands: []string{"echo 'setup'"},
					NTP: &eksbootstrapv1.NTP{
						Enabled: ptr.To(true),
						Servers: []string{"ntp.example.com"},
					},
				},
			},
			expectErr: false,
			verifyOutput: func(output string) bool {
				return strings.Contains(output, "production-cluster") &&
					strings.Contains(output, "eu-west-1") &&
					strings.Contains(output, "prod-activation-id") &&
					strings.Contains(output, "prod-activation-code") &&
					strings.Contains(output, "env=production") &&
					strings.Contains(output, "maxPods: 250") &&
					strings.Contains(output, "containerd:") &&
					strings.Contains(output, "echo 'setup'") &&
					strings.Contains(output, "ntp.example.com")
			},
		},
		// Error cases
		{
			name: "missing cluster name",
			args: args{
				input: &HybridNodeadmInput{
					Region:         "us-west-2",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr:   true,
			errContains: "cluster name is required",
		},
		{
			name: "missing region",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					ActivationID:   "test-activation-id",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr:   true,
			errContains: "region is required",
		},
		{
			name: "missing activation ID",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:    "test-cluster",
					Region:         "us-west-2",
					ActivationCode: "test-activation-code",
				},
			},
			expectErr:   true,
			errContains: "SSM activation ID is required",
		},
		{
			name: "missing activation code",
			args: args{
				input: &HybridNodeadmInput{
					ClusterName:  "test-cluster",
					Region:       "us-west-2",
					ActivationID: "test-activation-id",
				},
			},
			expectErr:   true,
			errContains: "SSM activation code is required",
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			bytes, err := NewHybridNodeadmUserdata(testcase.args.input)
			if testcase.expectErr {
				g.Expect(err).To(HaveOccurred())
				if testcase.errContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(testcase.errContains))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			if testcase.verifyOutput != nil {
				g.Expect(testcase.verifyOutput(string(bytes))).To(BeTrue(), "Output verification failed for: %s\n\nActual output:\n%s", testcase.name, string(bytes))
			}
		})
	}
}

func TestHybridNodeadmUserdataFormat(t *testing.T) {
	g := NewWithT(t)

	input := &HybridNodeadmInput{
		ClusterName:    "test-cluster",
		Region:         "us-west-2",
		ActivationID:   "abc-123",
		ActivationCode: "secret-code-xyz",
	}

	output, err := NewHybridNodeadmUserdata(input)
	g.Expect(err).NotTo(HaveOccurred())

	outputStr := string(output)

	// Verify MIME structure
	g.Expect(outputStr).To(HavePrefix("MIME-Version: 1.0\n"))
	g.Expect(outputStr).To(ContainSubstring("Content-Type: multipart/mixed"))

	// Verify NodeConfig structure
	g.Expect(outputStr).To(ContainSubstring("apiVersion: node.eks.aws/v1alpha1"))
	g.Expect(outputStr).To(ContainSubstring("kind: NodeConfig"))
	g.Expect(outputStr).To(ContainSubstring("spec:"))
	g.Expect(outputStr).To(ContainSubstring("cluster:"))
	g.Expect(outputStr).To(ContainSubstring("hybrid:"))
	g.Expect(outputStr).To(ContainSubstring("ssm:"))

	// Verify the final closing boundary
	g.Expect(outputStr).To(HaveSuffix("--//--"))
}

func TestHybridVsStandardNodeadmDifferences(t *testing.T) {
	g := NewWithT(t)

	// Create a hybrid input
	hybridInput := &HybridNodeadmInput{
		ClusterName:    "test-cluster",
		Region:         "us-west-2",
		ActivationID:   "test-activation-id",
		ActivationCode: "test-activation-code",
	}

	hybridOutput, err := NewHybridNodeadmUserdata(hybridInput)
	g.Expect(err).NotTo(HaveOccurred())
	hybridStr := string(hybridOutput)

	// Create a standard EC2 nodeadm input
	standardInput := &NodeadmInput{
		ClusterName:       "test-cluster",
		APIServerEndpoint: "https://api.example.com",
		CACert:            "test-ca-cert",
	}

	standardOutput, err := NewNodeadmUserdata(standardInput)
	g.Expect(err).NotTo(HaveOccurred())
	standardStr := string(standardOutput)

	// Hybrid should have region, standard should have apiServerEndpoint
	g.Expect(hybridStr).To(ContainSubstring("region: us-west-2"))
	g.Expect(hybridStr).NotTo(ContainSubstring("apiServerEndpoint"))
	g.Expect(hybridStr).NotTo(ContainSubstring("certificateAuthority"))

	g.Expect(standardStr).To(ContainSubstring("apiServerEndpoint"))
	g.Expect(standardStr).To(ContainSubstring("certificateAuthority"))
	g.Expect(standardStr).NotTo(ContainSubstring("hybrid:"))
	g.Expect(standardStr).NotTo(ContainSubstring("ssm:"))
	g.Expect(standardStr).NotTo(ContainSubstring("activationId"))

	// Hybrid should have SSM activation credentials
	g.Expect(hybridStr).To(ContainSubstring("hybrid:"))
	g.Expect(hybridStr).To(ContainSubstring("ssm:"))
	g.Expect(hybridStr).To(ContainSubstring("activationId:"))
	g.Expect(hybridStr).To(ContainSubstring("activationCode:"))

	// Both should have the same basic structure
	g.Expect(hybridStr).To(ContainSubstring("apiVersion: node.eks.aws/v1alpha1"))
	g.Expect(standardStr).To(ContainSubstring("apiVersion: node.eks.aws/v1alpha1"))
}
