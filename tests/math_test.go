package math

import "testing"

func TestAddition(t *testing.T) {
	testCases := []struct {
		name         string
		a, b, expect int
	}{
		{"positive", 1, 2, 3},
		{"negative", -1, -2, -3},
		{"mixed", -1, 2, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a + tc.b
			if result != tc.expect {
				t.Errorf("Addition failed: expected %d, got %d", tc.expect, result)
			}
		})
	}
}

func TestSubtraction(t *testing.T) {
	testCases := []struct {
		name         string
		a, b, expect int
	}{
		{"simple", 5, 3, 2},
		{"negative result", 3, 5, -2},
		{"with negatives", -5, -3, -2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a - tc.b
			if result != tc.expect {
				t.Errorf("Subtraction failed: expected %d, got %d", tc.expect, result)
			}
		})
	}
}

func TestMultiplication(t *testing.T) {
	testCases := []struct {
		name         string
		a, b, expect int
	}{
		{"positive", 2, 3, 6},
		{"negative", -2, 3, -6},
		{"zero", 0, 10, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.a * tc.b
			if result != tc.expect {
				t.Errorf("Multiplication failed: expected %d, got %d", tc.expect, result)
			}
		})
	}
}

func TestDivision(t *testing.T) {
	testCases := []struct {
		name        string
		a, b        int
		expect      int
		expectError bool
	}{
		{"exact division", 10, 2, 5, false},
		{"integer division truncation", 7, 2, 3, false},
		{"division by zero", 5, 0, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.b == 0 {
				if !tc.expectError {
					t.Errorf("Unexpected error: division by zero")
				}
			} else {
				result := tc.a / tc.b
				if result != tc.expect {
					t.Errorf("Division failed: expected %d, got %d", tc.expect, result)
				}
			}
		})
	}
}
