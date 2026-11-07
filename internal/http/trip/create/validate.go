package create

import (
	"encoding/json"
)

func CheckRequiredFields(body []byte) string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return "Request body must be a JSON object"
	}
	for _, f := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		if v, ok := top[f]; !ok || string(v) == "null" {
			return f + " is required"
		}
	}
	for _, p := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(top[p], &point); err != nil {
			return p + " must be an object"
		}
		for _, f := range []string{"latitude", "longitude"} {
			if v, ok := point[f]; !ok || string(v) == "null" {
				return p + "." + f + " is required"
			}
		}
	}
	return ""
}
