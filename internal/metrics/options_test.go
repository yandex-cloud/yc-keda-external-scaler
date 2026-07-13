package metrics

import "testing"

func TestParseNaNStrategySkip(t *testing.T) {
	if got := ParseNaNStrategy("skip"); got != NaNStrategySkip {
		t.Fatalf("ParseNaNStrategy(skip) = %q, want %q", got, NaNStrategySkip)
	}
}

func TestParseOptionalAggregationMethod(t *testing.T) {
	if got := ParseOptionalAggregationMethod(""); got != "" {
		t.Fatalf("empty optional aggregation = %q, want disabled", got)
	}
	if got := ParseOptionalAggregationMethod("avg"); got != AggregationAvg {
		t.Fatalf("avg optional aggregation = %q, want %q", got, AggregationAvg)
	}
}
