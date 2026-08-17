# Имя выходного бинарника
BINARY_NAME = tdiscovery

# Путь к основному Go-файлу
MAIN_FILE = main.go

# Платформы для кросс-компиляции (опционально)
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

# Получаем информацию о версии из git
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.version=$(GIT_TAG) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME) -extldflags '-static'"

.PHONY: all proto build clean

all: proto build

proto:
	protoc -I ./discovery-api/api -I /usr/include --proto_path=./discovery-api  --go_out=./discovery-api  --go-grpc_out=./discovery-api \
	--go_opt=paths=source_relative  --go-grpc_opt=paths=source_relative  --descriptor_set_out=./discovery-api/api/descriptor.pb \
      	--include_imports --experimental_allow_proto3_optional api/v1alpha1/server/cluster.proto

build:
	GOOS=$(OS) GOARCH=$(ARCH) go build  -tags netgo $(LDFLAGS) -o _out/$(BINARY_NAME) $(MAIN_FILE)

.PHONY: version
version:
	@echo "Version: $(GIT_TAG)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Build time: $(BUILD_TIME)"

clean:
	rm -f _out/$(BINARY_NAME)
