FROM golang AS builder
WORKDIR /app
COPY . /app/tDiscovery
RUN  apt update && apt install -y protobuf-compiler && \
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    git clone https://github.com/siderolabs/discovery-api /app/tDiscovery/discovery-api && \
    cd /app/tDiscovery && go mod tidy && make 

RUN /app/tDiscovery/_out/tdiscovery --help

FROM scratch AS final
COPY --from=builder /app/tDiscovery/_out/tdiscovery /tdiscovery

ENTRYPOINT [ "/tdiscovery" ]

