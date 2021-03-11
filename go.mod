module github.com/drud/api-common

go 1.14

require (
	cloud.google.com/go/firestore v1.5.0
	firebase.google.com/go v3.13.0+incompatible
	github.com/bsm/redislock v0.7.0
	github.com/drud/billing-api v0.0.0-20210305152904-3061ed818d39
	github.com/drud/ddev-apis-go v0.0.0-20210311203357-c09d2c799f3f
	github.com/drud/org-operator v0.0.12
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gorilla/mux v1.8.0
	github.com/stripe/stripe-go v70.15.0+incompatible
	google.golang.org/api v0.41.0
	google.golang.org/genproto v0.0.0-20210311153111-e2979279ddde
	google.golang.org/grpc v1.36.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.5.0
)
