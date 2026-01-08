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
	"bytes"
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"

	eksbootstrapv1 "sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
)

const (
	// hybridNodeConfigPartTemplate generates the NodeConfig for hybrid nodes.
	// Key differences from EC2 nodeadm:
	// - Uses 'region' instead of 'apiServerEndpoint' and 'certificateAuthority'
	// - Includes 'hybrid.ssm' block with activation credentials
	// - Does not include 'cidr' (cluster service CIDR) - hybrid nodeadm discovers this
	hybridNodeConfigPartTemplate = `
--{{.Boundary}}
Content-Type: application/node.eks.aws

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: {{.ClusterName}}
    region: {{.Region}}
{{- if .KubeletConfig }}
  kubelet:
    config:
{{ Indent 6 (toYaml .KubeletConfig) }}
{{- end }}
{{- if .KubeletFlags }}
    flags:
{{- range $flag := .KubeletFlags }}
      - "{{$flag}}"
{{- end }}
{{- end }}
{{- if .ContainerdConfig }}
  containerd:
    config: |
{{ Indent 6 .ContainerdConfig }}
{{- end }}
  hybrid:
    ssm:
      activationId: {{.ActivationID}}
      activationCode: {{.ActivationCode}}

--{{.Boundary}}`
)

// HybridNodeadmInput contains all the information required to generate
// userdata for an EKS hybrid node.
type HybridNodeadmInput struct {
	// Cluster information
	ClusterName string
	Region      string

	// SSM activation credentials
	ActivationID   string
	ActivationCode string

	// Kubelet configuration
	KubeletFlags  []string
	KubeletConfig *runtime.RawExtension

	// Containerd configuration
	ContainerdConfig string

	// Cloud-init configuration (reused from standard nodeadm)
	PreNodeadmCommands []string
	Files              []eksbootstrapv1.File
	DiskSetup          *eksbootstrapv1.DiskSetup
	Mounts             []eksbootstrapv1.MountPoints
	Users              []eksbootstrapv1.User
	NTP                *eksbootstrapv1.NTP

	// MIME boundary
	Boundary string
}

// validateHybridNodeadmInput validates the input for hybrid nodeadm userdata generation.
func validateHybridNodeadmInput(input *HybridNodeadmInput) error {
	if input.ClusterName == "" {
		return fmt.Errorf("cluster name is required for hybrid nodeadm")
	}
	if input.Region == "" {
		return fmt.Errorf("region is required for hybrid nodeadm")
	}
	if input.ActivationID == "" {
		return fmt.Errorf("SSM activation ID is required for hybrid nodeadm")
	}
	if input.ActivationCode == "" {
		return fmt.Errorf("SSM activation code is required for hybrid nodeadm")
	}
	if input.Boundary == "" {
		input.Boundary = boundary
	}
	return nil
}

// NewHybridNodeadmUserdata generates userdata for an EKS hybrid node.
// The output is a MIME multipart document compatible with cloud-init.
// TODO: There is opportunity to reduce code duplication here between nodeadm.go but for now
// a duplication will be easier to implement.
func NewHybridNodeadmUserdata(input *HybridNodeadmInput) ([]byte, error) {
	if err := validateHybridNodeadmInput(input); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	// Write MIME header
	if _, err := buf.WriteString(fmt.Sprintf("MIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=%q\n\n", input.Boundary)); err != nil {
		return nil, fmt.Errorf("failed to write MIME header: %w", err)
	}

	// Write shell script part if pre-nodeadm commands exist
	if len(input.PreNodeadmCommands) > 0 {
		shellScriptTemplate := template.Must(template.New("shell").Parse(shellScriptPartTemplate))
		shellInput := struct {
			Boundary           string
			PreNodeadmCommands []string
		}{
			Boundary:           input.Boundary,
			PreNodeadmCommands: input.PreNodeadmCommands,
		}
		if err := shellScriptTemplate.Execute(&buf, shellInput); err != nil {
			return nil, fmt.Errorf("failed to execute shell script template: %w", err)
		}
		if _, err := buf.WriteString("\n"); err != nil {
			return nil, fmt.Errorf("failed to write newline: %w", err)
		}
	}

	// Write hybrid node config part
	hybridConfigTemplate := template.Must(
		template.New("hybridNode").
			Funcs(defaultTemplateFuncMap).
			Parse(hybridNodeConfigPartTemplate),
	)
	if err := hybridConfigTemplate.Execute(&buf, input); err != nil {
		return nil, fmt.Errorf("failed to execute hybrid node config template: %w", err)
	}

	// Write cloud-config part if needed (files, users, NTP, disk, mounts)
	if input.NTP != nil || input.DiskSetup != nil || input.Mounts != nil || input.Users != nil || input.Files != nil {
		tm := template.New("Node").Funcs(defaultTemplateFuncMap)

		if _, err := tm.Parse(filesTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse files template: %w", err)
		}
		if _, err := tm.Parse(ntpTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse ntp template: %w", err)
		}
		if _, err := tm.Parse(usersTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse users template: %w", err)
		}
		if _, err := tm.Parse(diskSetupTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse disk setup template: %w", err)
		}
		if _, err := tm.Parse(fsSetupTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse fs setup template: %w", err)
		}
		if _, err := tm.Parse(mountsTemplate); err != nil {
			return nil, fmt.Errorf("failed to parse mounts template: %w", err)
		}

		t, err := tm.Parse(cloudInitUserData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cloud-init template: %w", err)
		}

		cloudInitInput := struct {
			Boundary  string
			Files     []eksbootstrapv1.File
			NTP       *eksbootstrapv1.NTP
			Users     []eksbootstrapv1.User
			DiskSetup *eksbootstrapv1.DiskSetup
			Mounts    []eksbootstrapv1.MountPoints
		}{
			Boundary:  input.Boundary,
			Files:     input.Files,
			NTP:       input.NTP,
			Users:     input.Users,
			DiskSetup: input.DiskSetup,
			Mounts:    input.Mounts,
		}

		if err := t.Execute(&buf, cloudInitInput); err != nil {
			return nil, fmt.Errorf("failed to execute cloud-init template: %w", err)
		}
	}

	// Write final boundary closing
	buf.WriteString("--")

	return buf.Bytes(), nil
}
