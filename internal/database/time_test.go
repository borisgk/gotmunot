package database

import (
	"testing"
	"time"
)

func TestParseFlexibleTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "Standard SQLite format",
			input:    "2023-05-21 14:30:05",
			expected: time.Date(2023, 5, 21, 14, 30, 5, 0, time.UTC),
		},
		{
			name:     "RFC3339 format",
			input:    "2023-05-21T14:30:05Z",
			expected: time.Date(2023, 5, 21, 14, 30, 5, 0, time.UTC),
		},
		{
			name:     "Go String() format",
			input:    "2023-05-21 14:30:05 +0000 UTC",
			expected: time.Date(2023, 5, 21, 14, 30, 5, 0, time.UTC),
		},
		{
			name:     "SQLite style with offset",
			input:    "2023-05-21 14:30:05+00:00",
			expected: time.Date(2023, 5, 21, 14, 30, 5, 0, time.UTC),
		},
		{
			name:     "ISO without Z",
			input:    "2023-05-21T14:30:05",
			expected: time.Date(2023, 5, 21, 14, 30, 5, 0, time.UTC),
		},
		{
			name:     "Invalid format",
			input:    "invalid",
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlexibleTime(tt.input)
			if !got.Equal(tt.expected) {
				t.Errorf("parseFlexibleTime(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTimeStorageConsistency(t *testing.T) {
	// Verify that the constant matches our expected format
	now := time.Date(2023, 12, 23, 22, 45, 0, 0, time.UTC)
	formatted := now.Format(SqliteTimeLayout)
	expected := "2023-12-23 22:45:00"
	if formatted != expected {
		t.Errorf("sqliteTimeLayout format mismatch: got %s, want %s", formatted, expected)
	}
}
