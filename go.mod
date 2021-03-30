module github.com/drud/api-common

go 1.14

require (
	cloud.google.com/go/firestore v1.5.0
	firebase.google.com/go v3.13.0+incompatible
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/onsi/ginkgo v1.15.0 // indirect
	github.com/onsi/gomega v1.10.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	go.uber.org/zap v1.14.0 // indirect
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899 // indirect
	google.golang.org/api v0.41.0 // indirect
	google.golang.org/genproto v0.0.0-20210311153111-e2979279ddde
	google.golang.org/grpc v1.36.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	k8s.io/api v0.18.3
	k8s.io/apiextensions-apiserver v0.17.3 // indirect
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible // indirect
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19 // indirect
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
)
