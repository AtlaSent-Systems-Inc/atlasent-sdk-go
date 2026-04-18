module github.com/atlasent-systems-inc/atlasent-sdk-go/connectrpc

go 1.24.7

require (
	connectrpc.com/connect v1.17.0
	github.com/atlasent-systems-inc/atlasent-sdk-go v0.0.0
)

require google.golang.org/protobuf v1.34.2 // indirect

replace github.com/atlasent-systems-inc/atlasent-sdk-go => ../
