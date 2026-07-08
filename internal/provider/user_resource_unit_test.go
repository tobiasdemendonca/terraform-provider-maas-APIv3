package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

func TestFlattenUser(t *testing.T) {
	tests := []struct {
		name     string
		input    maasclientv3.UserResponse
		expected userResourceModel
	}{
		{
			name: "all fields set",
			input: maasclientv3.UserResponse{
				Id:        42,
				Username:  "alice",
				FirstName: "Alice",
				LastName:  strPtr("Smith"),
				Email:     strPtr("alice@example.com"),
				Groups:    []maasclientv3.UserGroupSummaryResponse{{Id: 1, Name: "admins"}},
			},
			expected: userResourceModel{
				Id:        types.Int64Value(42),
				Username:  types.StringValue("alice"),
				FirstName: types.StringValue("Alice"),
				LastName:  types.StringValue("Smith"),
				Email:     types.StringValue("alice@example.com"),
				Groups:    types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
			},
		},
		{
			name: "nullable fields null",
			input: maasclientv3.UserResponse{
				Id:        7,
				Username:  "bob",
				FirstName: "Bob",
				LastName:  nil,
				Email:     nil,
				Groups:    []maasclientv3.UserGroupSummaryResponse{},
			},
			expected: userResourceModel{
				Id:        types.Int64Value(7),
				Username:  types.StringValue("bob"),
				FirstName: types.StringValue("Bob"),
				LastName:  types.StringValue(""),
				Email:     types.StringNull(),
				Groups:    types.ListValueMust(types.Int64Type, []attr.Value{}),
			},
		},
		{
			name: "multiple groups",
			input: maasclientv3.UserResponse{
				Id:        100,
				Username:  "carol",
				FirstName: "Carol",
				LastName:  strPtr("Jones"),
				Email:     strPtr("carol@example.com"),
				Groups: []maasclientv3.UserGroupSummaryResponse{
					{Id: 1, Name: "admins"},
					{Id: 3, Name: "users"},
				},
			},
			expected: userResourceModel{
				Id:        types.Int64Value(100),
				Username:  types.StringValue("carol"),
				FirstName: types.StringValue("Carol"),
				LastName:  types.StringValue("Jones"),
				Email:     types.StringValue("carol@example.com"),
				Groups:    types.ListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1), types.Int64Value(3)}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data userResourceModel
			flattenUser(&tt.input, &data)
			if !data.Id.Equal(tt.expected.Id) {
				t.Errorf("Id: got %v, want %v", data.Id, tt.expected.Id)
			}
			if !data.Username.Equal(tt.expected.Username) {
				t.Errorf("Username: got %v, want %v", data.Username, tt.expected.Username)
			}
			if !data.FirstName.Equal(tt.expected.FirstName) {
				t.Errorf("FirstName: got %v, want %v", data.FirstName, tt.expected.FirstName)
			}
			if !data.LastName.Equal(tt.expected.LastName) {
				t.Errorf("LastName: got %v, want %v", data.LastName, tt.expected.LastName)
			}
			if !data.Email.Equal(tt.expected.Email) {
				t.Errorf("Email: got %v, want %v", data.Email, tt.expected.Email)
			}
			if !data.Groups.Equal(tt.expected.Groups) {
				t.Errorf("Groups: got %v, want %v", data.Groups, tt.expected.Groups)
			}
		})
	}
}

func TestOptionalInt64List(t *testing.T) {
	tests := []struct {
		name      string
		input     types.List
		expectNil bool
	}{
		{
			name:      "null list",
			input:     types.ListNull(types.Int64Type),
			expectNil: true,
		},
		{
			name:      "unknown list",
			input:     types.ListUnknown(types.Int64Type),
			expectNil: true,
		},
		{
			name:      "empty list",
			input:     types.ListValueMust(types.Int64Type, []attr.Value{}),
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optionalInt64List(tt.input)
			if tt.expectNil && result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			if !tt.expectNil && result == nil {
				t.Errorf("expected non-nil, got nil")
			}
		})
	}
}
