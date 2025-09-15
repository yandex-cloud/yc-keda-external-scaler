package metrics

import (
	"strconv"
	"strings"
)

type NaNStrategy string

const (
	NaNStrategySkip      NaNStrategy = "skip"
	NaNStrategyZero      NaNStrategy = "zero"
	NaNStrategyError     NaNStrategy = "error"
	NaNStrategyLastValid NaNStrategy = "lastValid"
)

type AggregationMethod string

const (
	AggregationSum  AggregationMethod = "sum"
	AggregationAvg  AggregationMethod = "avg"
	AggregationMax  AggregationMethod = "max"
	AggregationMin  AggregationMethod = "min"
	AggregationLast AggregationMethod = "last"
)

type DownsamplingMode string

const (
	DownsamplingMaxPoints    DownsamplingMode = "maxPoints"
	DownsamplingGridInterval DownsamplingMode = "gridInterval"
	DownsamplingDisabled     DownsamplingMode = "disabled"
	DownsamplingNone         DownsamplingMode = "none"
)

type DownsamplingOptions struct {
	GridAggregation string           // MAX, MIN, SUM, AVG, LAST, COUNT
	GapFilling      string           // NULL, NONE, PREVIOUS
	Mode            DownsamplingMode // Which downsampling method to use
	MaxPoints       int              // For maxPoints mode (≥10)
	GridInterval    int64            // For gridInterval mode (milliseconds, >0)
	Disabled        bool             // For disabled mode
	HasSettings     bool             // Whether any downsampling settings were provided
}

type QueryOptions struct {
	Query                 string
	FolderID              string
	NaNStrategy           NaNStrategy
	AggregationMethod     AggregationMethod
	TimeSeriesAggregation AggregationMethod
	TimeWindow            string
	TimeWindowOffset      string
	Downsampling          DownsamplingOptions
}

func ParseNaNStrategy(s string) NaNStrategy {
	switch strings.ToLower(s) {
	case "zero":
		return NaNStrategyZero
	case "error":
		return NaNStrategyError
	case "lastvalid", "last_valid":
		return NaNStrategyLastValid
	default:
		return NaNStrategyError
	}
}

func ParseAggregationMethod(s string) AggregationMethod {
	switch strings.ToLower(s) {
	case "sum":
		return AggregationSum
	case "max", "maximum":
		return AggregationMax
	case "min", "minimum":
		return AggregationMin
	case "last":
		return AggregationLast
	case "avg", "average", "mean":
		return AggregationAvg
	default:
		return AggregationMax
	}
}

func ParseDownsamplingOptions(metadata map[string]string) DownsamplingOptions {
	opts := DownsamplingOptions{
		GridAggregation: parseGridAggregation(metadata["downsampling.gridAggregation"]),
		GapFilling:      parseGapFilling(metadata["downsampling.gapFilling"]),
		HasSettings:     false,
	}

	downsamplingKeys := []string{
		"downsampling.gridAggregation",
		"downsampling.gapFilling",
		"downsampling.maxPoints",
		"downsampling.gridInterval",
		"downsampling.disabled",
	}

	for _, key := range downsamplingKeys {
		if metadata[key] != "" {
			opts.HasSettings = true
			break
		}
	}

	if !opts.HasSettings {
		return DownsamplingOptions{
			Mode:        DownsamplingNone,
			HasSettings: false,
		}
	}

	modes := []struct {
		key   string
		mode  DownsamplingMode
		value interface{}
	}{
		{"downsampling.maxPoints", DownsamplingMaxPoints, parseMaxPoints(metadata["downsampling.maxPoints"])},
		{"downsampling.gridInterval", DownsamplingGridInterval, parseGridInterval(metadata["downsampling.gridInterval"])},
		{"downsampling.disabled", DownsamplingDisabled, parseBool(metadata["downsampling.disabled"])},
	}

	activeMode := ""
	for _, m := range modes {
		if metadata[m.key] != "" {
			if activeMode != "" {
				return getErrorDownsampling()
			}
			activeMode = m.key
			opts.Mode = m.mode
			switch m.mode {
			case DownsamplingMaxPoints:
				opts.MaxPoints = m.value.(int)
			case DownsamplingGridInterval:
				opts.GridInterval = m.value.(int64)
			case DownsamplingDisabled:
				opts.Disabled = m.value.(bool)
			}
		}
	}

	if activeMode == "" && opts.HasSettings {
		opts.Mode = DownsamplingMaxPoints
		opts.MaxPoints = 10
	}

	return opts
}

func parseGridAggregation(s string) string {
	if s == "" {
		return ""
	}
	switch strings.ToUpper(s) {
	case "MAX", "MIN", "SUM", "AVG", "LAST", "COUNT":
		return strings.ToUpper(s)
	default:
		return ""
	}
}

func parseGapFilling(s string) string {
	if s == "" {
		return ""
	}
	switch strings.ToUpper(s) {
	case "NULL", "NONE", "PREVIOUS":
		return strings.ToUpper(s)
	default:
		return ""
	}
}

func parseMaxPoints(s string) int {
	if s == "" {
		return 0
	}
	if val, err := strconv.Atoi(s); err == nil && val >= 10 {
		return val
	}
	return 10
}

func parseGridInterval(s string) int64 {
	if s == "" {
		return 0
	}
	if val, err := strconv.ParseInt(s, 10, 64); err == nil && val > 0 {
		return val
	}
	return 0
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}

func getErrorDownsampling() DownsamplingOptions {
	return DownsamplingOptions{
		Mode:        DownsamplingMaxPoints,
		MaxPoints:   10,
		HasSettings: true,
	}
}
