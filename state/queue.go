package state

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	pb "github.com/drud/billing-api/gen/live/billing/v1alpha1"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"k8s.io/klog"
)

const mutexPrefix = "mutex"

var (
	LockRetries = 3
	Expiry      = time.Second * 5
)

func getDatabasePath() string {
	return fmt.Sprintf("projects/%s/databases/(default)", projectID)
}

func protectRoutine(f func()) (retErr error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					if !errors.Is(err, context.Canceled) {
						retErr = err
					}
				}
			}
		}()

		f()
	}()
	return
}

func (s *Manager) UpstreamWatch(ctx context.Context, collection string, id string) (retErr error) {

	// Exception Recovery
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				if !errors.Is(err, context.Canceled) {
					retErr = err
				}
			}
		}
	}()

	aquireFunc := func() (*redislock.Lock, error) {
		return s.locker.Obtain(ctx, fmt.Sprintf("%s-populate-subscriptions", id), 5*time.Second, &redislock.Options{
			RetryStrategy: redislock.ExponentialBackoff(time.Second, 15*time.Second),
		})
	}

	lock, lockerr := aquireFunc()
	if lockerr != nil {
		return lockerr
	}
	defer lock.Release(ctx)

	listenCtx := metadata.AppendToOutgoingContext(ctx,
		"google-cloud-resource-prefix", getDatabasePath(),
	)
	// List All Subscriptions and Watch
	listenC, err := s.firestoreGRPC.Listen(listenCtx)
	if err != nil {
		return status.Errorf(codes.Internal, "upstream listen init error: %v", err)
	}
	err = listenC.Send(&firestorepb.ListenRequest{
		Database: getDatabasePath(),
		TargetChange: &firestorepb.ListenRequest_AddTarget{
			AddTarget: &firestorepb.Target{
				TargetType: &firestorepb.Target_Query{
					Query: &firestorepb.Target_QueryTarget{
						Parent: fmt.Sprintf("%s/documents", getDatabasePath()),
						QueryType: &firestorepb.Target_QueryTarget_StructuredQuery{
							StructuredQuery: &firestorepb.StructuredQuery{
								From: []*firestorepb.StructuredQuery_CollectionSelector{
									{
										CollectionId: collection,
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return status.Errorf(codes.Internal, "upstream listen request error: %v", err)
	}
	// receive can either return a response
	// or it can error, which we will return and release the lock
	// or it can block
	// in all cases but error we want to hold our lock

	respChan := make(chan *firestorepb.ListenResponse)
	err = protectRoutine(func() {
		for {
			resp, err := listenC.Recv()
			if err != nil {
				panic(err)
			}
			respChan <- resp
		}
	})
	if err != nil {
		panic(err)
	}
	liveness := time.NewTicker(time.Second)

	for {
		select {
		case resp := <-respChan:
			switch t := resp.ResponseType.(type) {
			case *firestorepb.ListenResponse_DocumentChange:
				lock.Refresh(ctx, time.Second*5, &redislock.Options{})
				doc := t.DocumentChange.GetDocument()
				var sub pb.Subscription
				err := Deserialize(doc, &sub)
				if err != nil {
					klog.Errorf("failed deserialization %s: %v", doc.Name, err)
					continue
				}
				for {
					err = s.Enqueue(ctx, id, &sub)
					if err != nil {
						if err != redislock.ErrNotObtained {
							panic(err)
						}
						continue
					}
					klog.Infof("enqueued subscription %s", sub.GetId())
					break
				}
			}
		case <-liveness.C:
			if err := lock.Refresh(ctx, time.Second*5, &redislock.Options{}); err != nil {
				if errors.Is(err, context.Canceled) {
					break
				}
				klog.Errorf("lock refresh error: %v", err)
			}
			if err := s.RefreshExpire(ctx, id); err != nil {
				if errors.Is(err, context.Canceled) {
					break
				}
				klog.Errorf("failed to refresh dataset %s expiry: %v", id, err)
			}
		}
	}
}

func (s *Manager) RefreshExpire(ctx context.Context, queue string) error {
	_, err := s.redis.Expire(ctx, queue, Expiry).Result()
	if err != nil {
		return err
	}
	return nil
}

func (s *Manager) Enqueue(ctx context.Context, queue string, msg protoreflect.ProtoMessage) error {
	key := fmt.Sprintf("%s-%s", mutexPrefix, queue)
	//TODO: We may want to never expire if we are listening to firestore
	mutex, err := s.locker.Obtain(ctx, key, 5*time.Second, &redislock.Options{
		RetryStrategy: redislock.ExponentialBackoff(time.Second, 10*time.Second),
	})
	if err != nil {
		return err
	}
	defer mutex.Release(ctx)

	if err := s.RefreshExpire(ctx, queue); err != nil {
		return err
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = s.redis.SAdd(ctx, queue, data).Result()
	if err != nil {
		return err
	}
	klog.Infof("enqueued %s: %v", queue, msg)
	return nil
}

func (s *Manager) Dequeue(ctx context.Context, queue string, msg protoreflect.ProtoMessage, timeout time.Duration) error {
	key := fmt.Sprintf("%s-%s", mutexPrefix, queue)
	//TODO: We may want to never expire if we are listening to firestore
	mutex, err := s.locker.Obtain(ctx, key, 5*time.Second, &redislock.Options{
		RetryStrategy: redislock.ExponentialBackoff(time.Second, 5*time.Second),
	})
	if err != nil {
		return err
	}
	defer mutex.Release(ctx)

	if err := s.RefreshExpire(ctx, queue); err != nil {
		return err
	}

	// These functions should be threaded and may want to wait forever, or until the ctx is not valid
	obj, err := s.redis.SPop(ctx, queue).Result()
	if err != nil {
		return err
	}
	err = proto.Unmarshal([]byte(obj), msg)
	if err != nil {
		return err
	}
	return nil
}
