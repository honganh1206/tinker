package tools

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// generate translates Go structs into JSON schema during runtime
// to produce a standard format usable outside Go
func generate[T any]() *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}

	var v T

	rawSchema := reflector.Reflect(v)

	return rawSchema
}

// decode translates raw JSON message to structured, predefined tool schemas.
func decode[T any](raw json.RawMessage) (T, error) {
	var out T
	err := json.Unmarshal(raw, &out)
	return out, err
}
