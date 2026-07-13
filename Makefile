VERSION ?= 1.3.0
CHART_DIR ?= helm/yc-keda-external-scaler
DIST_DIR ?= dist
IMAGE ?= cr.yandex/sol/keda/yc-keda-external-scaler:v$(VERSION)

.PHONY: test vet helm-lint helm-test package verify docker-build clean

test:
	go test ./...

vet:
	go vet ./...

helm-lint:
	helm lint --strict $(CHART_DIR) --set-string secret.data=test-key

helm-test:
	./hack/test-chart.sh

package:
	mkdir -p $(DIST_DIR)
	helm package $(CHART_DIR) --destination $(DIST_DIR)

verify: test vet helm-lint helm-test

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 --tag $(IMAGE) .

clean:
	rm -rf $(DIST_DIR)
