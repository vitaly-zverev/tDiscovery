# Имя выходного бинарника
BINARY_NAME = talos-discovery

# Путь к основному Go-файлу
MAIN_FILE = main.go

# Платформы для кросс-компиляции (опционально)
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

.PHONY: all proto build clean

all: proto build

proto:
	protoc -I ../discovery-api/api --proto_path=../discovery-api  --go_out=../discovery-api  --go-grpc_out=../discovery-api --go_opt=paths=source_relative  --go-grpc_opt=paths=source_relative  api/v1alpha1/server/cluster.proto

build:
	GOOS=$(OS) GOARCH=$(ARCH) go build -o _out/$(BINARY_NAME) $(MAIN_FILE)

clean:
	rm -f _out/$(BINARY_NAME)