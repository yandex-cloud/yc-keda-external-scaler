package metrics

import (
	"fmt"
	"strconv"
)

func Aggregate(values []float64, method AggregationMethod) (float64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("no values to aggregate")
	}

	switch method {
	case AggregationSum:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum, nil

	case AggregationAvg:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil

	case AggregationMax:
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max, nil

	case AggregationMin:
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min, nil

	case AggregationLast:
		return values[len(values)-1], nil

	default:
		return 0, fmt.Errorf("unknown aggregation method: %s", method)
	}
}

func ExtractValidValues(values []interface{}, strategy NaNStrategy, lastValid *float64) ([]float64, *float64) {
	var result []float64

	for _, val := range values {
		switch v := val.(type) {
		case float64:
			result = append(result, v)
			lastValid = &v
		case int64:
			floatVal := float64(v)
			result = append(result, floatVal)
			lastValid = &floatVal
		case string:
			if v == "NaN" {
				switch strategy {
				case NaNStrategyZero:
					result = append(result, 0.0)
				case NaNStrategyLastValid:
					if lastValid != nil {
						result = append(result, *lastValid)
					}
				case NaNStrategySkip:
				case NaNStrategyError:
				}
			} else {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					result = append(result, f)
					lastValid = &f
				}
			}
		}
	}

	return result, lastValid
}
