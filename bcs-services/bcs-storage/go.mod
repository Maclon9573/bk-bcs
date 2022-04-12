module github.com/Tencent/bk-bcs/bcs-services/bcs-storage

go 1.14

replace (
	github.com/Tencent/bk-bcs/bcs-common => ../../bcs-common
	github.com/coreos/bbolt v1.3.4 => go.etcd.io/bbolt v1.3.4
	go.etcd.io/bbolt v1.3.4 => github.com/coreos/bbolt v1.3.4
)

require (
	github.com/Tencent/bk-bcs/bcs-common v0.0.0-00010101000000-000000000000
	github.com/asim/go-micro/v3 v3.7.1
	github.com/deckarep/golang-set v1.8.0
	github.com/emicklei/go-restful v2.15.0+incompatible
	github.com/google/uuid v1.3.0
	github.com/mitchellh/mapstructure v1.4.3
	github.com/prometheus/client_golang v1.12.1
	go.mongodb.org/mongo-driver v1.9.0
	go.opentelemetry.io/otel v1.6.3
	go.opentelemetry.io/otel/exporters/jaeger v1.6.3 // indirect
	go.opentelemetry.io/otel/trace v1.6.3
	golang.org/x/net v0.0.0-20220407224826-aac1ed45d8e3
)
