module talos-discovery

go 1.24.0

replace github.com/siderolabs/discovery-api => ./discovery-api

require github.com/siderolabs/discovery-api v0.1.6

require google.golang.org/grpc v1.78.0

require (
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
