package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOptionalString(t *testing.T) {
	tests := []struct {
		name  string
		input types.String
		want  *string
	}{
		{"null", types.StringNull(), nil},
		{"unknown", types.StringUnknown(), nil},
		{"empty string", types.StringValue(""), strPtr("")},
		{"value", types.StringValue("x"), strPtr("x")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optionalString(tt.input)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("got %q, want nil", *got)
			case tt.want != nil && got == nil:
				t.Errorf("got nil, want %q", *tt.want)
			case tt.want != nil && got != nil && *got != *tt.want:
				t.Errorf("got %q, want %q", *got, *tt.want)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	tests := []struct {
		name   string
		status string
		body   string
		want   string
	}{
		{
			// real MAAS body for POST /fabrics with name "bad!name"
			name:   "422 validation error with field detail",
			status: "422 Unprocessable Entity",
			body:   `{"kind":"Error","code":422,"message":"Failed to validate the request.","details":[{"type":"value_error","message":"Value error, Invalid entity name.","field":"name","location":"body"}]}`,
			want:   "API returned 422 Unprocessable Entity: Failed to validate the request. name: Value error, Invalid entity name.",
		},
		{
			// real MAAS body for DELETE /fabrics/0
			name:   "400 with detail and no field",
			status: "400 Bad Request",
			body:   `{"kind":"Error","code":400,"message":"Bad request.","details":[{"type":"CannotDeleteDefaultFabricViolation","message":"The default Fabric (id=0) cannot be deleted.","field":null,"location":null}]}`,
			want:   "API returned 400 Bad Request: Bad request. The default Fabric (id=0) cannot be deleted.",
		},
		{
			name:   "non-JSON body falls back to status",
			status: "502 Bad Gateway",
			body:   "<html>proxy error</html>",
			want:   "API returned 502 Bad Gateway",
		},
		{
			name:   "empty body falls back to status",
			status: "500 Internal Server Error",
			body:   "",
			want:   "API returned 500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apiError(tt.status, []byte(tt.body)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
