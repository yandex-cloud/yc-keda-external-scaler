FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o keda-external-scaler-yc-monitoring ./cmd/keda-external-scaler-yc-monitoring

FROM alpine:3.22
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/keda-external-scaler-yc-monitoring .
RUN chown nobody:nobody /app/keda-external-scaler-yc-monitoring

USER nobody

EXPOSE 8080 8081
CMD ["./keda-external-scaler-yc-monitoring"]