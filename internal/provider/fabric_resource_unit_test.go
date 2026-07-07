package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-maas-apiv3/internal/client/maasclientv3"
)

func TestFlattenFabric(t *testing.T) {
	tests := []struct {
		name            string
		fabric          maasclientv3.FabricResponse
		wantName        types.String
		wantDescription types.String
		wantClassType   types.String
	}{
		{
			name: "all fields set",
			fabric: maasclientv3.FabricResponse{
				Id:          7,
				Name:        strPtr("fabric-7"),
				Description: strPtr("my description"),
				ClassType:   strPtr("10g"),
			},
			wantName:        types.StringValue("fabric-7"),
			wantDescription: types.StringValue("my description"),
			wantClassType:   types.StringValue("10g"),
		},
		{
			// description is not nullable in MAAS, nil is coerced to ""
			// class_type can be null in MAAS, nil stays null
			name:            "all optional fields nil",
			fabric:          maasclientv3.FabricResponse{Id: 7},
			wantName:        types.StringNull(),
			wantDescription: types.StringValue(""),
			wantClassType:   types.StringNull(),
		},
		{
			name: "empty strings stay empty strings",
			fabric: maasclientv3.FabricResponse{
				Id:          7,
				Name:        strPtr("fabric-7"),
				Description: strPtr(""),
				ClassType:   strPtr(""),
			},
			wantName:        types.StringValue("fabric-7"),
			wantDescription: types.StringValue(""),
			wantClassType:   types.StringValue(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data fabricResourceModel
			flattenFabric(&tt.fabric, &data)

			if got := data.Id; got != types.Int64Value(int64(tt.fabric.Id)) {
				t.Errorf("id: got %v, want %v", got, tt.fabric.Id)
			}
			if got := data.Name; !got.Equal(tt.wantName) {
				t.Errorf("name: got %v, want %v", got, tt.wantName)
			}
			if got := data.Description; !got.Equal(tt.wantDescription) {
				t.Errorf("description: got %v, want %v", got, tt.wantDescription)
			}
			if got := data.ClassType; !got.Equal(tt.wantClassType) {
				t.Errorf("class_type: got %v, want %v", got, tt.wantClassType)
			}
		})
	}
}
