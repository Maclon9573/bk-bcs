module github.com/Tencent/bk-bcs/bcs-services/bcs-storage

go 1.14

replace (
	github.com/Tencent/bk-bcs/bcs-common v0.0.0-00010101000000-000000000000 => ../../bcs-common
	github.com/Tencent/bk-bcs/bcs-common v0.0.0-20220325081326-54930fbf5bb7 => ../../bcs-common
	github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2 v0.0.0-00010101000000-000000000000 => github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2 v0.0.0-20210927020148-09d631e874bc
	github.com/coreos/bbolt v1.3.4 => go.etcd.io/bbolt v1.3.4
	github.com/haproxytech/client-native v0.0.0-00010101000000-000000000000 => github.com/haproxytech/client-native v1.2.7
	go.etcd.io/bbolt v1.3.4 => github.com/coreos/bbolt v1.3.4
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
)

require (
	//github.com/Tencent/bk-bcs v1.23.0
	github.com/Tencent/bk-bcs/bcs-common v0.0.0-20220325081326-54930fbf5bb7
	github.com/deckarep/golang-set v1.8.0
	github.com/emicklei/go-restful v2.15.0+incompatible
	github.com/google/uuid v1.2.0
	github.com/micro/go-micro/v2 v2.9.1
	github.com/mitchellh/mapstructure v1.4.1
	github.com/prometheus/client_golang v1.11.0
	go.mongodb.org/mongo-driver v1.5.3
	go.opentelemetry.io/otel v1.4.1
	go.opentelemetry.io/otel/exporters/jaeger v1.4.1 // indirect
	go.opentelemetry.io/otel/sdk v1.4.1 // indirect
	go.opentelemetry.io/otel/trace v1.4.1
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
)
