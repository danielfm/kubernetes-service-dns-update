.PHONY: clean

GO=CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go
TAG=v1.4.0
BIN=kubernetes-service-dns-update
IMAGE=danielfm/$(BIN)

all: image
	docker push $(IMAGE):$(TAG)

build:
	$(GO) build -a -installsuffix cgo -o $(BIN) .

image: build
	docker build -t $(IMAGE):$(TAG) .

clean:
	rm $(BIN)

cover:
	rm -f cover.out coverage.html
	go test -coverprofile cover.out
	go tool cover -html=cover.out -o coverage.html
