package server

import (
	"context"
	"fmt"
	protos "keda-external-scaler-yc-monitoring/gen/proto/externalscaler"
	"keda-external-scaler-yc-monitoring/internal/auth"
	"keda-external-scaler-yc-monitoring/internal/config"
	"keda-external-scaler-yc-monitoring/internal/logger"
	"keda-external-scaler-yc-monitoring/internal/metrics"
	"strconv"
)

type ExternalScalerServer struct {
	protos.UnimplementedExternalScalerServer
	metricsClient *metrics.Client
	config        *config.Config
}

func NewExternalScalerServer(keyPath string, cfg *config.Config) (*ExternalScalerServer, error) {
	auth, err := auth.NewYandexAuth(keyPath, cfg)
	if err != nil {
		return nil, err
	}

	return &ExternalScalerServer{
		metricsClient: metrics.NewClient(auth, cfg),
		config:        cfg,
	}, nil
}

func (s *ExternalScalerServer) IsActive(ctx context.Context, req *protos.ScaledObjectRef) (*protos.IsActiveResponse, error) {
	metadata := req.ScalerMetadata
	log := logger.NewLogger(metadata, req.Name)

	log.Debug("IsActive called: name=%s, namespace=%s", req.Name, req.Namespace)

	options := metrics.QueryOptions{
		Query:             metadata["query"],
		FolderID:          metadata["folderId"],
		NaNStrategy:       metrics.ParseNaNStrategy(metadata["nanStrategy"]),
		AggregationMethod: metrics.ParseAggregationMethod(metadata["aggregationMethod"]),
		TimeWindow:        metadata["timeWindow"],
		TimeWindowOffset:  metadata["timeWindowOffset"],
		Downsampling:      metrics.ParseDownsamplingOptions(metadata),
	}

	value, err := s.metricsClient.QueryMetric(options, log)
	if err != nil {
		log.Error("Error querying metric: %v", err)
		log.LogKEDAResponse("IsActive", false, 0, 0, err)
		return &protos.IsActiveResponse{Result: false}, nil
	}

	result := value > 0
	log.Info("IsActive result: %t (value: %f)", result, value)

	log.LogKEDAResponse("IsActive", result, value, 0, nil)

	return &protos.IsActiveResponse{Result: result}, nil
}

func (s *ExternalScalerServer) GetMetricSpec(ctx context.Context, req *protos.ScaledObjectRef) (*protos.GetMetricSpecResponse, error) {
	metadata := req.ScalerMetadata
	log := logger.NewLogger(metadata, req.Name)

	log.Debug("GetMetricSpec called: name=%s, namespace=%s", req.Name, req.Namespace)

	targetStr := metadata["targetValue"]
	targetValue := 80.0
	if targetStr != "" {
		if parsed, err := strconv.ParseFloat(targetStr, 64); err == nil {
			targetValue = parsed
		} else {
			log.Warn("Failed to parse targetValue '%s', using default: %f", targetStr, targetValue)
		}
	}

	metricSpec := &protos.MetricSpec{
		MetricName:      "yandex_monitoring_metric",
		TargetSizeFloat: targetValue,
	}

	log.Info("Returning metric spec with target value: %f", targetValue)

	return &protos.GetMetricSpecResponse{
		MetricSpecs: []*protos.MetricSpec{metricSpec},
	}, nil
}

func (s *ExternalScalerServer) GetMetrics(ctx context.Context, req *protos.GetMetricsRequest) (*protos.GetMetricsResponse, error) {
	metadata := req.ScaledObjectRef.ScalerMetadata

	log := logger.NewLogger(metadata, req.ScaledObjectRef.Name)

	log.Debug("GetMetrics called: name=%s, namespace=%s, metadata=%v",
		req.ScaledObjectRef.Name, req.ScaledObjectRef.Namespace, metadata)

	targetValue := 80.0
	if targetStr := metadata["targetValue"]; targetStr != "" {
		if parsed, err := strconv.ParseFloat(targetStr, 64); err == nil {
			targetValue = parsed
		}
	}

	options := metrics.QueryOptions{
		Query:                 metadata["query"],
		FolderID:              metadata["folderId"],
		NaNStrategy:           metrics.ParseNaNStrategy(metadata["nanStrategy"]),
		AggregationMethod:     metrics.ParseAggregationMethod(metadata["aggregationMethod"]),
		TimeSeriesAggregation: metrics.ParseAggregationMethod(metadata["timeSeriesAggregation"]),
		TimeWindow:            metadata["timeWindow"],
		TimeWindowOffset:      metadata["timeWindowOffset"],
		Downsampling:          metrics.ParseDownsamplingOptions(metadata),
	}

	value, err := s.metricsClient.QueryMetric(options, log)
	if err != nil {
		log.Error("Failed to query metric: %v", err)
		log.LogKEDAResponse("GetMetrics", false, 0, targetValue, err)
		return nil, fmt.Errorf("failed to query metric: %v", err)
	}

	log.Info("Returning metric value: %f for metric: %s", value, req.MetricName)

	log.LogKEDAResponse("GetMetrics", value > 0, value, targetValue, nil)

	metricVal := &protos.MetricValue{
		MetricName:       req.MetricName,
		MetricValueFloat: value,
	}

	return &protos.GetMetricsResponse{
		MetricValues: []*protos.MetricValue{metricVal},
	}, nil
}
