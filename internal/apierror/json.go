package apierror

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

func decodeStrict(data []byte, dst any) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed[0] != '{' {
		return errors.New("request must be a JSON object")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}

		return err
	}

	return nil
}

// DecodeKnownObject decodes an object while rejecting any unrecognized field.
// It supports custom unmarshalling that must distinguish omission from zero.
func DecodeKnownObject(data []byte, dst any, allowed ...string) error {
	if !StrictJSONObject(data) {
		return errors.New("request must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowedFields := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedFields[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := allowedFields[field]; !ok {
			return errors.New("unknown field")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	return decoder.Decode(dst)
}

// StrictJSONObject reports whether a body starts with a JSON object.
// Full JSON validity is checked by the decoder.
func StrictJSONObject(data []byte) bool {
	trimmed := bytes.TrimSpace(data)

	return len(trimmed) > 0 && trimmed[0] == '{'
}
