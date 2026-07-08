package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

func TestFlattenResourcePool(t *testing.T) {
	tests := []struct {
		name            string
		pool            maasclientv3.ResourcePoolResponse
		wantName        types.String
		wantDescription types.String
	}{
		{
			name: "all fields set",
			pool: maasclientv3.ResourcePoolResponse{
				Id:          7,
				Name:        "pool-7",
				Description: "my description",
			},
			wantName:        types.StringValue("pool-7"),
			wantDescription: types.StringValue("my description"),
		},
		{
			// Zero values map to "", correct for a NOT NULL description.
			name:            "empty strings stay empty strings",
			pool:            maasclientv3.ResourcePoolResponse{Id: 7, Name: "", Description: ""},
			wantName:        types.StringValue(""),
			wantDescription: types.StringValue(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data resourcePoolResourceModel
			flattenResourcePool(&tt.pool, &data)

			if got := data.Id; got != types.Int64Value(int64(tt.pool.Id)) {
				t.Errorf("id: got %v, want %v", got, tt.pool.Id)
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
