package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type LogLevel int

const (
	LogLevelNone LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

type Logger struct {
	level          LogLevel
	logMetrics     bool
	logAggregation bool
	scalerName     string
}

func NewLogger(metadata map[string]string, scalerName string) *Logger {
	return &Logger{
		level:          parseLogLevel(metadata["logLevel"]),
		logMetrics:     parseBool(metadata["logMetrics"], false),
		logAggregation: parseBool(metadata["logAggregation"], false),
		scalerName:     scalerName,
	}
}

func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "none", "off":
		return LogLevelNone
	default:
		return LogLevelInfo
	}
}

func parseBool(value string, defaultValue bool) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "1", "on":
		return true
	case "false", "no", "0", "off":
		return false
	default:
		return defaultValue
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("[DEBUG] [%s] %s", l.scalerName, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		log.Printf("[INFO] [%s] %s", l.scalerName, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level >= LogLevelWarn {
		log.Printf("[WARN] [%s] %s", l.scalerName, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l.level >= LogLevelError {
		log.Printf("[ERROR] [%s] %s", l.scalerName, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) LogMetrics(metrics interface{}) {
	if l.logMetrics && l.level >= LogLevelDebug {
		log.Printf("[METRICS] [%s] Raw metrics: %+v", l.scalerName, metrics)
	}
}

func (l *Logger) LogAggregation(method string, values []float64, result float64) {
	if l.logAggregation && l.level >= LogLevelDebug {
		log.Printf("[AGGREGATION] [%s] Method: %s, Values: %v, Result: %f",
			l.scalerName, method, values, result)
	}
}

func (l *Logger) LogAPIRequest(url string, payload interface{}, payloadBytes []byte) {
	if l.level >= LogLevelDebug {
		log.Printf("[API-REQUEST] [%s] URL: %s", l.scalerName, url)
		log.Printf("[API-REQUEST] [%s] Payload: %s", l.scalerName, string(payloadBytes))

		if prettyJSON, err := json.MarshalIndent(payload, "", "  "); err == nil {
			log.Printf("[API-REQUEST] [%s] Structured payload:\n%s", l.scalerName, string(prettyJSON))
		}
	}
}

func (l *Logger) LogAPIResponse(statusCode int, body []byte) {
	if l.level >= LogLevelDebug {
		log.Printf("[API-RESPONSE] [%s] Status: %d", l.scalerName, statusCode)
		log.Printf("[API-RESPONSE] [%s] Body: %s", l.scalerName, string(body))

		var jsonData interface{}
		if err := json.Unmarshal(body, &jsonData); err == nil {
			if prettyJSON, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
				log.Printf("[API-RESPONSE] [%s] Formatted response:\n%s", l.scalerName, string(prettyJSON))
			}
		}
	}
}

func (l *Logger) LogParsedMetrics(metrics interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("[PARSED-METRICS] [%s] Parsed metrics structure: %+v", l.scalerName, metrics)

		if prettyJSON, err := json.MarshalIndent(metrics, "", "  "); err == nil {
			log.Printf("[PARSED-METRICS] [%s] Formatted metrics:\n%s", l.scalerName, string(prettyJSON))
		}
	}
}

func (l *Logger) LogClientProcessing(totalCount, nanCount, validCount int, allValues []float64, nanStrategy interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("[CLIENT-PROCESSING] [%s] Data summary: total=%d, NaN=%d, valid=%d, nanStrategy=%v",
			l.scalerName, totalCount, nanCount, validCount, nanStrategy)
		log.Printf("[CLIENT-PROCESSING] [%s] All extracted values: %v", l.scalerName, allValues)

		if len(allValues) > 0 {
			sum := 0.0
			min := allValues[0]
			max := allValues[0]
			for _, v := range allValues {
				sum += v
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
			avg := sum / float64(len(allValues))
			log.Printf("[CLIENT-PROCESSING] [%s] Value statistics: min=%.6f, max=%.6f, avg=%.6f, sum=%.6f",
				l.scalerName, min, max, avg, sum)
		}
	}
}

func (l *Logger) LogKEDAResponse(method string, active bool, currentValue, targetValue float64, err error) {
	if l.level >= LogLevelDebug {
		if err != nil {
			log.Printf("[KEDA-RESPONSE] [%s] Method: %s, Error: %v", l.scalerName, method, err)
		} else {
			if method == "GetMetrics" {
				ratio := currentValue / targetValue
				scaleDirection := "STABLE"
				if currentValue > targetValue {
					scaleDirection = "SCALE-UP"
				} else if currentValue < targetValue {
					scaleDirection = "SCALE-DOWN"
				}

				log.Printf("[KEDA-RESPONSE] [%s] Method: %s, Current: %.6f, Target: %.6f, Ratio: %.6f, Direction: %s",
					l.scalerName, method, currentValue, targetValue, ratio, scaleDirection)
			} else {
				log.Printf("[KEDA-RESPONSE] [%s] Method: %s, Active: %t, Value: %.6f",
					l.scalerName, method, active, currentValue)
			}
		}
	}
}
