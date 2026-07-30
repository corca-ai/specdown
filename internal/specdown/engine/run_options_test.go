package engine

import "testing"

func TestNormalizeRunOptions(t *testing.T) {
	tests := []struct {
		name            string
		options         RunOptions
		wantJobs        int
		wantMaxFailures int
	}{
		{
			name:            "zero values use defaults",
			options:         RunOptions{},
			wantJobs:        1,
			wantMaxFailures: 0,
		},
		{
			name: "negative values preserve unlimited serial behavior",
			options: RunOptions{
				Jobs:        -4,
				MaxFailures: -2,
			},
			wantJobs:        1,
			wantMaxFailures: 0,
		},
		{
			name: "positive values are preserved",
			options: RunOptions{
				Jobs:        4,
				MaxFailures: 2,
			},
			wantJobs:        4,
			wantMaxFailures: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeRunOptions(test.options)
			if got.Jobs != test.wantJobs {
				t.Errorf("Jobs = %d, want %d", got.Jobs, test.wantJobs)
			}
			if got.MaxFailures != test.wantMaxFailures {
				t.Errorf(
					"MaxFailures = %d, want %d",
					got.MaxFailures,
					test.wantMaxFailures,
				)
			}
		})
	}
}
