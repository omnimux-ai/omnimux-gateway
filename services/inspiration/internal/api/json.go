package api

import "encoding/json"

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
