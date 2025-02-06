package env

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"testing"

	"github.com/Azure/msi-dataplane/pkg/dataplane"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
)

func managedIdentityCredentials(censor bool, delegatedResources []*dataplane.DelegatedResource, explicitIdentities []*dataplane.UserAssignedIdentityCredentials) dataplane.ManagedIdentityCredentials {
	return dataplane.ManagedIdentityCredentials{
		AuthenticationEndpoint: ptr.To("AuthenticationEndpoint"),
		CannotRenewAfter:       ptr.To("CannotRenewAfter"),
		ClientID:               ptr.To("ClientID"),
		ClientSecret: func() *string {
			if censor {
				return nil
			}
			return ptr.To("ClientSecret")
		}(),
		ClientSecretURL: ptr.To("ClientSecretURL"),
		CustomClaims:    ptr.To(customClaims()),
		DelegatedResources: func() []*dataplane.DelegatedResource {
			if len(delegatedResources) > 0 {
				return delegatedResources
			}
			return nil
		}(),
		DelegationURL: ptr.To("DelegationURL"),
		ExplicitIdentities: func() []*dataplane.UserAssignedIdentityCredentials {
			if len(explicitIdentities) > 0 {
				return explicitIdentities
			}
			return nil
		}(),
		InternalID:                 ptr.To("InternalID"),
		MtlsAuthenticationEndpoint: ptr.To("MtlsAuthenticationEndpoint"),
		NotAfter:                   ptr.To("NotAfter"),
		NotBefore:                  ptr.To("NotBefore"),
		ObjectID:                   ptr.To("ObjectID"),
		RenewAfter:                 ptr.To("RenewAfter"),
		TenantID:                   ptr.To("TenantID"),
	}
}

func delegatedResource(implicitIdentity *dataplane.UserAssignedIdentityCredentials, explicitIdentities ...*dataplane.UserAssignedIdentityCredentials) *dataplane.DelegatedResource {
	return &dataplane.DelegatedResource{
		DelegationID:  ptr.To("DelegationID"),
		DelegationURL: ptr.To("DelegationURL"),
		ExplicitIdentities: func() []*dataplane.UserAssignedIdentityCredentials {
			if len(explicitIdentities) > 0 {
				return explicitIdentities
			}
			return nil
		}(),
		ImplicitIdentity: implicitIdentity,
		InternalID:       ptr.To("InternalID"),
		ResourceID:       ptr.To("ResourceID"),
	}
}

func userAssignedIdentityCredentials(censor bool) *dataplane.UserAssignedIdentityCredentials {
	return &dataplane.UserAssignedIdentityCredentials{
		AuthenticationEndpoint: ptr.To("AuthenticationEndpoint"),
		CannotRenewAfter:       ptr.To("CannotRenewAfter"),
		ClientID:               ptr.To("ClientID"),
		ClientSecret: func() *string {
			if censor {
				return nil
			}
			return ptr.To("ClientSecret")
		}(),
		ClientSecretURL:            ptr.To("ClientSecretURL"),
		CustomClaims:               ptr.To(customClaims()),
		MtlsAuthenticationEndpoint: ptr.To("MtlsAuthenticationEndpoint"),
		NotAfter:                   ptr.To("NotAfter"),
		NotBefore:                  ptr.To("NotBefore"),
		ObjectID:                   ptr.To("ObjectID"),
		RenewAfter:                 ptr.To("RenewAfter"),
		ResourceID:                 ptr.To("ResourceID"),
		TenantID:                   ptr.To("TenantID"),
	}
}

func customClaims() dataplane.CustomClaims {
	return dataplane.CustomClaims{
		XMSAzNwperimid: []*string{ptr.To("XMSAzNwperimid")},
		XMSAzTm:        ptr.To("XMSAzTm"),
	}
}

func TestCensorCredentials(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		generateData func(censor bool) (data *dataplane.ManagedIdentityCredentials)
	}{
		{
			name: "no delegated resources, explicit credentials",
			generateData: func(censor bool) (data *dataplane.ManagedIdentityCredentials) {
				return ptr.To(managedIdentityCredentials(censor, nil, nil))
			},
		},
		{
			name: "delegated resource without explicit credentials, no top-level explicit credentials",
			generateData: func(censor bool) (data *dataplane.ManagedIdentityCredentials) {
				return ptr.To(managedIdentityCredentials(censor, []*dataplane.DelegatedResource{
					delegatedResource(userAssignedIdentityCredentials(censor)),
					delegatedResource(userAssignedIdentityCredentials(censor)),
					nil,
				}, nil))
			},
		},
		{
			name: "delegated resource with explicit credentials, no top-level explicit credentials",
			generateData: func(censor bool) (data *dataplane.ManagedIdentityCredentials) {
				return ptr.To(managedIdentityCredentials(censor, []*dataplane.DelegatedResource{
					delegatedResource(userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor)),
					delegatedResource(userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), nil),
				}, nil))
			},
		},
		{
			name: "delegated resource with explicit credentials, top-level explicit credentials",
			generateData: func(censor bool) (data *dataplane.ManagedIdentityCredentials) {
				return ptr.To(managedIdentityCredentials(censor, []*dataplane.DelegatedResource{
					delegatedResource(userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor)),
					delegatedResource(userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), userAssignedIdentityCredentials(censor), nil),
				}, []*dataplane.UserAssignedIdentityCredentials{
					userAssignedIdentityCredentials(censor),
					userAssignedIdentityCredentials(censor),
					nil,
				}))
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			input, output := testCase.generateData(false), testCase.generateData(true)
			censorCredentials(input)
			if diff := cmp.Diff(output, input); diff != "" {
				t.Errorf("censorCredentials mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
