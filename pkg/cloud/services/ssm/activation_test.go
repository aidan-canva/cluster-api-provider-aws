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

package ssm

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-aws/v2/pkg/cloud/services/ssm/mock_ssmiface"
)

func TestCreateHybridActivation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name        string
		params      *HybridActivationParams
		expect      func(m *mock_ssmiface.MockSSMAPIMockRecorder)
		wantErr     bool
		errContains string
		validate    func(t *testing.T, result *HybridActivationResult)
	}{
		{
			name:        "nil params returns error",
			params:      nil,
			wantErr:     true,
			errContains: "params cannot be nil",
		},
		{
			name: "missing IAMRoleARN returns error",
			params: &HybridActivationParams{
				RegistrationLimit: 1,
				ExpirationDays:    7,
			},
			wantErr:     true,
			errContains: "IAMRoleARN is required",
		},
		{
			name: "zero RegistrationLimit returns error",
			params: &HybridActivationParams{
				IAMRoleARN:        "arn:aws:iam::123456789012:role/TestRole",
				RegistrationLimit: 0,
				ExpirationDays:    7,
			},
			wantErr:     true,
			errContains: "RegistrationLimit must be greater than 0",
		},
		{
			name: "zero ExpirationDays returns error",
			params: &HybridActivationParams{
				IAMRoleARN:        "arn:aws:iam::123456789012:role/TestRole",
				RegistrationLimit: 1,
				ExpirationDays:    0,
			},
			wantErr:     true,
			errContains: "ExpirationDays must be greater than 0",
		},
		{
			name: "successful creation with minimal params",
			params: &HybridActivationParams{
				IAMRoleARN:        "arn:aws:iam::123456789012:role/TestRole",
				RegistrationLimit: 1,
				ExpirationDays:    7,
			},
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.CreateActivation(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, input *ssm.CreateActivationInput, optFns ...func(*ssm.Options)) (*ssm.CreateActivationOutput, error) {
						// Validate input
						if aws.ToString(input.IamRole) != "arn:aws:iam::123456789012:role/TestRole" {
							t.Errorf("expected IAM role ARN to be set")
						}
						if aws.ToInt32(input.RegistrationLimit) != 1 {
							t.Errorf("expected RegistrationLimit to be 1")
						}
						// Verify managed tag is present
						hasManagedTag := false
						for _, tag := range input.Tags {
							if aws.ToString(tag.Key) == TagKeyManaged && aws.ToString(tag.Value) == "true" {
								hasManagedTag = true
								break
							}
						}
						if !hasManagedTag {
							t.Errorf("expected managed tag to be present")
						}
						return &ssm.CreateActivationOutput{
							ActivationId:   aws.String("test-activation-id"),
							ActivationCode: aws.String("test-activation-code"),
						}, nil
					},
				)
			},
			wantErr: false,
			validate: func(t *testing.T, result *HybridActivationResult) {
				if result.ActivationID != "test-activation-id" {
					t.Errorf("expected ActivationID to be 'test-activation-id', got %q", result.ActivationID)
				}
				if result.ActivationCode != "test-activation-code" {
					t.Errorf("expected ActivationCode to be 'test-activation-code', got %q", result.ActivationCode)
				}
				// Verify expiration time is approximately 7 days from now
				expectedExpiration := time.Now().AddDate(0, 0, 7)
				if result.ExpirationTime.Before(expectedExpiration.Add(-time.Minute)) ||
					result.ExpirationTime.After(expectedExpiration.Add(time.Minute)) {
					t.Errorf("expected ExpirationTime to be approximately 7 days from now")
				}
			},
		},
		{
			name: "successful creation with all params",
			params: &HybridActivationParams{
				IAMRoleARN:          "arn:aws:iam::123456789012:role/TestRole",
				RegistrationLimit:   10,
				ExpirationDays:      14,
				DefaultInstanceName: "hybrid-node",
				Description:         "Test activation for hybrid nodes",
				ClusterName:         "test-cluster",
				Namespace:           "default",
				ConfigName:          "test-config",
				Tags: infrav1.Tags{
					"Environment": "test",
					"Team":        "platform",
				},
			},
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.CreateActivation(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, input *ssm.CreateActivationInput, optFns ...func(*ssm.Options)) (*ssm.CreateActivationOutput, error) {
						// Validate optional fields are set
						if aws.ToString(input.DefaultInstanceName) != "hybrid-node" {
							t.Errorf("expected DefaultInstanceName to be set")
						}
						if aws.ToString(input.Description) != "Test activation for hybrid nodes" {
							t.Errorf("expected Description to be set")
						}
						// Verify all tags are present
						tagMap := make(map[string]string)
						for _, tag := range input.Tags {
							tagMap[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
						}
						if tagMap[infrav1.ClusterTagKey("test-cluster")] != string(infrav1.ResourceLifecycleOwned) {
							t.Errorf("expected cluster tag to be set")
						}
						if tagMap[TagKeyNodeadmConfig] != "default/test-config" {
							t.Errorf("expected nodeadmconfig tag to be set")
						}
						if tagMap["Environment"] != "test" {
							t.Errorf("expected Environment tag to be set")
						}
						if tagMap["Team"] != "platform" {
							t.Errorf("expected Team tag to be set")
						}
						return &ssm.CreateActivationOutput{
							ActivationId:   aws.String("full-test-activation-id"),
							ActivationCode: aws.String("full-test-activation-code"),
						}, nil
					},
				)
			},
			wantErr: false,
			validate: func(t *testing.T, result *HybridActivationResult) {
				if result.ActivationID != "full-test-activation-id" {
					t.Errorf("expected ActivationID to be 'full-test-activation-id', got %q", result.ActivationID)
				}
			},
		},
		{
			name: "AWS API error is propagated",
			params: &HybridActivationParams{
				IAMRoleARN:        "arn:aws:iam::123456789012:role/TestRole",
				RegistrationLimit: 1,
				ExpirationDays:    7,
			},
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.CreateActivation(gomock.Any(), gomock.Any()).Return(nil, &smithy.GenericAPIError{
					Code:    "ValidationException",
					Message: "Invalid IAM role ARN",
				})
			},
			wantErr:     true,
			errContains: "failed to create SSM activation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			_ = infrav1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			clusterScope, err := getClusterScope(client)
			g.Expect(err).NotTo(HaveOccurred())

			ssmClientMock := mock_ssmiface.NewMockSSMAPI(mockCtrl)
			if tt.expect != nil {
				tt.expect(ssmClientMock.EXPECT())
			}

			s := NewService(clusterScope)
			s.SSMClient = ssmClientMock

			result, err := s.CreateHybridActivation(context.Background(), tt.params)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errContains))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).NotTo(BeNil())

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestDeleteHybridActivation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name         string
		activationID string
		expect       func(m *mock_ssmiface.MockSSMAPIMockRecorder)
		wantErr      bool
		errContains  string
	}{
		{
			name:         "empty activationID returns error",
			activationID: "",
			wantErr:      true,
			errContains:  "activationID is required",
		},
		{
			name:         "successful deletion",
			activationID: "test-activation-id",
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.DeleteActivation(gomock.Any(), gomock.Eq(&ssm.DeleteActivationInput{
					ActivationId: aws.String("test-activation-id"),
				})).Return(&ssm.DeleteActivationOutput{}, nil)
			},
			wantErr: false,
		},
		{
			name:         "activation not found is not an error (idempotent)",
			activationID: "nonexistent-activation",
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.DeleteActivation(gomock.Any(), gomock.Any()).Return(nil, &mockAPIError{
					Code:    "InvalidActivation",
					Message: "Activation not found",
				})
			},
			wantErr: false,
		},
		{
			name:         "other AWS errors are propagated",
			activationID: "test-activation-id",
			expect: func(m *mock_ssmiface.MockSSMAPIMockRecorder) {
				m.DeleteActivation(gomock.Any(), gomock.Any()).Return(nil, &smithy.GenericAPIError{
					Code:    "AccessDenied",
					Message: "Access denied",
				})
			},
			wantErr:     true,
			errContains: "failed to delete SSM activation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			_ = infrav1.AddToScheme(scheme)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			clusterScope, err := getClusterScope(client)
			g.Expect(err).NotTo(HaveOccurred())

			ssmClientMock := mock_ssmiface.NewMockSSMAPI(mockCtrl)
			if tt.expect != nil {
				tt.expect(ssmClientMock.EXPECT())
			}

			s := NewService(clusterScope)
			s.SSMClient = ssmClientMock

			err = s.DeleteHybridActivation(context.Background(), tt.activationID)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errContains))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestHybridActivationTagGeneration(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	clusterScope, err := getClusterScope(client)
	g.Expect(err).NotTo(HaveOccurred())

	ssmClientMock := mock_ssmiface.NewMockSSMAPI(mockCtrl)

	// Capture the tags that were passed to CreateActivation
	var capturedTags []ssmtypes.Tag
	ssmClientMock.EXPECT().CreateActivation(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, input *ssm.CreateActivationInput, optFns ...func(*ssm.Options)) (*ssm.CreateActivationOutput, error) {
			capturedTags = input.Tags
			return &ssm.CreateActivationOutput{
				ActivationId:   aws.String("test-id"),
				ActivationCode: aws.String("test-code"),
			}, nil
		},
	)

	s := NewService(clusterScope)
	s.SSMClient = ssmClientMock

	params := &HybridActivationParams{
		IAMRoleARN:        "arn:aws:iam::123456789012:role/TestRole",
		RegistrationLimit: 1,
		ExpirationDays:    7,
		ClusterName:       "my-cluster",
		Namespace:         "kube-system",
		ConfigName:        "my-nodeadm-config",
		Tags: infrav1.Tags{
			"custom-tag": "custom-value",
		},
	}

	_, err = s.CreateHybridActivation(context.Background(), params)
	g.Expect(err).NotTo(HaveOccurred())

	// Convert captured tags to a map for easier verification
	tagMap := make(map[string]string)
	for _, tag := range capturedTags {
		tagMap[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	// Verify standard tags - use ClusterTagKey for the cluster tag
	g.Expect(tagMap).To(HaveKeyWithValue(infrav1.ClusterTagKey("my-cluster"), string(infrav1.ResourceLifecycleOwned)))
	g.Expect(tagMap).To(HaveKeyWithValue(TagKeyNodeadmConfig, "kube-system/my-nodeadm-config"))
	g.Expect(tagMap).To(HaveKeyWithValue(TagKeyManaged, "true"))

	// Verify custom tag
	g.Expect(tagMap).To(HaveKeyWithValue("custom-tag", "custom-value"))
}
