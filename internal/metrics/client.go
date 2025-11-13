package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"keda-external-scaler-yc-monitoring/internal/auth"
	"keda-external-scaler-yc-monitoring/internal/config"
	"keda-external-scaler-yc-monitoring/internal/logger"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	auth   *auth.YandexAuth
	config *config.Config
}

type MetricQuery struct {
	Query        string                 `json:"query"`
	FromTime     string                 `json:"fromTime"`
	ToTime       string                 `json:"toTime"`
	Downsampling map[string]interface{} `json:"downsampling,omitempty"`
}

type MetricResponse struct {
	Metrics []struct {
		Name       string            `json:"name"`
		Labels     map[string]string `json:"labels"`
		Type       string            `json:"type"`
		Timeseries struct {
			Timestamps   []int64       `json:"timestamps"`
			DoubleValues []interface{} `json:"doubleValues,omitempty"`
			Int64Values  []int64       `json:"int64Values,omitempty"`
		} `json:"timeseries"`
	} `json:"metrics"`
}

func NewClient(auth *auth.YandexAuth, cfg *config.Config) *Client {
	return &Client{
		auth:   auth,
		config: cfg,
	}
}

func (c *Client) QueryMetric(options QueryOptions, logger *logger.Logger) (float64, error) {
	logger.Debug("Querying metric: query=%s, folder=%s, hasDownsampling=%t, timeWindowOffset=%s",
		options.Query, options.FolderID, options.Downsampling.HasSettings, options.TimeWindowOffset)

	token, err := c.auth.GetToken()
	if err != nil {
		logger.Error("Failed to get IAM token: %v", err)
		return 0, fmt.Errorf("failed to get IAM token: %v", err)
	}

	timeWindow := 5 * time.Minute
	if options.TimeWindow != "" {
		if duration, err := time.ParseDuration(options.TimeWindow); err == nil {
			timeWindow = duration
			logger.Debug("Using custom time window: %v", timeWindow)
		} else {
			logger.Warn("Invalid timeWindow format '%s', using default 5m", options.TimeWindow)
		}
	}

	timeWindowOffset := 30 * time.Second
	if options.TimeWindowOffset != "" {
		if duration, err := time.ParseDuration(options.TimeWindowOffset); err == nil {
			timeWindowOffset = duration
			logger.Debug("Using custom time window offset: %v", timeWindowOffset)
		} else {
			logger.Warn("Invalid timeWindowOffset format '%s', using default 30s", options.TimeWindowOffset)
		}
	}

	now := time.Now().UTC()
	endTime := now.Add(-timeWindowOffset)
	startTime := endTime.Add(-timeWindow)

	startTimeStr := startTime.Format("2006-01-02T15:04:05Z")
	endTimeStr := endTime.Format("2006-01-02T15:04:05Z")

	logger.Debug("Time range: %s to %s (window: %v, offset: %v)",
		startTimeStr, endTimeStr, timeWindow, timeWindowOffset)

	payload := MetricQuery{
		Query:    options.Query,
		FromTime: startTimeStr,
		ToTime:   endTimeStr,
	}

	if options.Downsampling.HasSettings {
		downsampling := map[string]interface{}{}

		if options.Downsampling.GridAggregation != "" {
			downsampling["gridAggregation"] = options.Downsampling.GridAggregation
			logger.Debug("Using gridAggregation: %s", options.Downsampling.GridAggregation)
		}

		if options.Downsampling.GapFilling != "" {
			downsampling["gapFilling"] = options.Downsampling.GapFilling
			logger.Debug("Using gapFilling: %s", options.Downsampling.GapFilling)
		}

		switch options.Downsampling.Mode {
		case DownsamplingMaxPoints:
			downsampling["maxPoints"] = options.Downsampling.MaxPoints
			logger.Debug("Using maxPoints: %d", options.Downsampling.MaxPoints)
		case DownsamplingGridInterval:
			downsampling["gridInterval"] = strconv.FormatInt(options.Downsampling.GridInterval, 10)
			logger.Debug("Using gridInterval: %d ms", options.Downsampling.GridInterval)
		case DownsamplingDisabled:
			downsampling["disabled"] = true
			logger.Debug("Downsampling disabled")
		}

		payload.Downsampling = downsampling
	} else {
		logger.Debug("No downsampling settings provided, using server defaults")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal payload: %v", err)
		return 0, fmt.Errorf("failed to marshal payload: %v", err)
	}

	url := c.config.GetMonitoringURL(options.FolderID)

	logger.LogAPIRequest(url, payload, payloadBytes)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		logger.Error("Failed to create request: %v", err)
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: c.config.APITimeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to execute request: %v", err)
		return 0, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response: %v", err)
		return 0, fmt.Errorf("failed to read response: %v", err)
	}

	logger.LogAPIResponse(resp.StatusCode, body)

	if resp.StatusCode != http.StatusOK {
		logger.Error("API error: status=%d, body=%s", resp.StatusCode, string(body))
		return 0, fmt.Errorf("API error: %d, %s", resp.StatusCode, string(body))
	}

	var metricResp MetricResponse
	if err := json.Unmarshal(body, &metricResp); err != nil {
		logger.Error("Failed to parse response: %v", err)
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	logger.LogParsedMetrics(metricResp)

	logger.LogMetrics(metricResp)

	var allValues []float64
	var lastValid *float64
	nanCount := 0
	totalCount := 0

	for i, metric := range metricResp.Metrics {
		logger.Debug("Processing metric %d: name=%s, labels=%v", i, metric.Name, metric.Labels)

		var allMetricValues []interface{}

		for _, val := range metric.Timeseries.DoubleValues {
			allMetricValues = append(allMetricValues, val)
		}

		for _, val := range metric.Timeseries.Int64Values {
			allMetricValues = append(allMetricValues, float64(val))
		}

		totalCount += len(allMetricValues)

		for _, val := range allMetricValues {
			if str, ok := val.(string); ok && str == "NaN" {
				nanCount++
			}
		}

		metricValues, newLastValid := ExtractValidValues(
			allMetricValues,
			options.NaNStrategy,
			lastValid,
		)
		lastValid = newLastValid

		logger.Debug("Extracted %d valid values from metric %d", len(metricValues), i)

		if len(metricValues) > 0 && options.TimeSeriesAggregation != "" {
			tsValue, err := Aggregate(metricValues, options.TimeSeriesAggregation)
			if err == nil {
				allValues = append(allValues, tsValue)
				logger.Debug("Time series aggregation (%s): %v -> %f",
					options.TimeSeriesAggregation, metricValues, tsValue)
			}
		} else {
			allValues = append(allValues, metricValues...)
		}
	}

	logger.LogClientProcessing(totalCount, nanCount, len(allValues), allValues, options.NaNStrategy)

	if options.NaNStrategy == NaNStrategyError && nanCount > 0 && len(allValues) == 0 {
		logger.Error("All values are NaN with error strategy")
		return 0, fmt.Errorf("all metric values are NaN")
	}

	if len(allValues) == 0 {
		logger.Warn("No valid values found after processing (total: %d, NaN: %d)", totalCount, nanCount)

		if options.NaNStrategy == NaNStrategyZero {
			logger.Info("No data available with zero strategy, returning 0")
			return 0, nil
		}

		return 0, fmt.Errorf("no valid metric data available")
	}

	result, err := Aggregate(allValues, options.AggregationMethod)
	if err != nil {
		logger.Error("Aggregation failed: %v", err)
		return 0, err
	}

	logger.LogAggregation(string(options.AggregationMethod), allValues, result)
	logger.Info("Final metric value: %f (processed %d values, %d were NaN)", result, totalCount, nanCount)

	return result, nil
}
