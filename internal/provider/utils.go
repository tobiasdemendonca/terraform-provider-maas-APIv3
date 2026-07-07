package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiError formats an unexpected API response for a diagnostic. All MAAS
// error bodies share one shape (message plus optional per-field details), so
// surface it to give the user the actionable server message, not just a
// status code. Falls back to the status alone if the body is not that shape.
func apiError(status string, body []byte) string {
	msg := fmt.Sprintf("API returned %s", status)

	var parsed struct {
		Message string `json:"message"`
		Details []struct {
			Field   *string `json:"field"`
			Message string  `json:"message"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return msg
	}

	parts := []string{}
	if parsed.Message != "" {
		parts = append(parts, parsed.Message)
	}
	for _, d := range parsed.Details {
		if d.Field != nil && *d.Field != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", *d.Field, d.Message))
		} else if d.Message != "" {
			parts = append(parts, d.Message)
		}
	}
	if len(parts) == 0 {
		return msg
	}
	return fmt.Sprintf("%s: %s", msg, strings.Join(parts, " "))
}

// optionalString converts a Terraform string attribute to an optional request
// field. Null and unknown values become nil; any known value, including "",
// becomes a pointer to it. Whether nil is then sent as an explicit JSON null
// or omitted from the request depends on the generated request struct's JSON
// tags; see the Marshal section of AGENTS.md.
func optionalString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}
