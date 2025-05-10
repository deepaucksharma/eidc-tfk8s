.PHONY: build test lint clean docs-sync

build:
	go build -v ./...

test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

docs-sync:
	python tools/test-matrix-builder/test_matrix_builder.py

deploy-dev:
	helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml

deploy-clean:
	helm uninstall nrdot-devlab

all: clean build test