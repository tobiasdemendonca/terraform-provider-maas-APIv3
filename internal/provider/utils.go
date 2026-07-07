package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// optionalString converts a Terraform string attribute to an optional request
// field. Null and unknown values become nil; any known value, including "",
// becomes a pointer to it. Whether nil is then sent as an explicit JSON null
// or omitted from the request depends on the generated request struct's JSON
// tags — see the Marshal section of AGENTS.md.
func optionalString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}
