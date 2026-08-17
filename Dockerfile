FROM golang AS builder
WORKDIR /app
COPY . /app/tDiscovery
RUN cd /app/tDiscovery && make version > version.info

RUN  apt update && apt install -y protobuf-compiler && \
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
go install github.com/go-bindata/go-bindata/v3/go-bindata@latest && \
    git clone https://github.com/siderolabs/discovery-api /app/tDiscovery/discovery-api && \
    cd /app/tDiscovery && make proto && \
    cd /app/tDiscovery && go-bindata -o internal/descriptor/bindata.go -pkg descriptor ./discovery-api/api/descriptor.pb && \
    cd /app/tDiscovery && go mod tidy && make && cat version.info

RUN /app/tDiscovery/_out/tdiscovery --help

FROM scratch AS final
COPY --from=builder /app/tDiscovery/_out/tdiscovery /tdiscovery
COPY --from=builder /app/tDiscovery/version.info /version.info

ENTRYPOINT [ "/tdiscovery" ]

