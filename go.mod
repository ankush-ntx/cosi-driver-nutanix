module nutanix-cosi-driver

go 1.23.0

toolchain go1.23.4

replace sigs.k8s.io/container-object-storage-interface/proto => sigs.k8s.io/container-object-storage-interface/proto v0.0.0-20241219204011-a29e5f67a9a8

require (
	github.com/aws/aws-sdk-go v1.55.5
	google.golang.org/grpc v1.69.4
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apimachinery v0.32.0
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/container-object-storage-interface/proto v0.0.0
	sigs.k8s.io/container-object-storage-interface/sidecar v0.0.0-20241219204011-a29e5f67a9a8
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
)
