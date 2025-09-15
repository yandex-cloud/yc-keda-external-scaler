package main

import (
	"log"
	"net"
	"net/http"

	protos "keda-external-scaler-yc-monitoring/gen/proto/externalscaler"
	"keda-external-scaler-yc-monitoring/internal/config"
	"keda-external-scaler-yc-monitoring/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.LoadConfig()
	
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	go func() {
		http.HandleFunc(cfg.HealthPath, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		log.Printf("Starting HTTP server for health checks on :%s", cfg.HTTPPort)
		if err := http.ListenAndServe(":"+cfg.HTTPPort, nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	scalerServer, err := server.NewExternalScalerServer(cfg.KeyPath, cfg)
	if err != nil {
		log.Fatalf("Failed to create scaler server: %v", err)
	}

	grpcServer := grpc.NewServer()
	protos.RegisterExternalScalerServer(grpcServer, scalerServer)

	reflection.Register(grpcServer)

	log.Printf("Starting gRPC server on :%s", cfg.GRPCPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
