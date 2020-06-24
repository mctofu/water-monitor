package water

import (
	"context"
	"strings"
	"testing"
)

func TestAnalyze(t *testing.T) {
	testInput := `Date	Consumption in GALLONS
4/21	1944
4/22	1944
4/23	2169
4/24	2169
4/25	1496
4/26	1421
4/27	1496
4/28	1720
4/29	2468
4/30	2244
5/01	1645
5/02	1570
5/03	1645
5/04	1795
5/05	1570`

	var tests = []struct {
		name        string
		expectAlert bool
		twoDayAvg,
		oneDayMax int
		expectMsg string
	}{
		{"no alert", false, 3000, 3000, ""},
		{"two day alert", true, 1600, 3000, "Two day avg water usage of 1682 gallons is greater than 1600 gallon limit."},
		{"one day alert", true, 3000, 1500, "Last day water usage of 1570 gallons is greater than 1500 gallon limit."},
	}

	for _, test := range tests {
		alerter := &mockAlerter{}
		report, err := parseUsage(testInput)
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		if err := AnalyzeUsage(context.Background(), report, test.twoDayAvg, test.oneDayMax, alerter); err != nil {
			t.Fatalf("Failed to analyze: %v", err)
		}
		if test.expectAlert != alerter.alerted {
			t.Errorf("Failed to alert for test: %s", test.name)
		}
		if !strings.HasPrefix(alerter.msg, test.expectMsg) {
			t.Errorf("Expected %s but was %s for test %s", test.expectMsg, alerter.msg, test.name)
		}
	}
}

type mockAlerter struct {
	alerted bool
	msg     string
}

func (m *mockAlerter) Alert(ctx context.Context, msg string) error {
	m.alerted = true
	m.msg = msg
	return nil
}
