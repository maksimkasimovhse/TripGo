package create_test

import (
	"strings"
	"testing"

	"github.com/maksimkasimovhse/TripGo/internal/http/trip/create"
)

const (
	wantJSONObject   = "Request body must be a JSON object"
	wantRequiredUser = "user_id is required"
)

func TestCheckRequiredFields(t *testing.T) {
	t.Parallel()

	const valid = `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":55,"longitude":37},"end_point":{"latitude":56,"longitude":38},"price":100}`
	const user = `"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c"`
	const driver = `"driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5"`
	const start = `"start_point":{"latitude":55,"longitude":37}`
	const end = `"end_point":{"latitude":56,"longitude":38}`

	with := func(old, replacement string) string {
		return strings.Replace(valid, old, replacement, 1)
	}

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "valid body", body: valid},
		{name: "zero price", body: with(`"price":100`, `"price":0`)},
		{
			name: "zero coordinates",
			body: with(start, `"start_point":{"latitude":0,"longitude":0}`),
		},
		{name: "empty body", body: "", want: wantJSONObject},
		{name: "malformed JSON", body: `{`, want: wantJSONObject},
		{name: "array body", body: `[]`, want: wantJSONObject},
		{name: "number body", body: `42`, want: wantJSONObject},
		{name: "string body", body: `"text"`, want: wantJSONObject},
		{name: "empty object", body: `{}`, want: wantRequiredUser},
		{name: "missing user", body: with(user+",", ""), want: wantRequiredUser},
		{name: "null user", body: with(user, `"user_id":null`), want: wantRequiredUser},
		{name: "missing driver", body: with(driver+",", ""), want: "driver_id is required"},
		{name: "null driver", body: with(driver, `"driver_id":null`), want: "driver_id is required"},
		{name: "missing start", body: with(start+",", ""), want: "start_point is required"},
		{name: "null start", body: with(start, `"start_point":null`), want: "start_point is required"},
		{name: "missing end", body: with(end+",", ""), want: "end_point is required"},
		{name: "null end", body: with(end, `"end_point":null`), want: "end_point is required"},
		{name: "missing price", body: with(`,"price":100`, ""), want: "price is required"},
		{name: "null price", body: with(`"price":100`, `"price":null`), want: "price is required"},
		{name: "start is number", body: with(start, `"start_point":42`), want: "start_point must be an object"},
		{name: "start is string", body: with(start, `"start_point":"text"`), want: "start_point must be an object"},
		{name: "end is number", body: with(end, `"end_point":42`), want: "end_point must be an object"},
		{name: "end is string", body: with(end, `"end_point":"text"`), want: "end_point must be an object"},
		{
			name: "missing start latitude",
			body: with(start, `"start_point":{"longitude":37}`),
			want: "start_point.latitude is required",
		},
		{
			name: "missing start longitude",
			body: with(start, `"start_point":{"latitude":55}`),
			want: "start_point.longitude is required",
		},
		{
			name: "missing end latitude",
			body: with(end, `"end_point":{"longitude":38}`),
			want: "end_point.latitude is required",
		},
		{
			name: "missing end longitude",
			body: with(end, `"end_point":{"latitude":56}`),
			want: "end_point.longitude is required",
		},
		{
			name: "null start latitude",
			body: with(start, `"start_point":{"latitude":null,"longitude":37}`),
			want: "start_point.latitude is required",
		},
		{
			name: "null start longitude",
			body: with(start, `"start_point":{"latitude":55,"longitude":null}`),
			want: "start_point.longitude is required",
		},
		{
			name: "null end latitude",
			body: with(end, `"end_point":{"latitude":null,"longitude":38}`),
			want: "end_point.latitude is required",
		},
		{
			name: "null end longitude",
			body: with(end, `"end_point":{"latitude":56,"longitude":null}`),
			want: "end_point.longitude is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := create.CheckRequiredFields([]byte(tt.body))
			if got != tt.want {
				t.Errorf("CheckRequiredFields() = %q, want %q", got, tt.want)
			}
		})
	}
}
