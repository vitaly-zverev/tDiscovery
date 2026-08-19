FROM golang AS builder
WORKDIR /app/tDiscovery
COPY . /app/tDiscovery
RUN  make version > version.info

RUN  apt update && apt install -y protobuf-compiler && \
     go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
     go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
     make

RUN /app/tDiscovery/_out/tdiscovery --version

FROM scratch AS final
COPY --from=builder /app/tDiscovery/_out/tdiscovery /tdiscovery
COPY --from=builder /app/tDiscovery/version.info /version.info

ENTRYPOINT [ "/tdiscovery" ]

