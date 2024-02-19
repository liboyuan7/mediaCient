module github.com/streamFunc/mediaClient

go 1.19

require (
	github.com/appcrash/media v0.0.4-0.20231205004418-e70ef46d7dea
	github.com/streamFunc/RTPGoAPI v1.0.10
	google.golang.org/grpc v1.60.1
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231002182017-d307bd883b97 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

//replace github.com/appcrash/media => ../media
//replace github.com/streamFunc/RTPGoAPI => ../RTPGoAPI
