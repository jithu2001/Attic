package scanner

import "testing"

func TestNumberFromRaw(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]interface{}
		keys []string
		want int
	}{
		{
			name: "bare number",
			raw:  map[string]interface{}{"tracknumber": "3"},
			keys: []string{"tracknumber", "track"},
			want: 3,
		},
		{
			// The form most rippers write, and the one dhowden's Atoi drops.
			name: "number of total",
			raw:  map[string]interface{}{"tracknumber": "3/12"},
			keys: []string{"tracknumber", "track"},
			want: 3,
		},
		{
			// What ffmpeg writes into a FLAC.
			name: "ffmpeg's non-canonical key",
			raw:  map[string]interface{}{"track": "7/9"},
			keys: []string{"tracknumber", "track"},
			want: 7,
		},
		{
			name: "keys are tried in order",
			raw:  map[string]interface{}{"track": "9", "tracknumber": "2"},
			keys: []string{"tracknumber", "track"},
			want: 2,
		},
		{
			name: "already an int",
			raw:  map[string]interface{}{"track": 4},
			keys: []string{"track"},
			want: 4,
		},
		{
			name: "padded and spaced",
			raw:  map[string]interface{}{"tracknumber": " 04 / 11 "},
			keys: []string{"tracknumber"},
			want: 4,
		},
		{
			name: "disc numbers work the same way",
			raw:  map[string]interface{}{"discnumber": "2/2"},
			keys: []string{"discnumber", "disc"},
			want: 2,
		},
		{
			name: "missing",
			raw:  map[string]interface{}{"title": "So What"},
			keys: []string{"tracknumber", "track"},
			want: 0,
		},
		{
			name: "not a number",
			raw:  map[string]interface{}{"tracknumber": "A-side"},
			keys: []string{"tracknumber"},
			want: 0,
		},
		{
			name: "zero is not a track number",
			raw:  map[string]interface{}{"tracknumber": "0/12"},
			keys: []string{"tracknumber"},
			want: 0,
		},
		{
			name: "unexpected type is ignored",
			raw:  map[string]interface{}{"tracknumber": []byte{1}},
			keys: []string{"tracknumber"},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := numberFromRaw(tc.raw, tc.keys...); got != tc.want {
				t.Errorf("numberFromRaw(%v, %v) = %d, want %d", tc.raw, tc.keys, got, tc.want)
			}
		})
	}
}
