package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

func TestFlattenSpace(t *testing.T) {
	tests := []struct {
		name            string
		space           maasclientv3.SpaceResponse
		wantName        types.String
		wantDescription types.String
	}{
		{
			name: "all fields set",
			space: maasclientv3.SpaceResponse{
				Id:          7,
				Name:        strPtr("space-7"),
				Description: strPtr("my description"),
			},
			wantName:        types.StringValue("space-7"),
			wantDescription: types.StringValue("my description"),
		},
		{
			// description is not nullable in MAAS, nil is coerced to ""
			// name is defensively treated as nullable in the response type
			name:            "all optional fields nil",
			space:           maasclientv3.SpaceResponse{Id: 7},
			wantName:        types.StringNull(),
			wantDescription: types.StringValue(""),
		},
		{
			name: "empty strings stay empty strings",
			space: maasclientv3.SpaceResponse{
				Id:          7,
				Name:        strPtr("space-7"),
				Description: strPtr(""),
			},
			wantName:        types.StringValue("space-7"),
			wantDescription: types.StringValue(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data spaceResourceModel
			flattenSpace(&tt.space, &data)

			if got := data.Id; got != types.Int64Value(int64(tt.space.Id)) {
				t.Errorf("id: got %v, want %v", got, tt.space.Id)
			}
			if got := data.Name; !got.Equal(tt.wantName) {
				t.Errorf("name: got %v, want %v", got, tt.wantName)
			}
			if got := data.Description; !got.Equal(tt.wantDescription) {
				t.Errorf("description: got %v, want %v", got, tt.wantDescription)
			}
		})
	}
}
