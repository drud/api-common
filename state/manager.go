package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strings"

	pb "github.com/drud/billing-api/gen/live/billing/v1alpha1"
	adminpb "github.com/drud/ddev-apis-go/live/administration/v1alpha1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"

	"cloud.google.com/go/firestore"
	firestorev1 "cloud.google.com/go/firestore/apiv1"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	gtransport "google.golang.org/api/transport/grpc"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"

	orgapi "github.com/drud/org-operator/pkg/apis/orgs/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (

	// Firestore collection names

	// CollectionCustomer - the collection to serialize stripe customer proto messages to
	CollectionCustomer = "customers"
	// CollectionSubscription - the collection to serialize stripe subscription proto messages to
	CollectionSubscription = "subscriptions"
	// CollectionWorkspace - the collection to serialize stripe subscription workspace proto messages to
	CollectionWorkspace = "workspaces"
	// CollectionPlan - the collection to serialize stripe plans proto messages to
	CollectionPlan = "plans"
	// CollectionProducts - the collection to serialize stripe product proto messages to
	CollectionProducts = "products"
)

var (
	projectID string
)

func init() {

	if project := os.Getenv("PROJECT_ID"); project != "" {
		projectID = project
	} else {
		// Retrieve the projectID from the metadata service
		req, err := http.NewRequest(http.MethodGet, "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
		if err != nil {
			klog.Fatalf("Failed to determine google project id: %v", err)
		}
		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			klog.Fatalf("Failed to request google project id: %v", err)
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			klog.Fatalf("Could not parse google project id: %v", err)
		}
		projectID = string(data)
	}
}

/*
The state manager manages all state transactions while keeping state out of this replicated service by:
- serving the stripe webhook that populates firestore
- streaming data handing buffering of streams from client connections which may be spread across replicas
- managing locks on distributed state
*/
type Manager struct {
	firestore     *firestore.Client
	firestoreGRPC firestorepb.FirestoreClient
	stripe        *client.API
	redis         *redis.Client
	locker        *redislock.Client
	crClient      crclient.Client
}

func generic(obj interface{}) (map[string]interface{}, error) {

	generic, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var genericMap map[string]interface{}
	err = json.Unmarshal(generic, &genericMap)
	if err != nil {
		return nil, err
	}
	return genericMap, nil
}

func (s *Manager) init() {

	stripeKey := os.Getenv("STRIPE_KEY")
	stripeClient := &client.API{}
	stripeConfig := &stripe.BackendConfig{
		MaxNetworkRetries: 3,
	}
	stripeClient.Init(stripeKey, &stripe.Backends{
		API:     stripe.GetBackendWithConfig(stripe.APIBackend, stripeConfig),
		Uploads: stripe.GetBackendWithConfig(stripe.UploadsBackend, stripeConfig),
	})

	custIDs := make(map[string]bool)
	subIDs := make(map[string]bool)
	planIDs := make(map[string]bool)
	productsIDs := make(map[string]bool)

	// Run this initialization in a single transaction
	err := s.firestore.RunTransaction(context.Background(), func(c context.Context, txn *firestore.Transaction) error {

		subiter := stripeClient.Subscriptions.List(&stripe.SubscriptionListParams{})
		for subiter.Next() {
			item := subiter.Current()
			if sub, ok := item.(*stripe.Subscription); ok {
				if sub.EndedAt == 0 && sub.CanceledAt == 0 {
					subIDs[sub.ID] = true
				}
				err := s.CreateOrUpdateSubscription(txn, sub)
				if err != nil {
					klog.Infof("Failed to update subscription state %s: %v", sub.ID, err)
				}
			}
		}

		custiter := stripeClient.Customers.List(&stripe.CustomerListParams{})
		for custiter.Next() {
			item := custiter.Current()
			if customer, ok := item.(*stripe.Customer); ok {
				custIDs[customer.ID] = true
				err := s.CreateOrUpdateCustomer(txn, customer)
				if err != nil {
					klog.Infof("Failed to update customer state %s: %v", customer.ID, err)
				}
			}
		}

		planiter := stripeClient.Plans.List(&stripe.PlanListParams{})
		for planiter.Next() {
			item := planiter.Current()
			if plan, ok := item.(*stripe.Plan); ok {
				planIDs[plan.ID] = true
				err := s.CreateOrUpdatePlan(txn, plan)
				if err != nil {
					klog.Infof("Failed to update plan state %s: %v", plan.ID, err)
				}
			}
		}

		productiter := stripeClient.Products.List(&stripe.ProductListParams{})
		for productiter.Next() {
			item := productiter.Current()
			if product, ok := item.(*stripe.Product); ok {
				productsIDs[product.ID] = true
				err := s.CreateOrUpdateProduct(txn, product)
				if err != nil {
					klog.Infof("Failed to update product state %s: %v", product.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		klog.Fatalf("failed to initialize firestore with stripe: %v", err)
	}

	s.cleanByID(CollectionCustomer, custIDs)
	s.cleanByID(CollectionSubscription, subIDs)
	s.cleanByID(CollectionPlan, planIDs)
	s.cleanByID(CollectionProducts, productsIDs)
	s.cleanWorkspaces(subIDs)

	klog.Info("firestore initialization complete.")
}

func (s *Manager) cleanByID(collection string, ids map[string]bool) {

	keys := make([]string, 0)
	for id := range ids {
		keys = append(keys, id)
	}
	ctx := context.Background()
	iter := s.firestore.Collection(collection).Documents(ctx)
	defer iter.Stop()
	err := s.firestore.RunTransaction(ctx, func(c context.Context, txn *firestore.Transaction) error {
		for {
			doc, err := iter.Next()
			if err != nil {
				if err == iterator.Done {
					return nil
				}
				return err
			}
			if _, ok := ids[doc.Ref.ID]; !ok {
				err := txn.Delete(doc.Ref)
				if err != nil {
					return err
				}
				klog.Infof("purged stale firestore document: %s", doc.Ref.Path)
			}
		}
	})
	if err != nil {
		klog.Fatalf("error cleaning %s collection: %v", collection, err)
	}
}

func (s *Manager) cleanWorkspaces(subscriptions map[string]bool) {

	keys := make([]string, 0)
	for subID := range subscriptions {
		keys = append(keys, subID)
	}
	ctx := context.Background()
	iter := s.firestore.Collection(CollectionWorkspace).Documents(ctx)
	defer iter.Stop()
	err := s.firestore.RunTransaction(ctx, func(c context.Context, txn *firestore.Transaction) error {
		for {
			doc, err := iter.Next()
			if err != nil {
				if err == iterator.Done {
					return nil
				}
				return err
			}
			subWorkspace := strings.Split(doc.Ref.ID, "-")
			if len(subWorkspace) > 1 {
				if _, ok := subscriptions[subWorkspace[0]]; !ok {
					err := txn.Delete(doc.Ref)
					if err != nil {
						return err
					}
					klog.Infof("purged stale firestore document: %s", doc.Ref.Path)
				}
			}
		}
	})
	if err != nil {
		klog.Fatalf("error cleaning workspaces collection: %v", err)
	}
}

func (s *Manager) CreateOrUpdateProduct(txn *firestore.Transaction, product *stripe.Product) error {

	state := &pb.Product{
		Active:      product.Active,
		Description: product.Description,
		Name:        product.Name,
		Id:          product.ID,
		Metadata:    product.Metadata,
	}
	if err := s.Serialize(txn, CollectionProducts, state); err != nil {
		return err
	}

	return nil
}

func (s *Manager) CreateOrUpdatePlan(txn *firestore.Transaction, plan *stripe.Plan) error {

	interval := string(plan.Interval)
	scope := pb.PlanScope_enterprize
	if strings.HasSuffix(plan.Product.ID, "-site") {
		scope = pb.PlanScope_public
	}
	state := &pb.Plan{
		Id:       plan.ID,
		Active:   plan.Active,
		Amount:   plan.Amount,
		Currency: pb.Currency_USD.String(),
		Interval: interval,
		Metadata: plan.Metadata,
		Nickname: plan.Nickname,
		Product:  plan.Product.ID,
		Scope:    scope,
	}
	if err := s.Serialize(txn, CollectionPlan, state); err != nil {
		return err
	}
	return nil
}

func (s *Manager) CreateOrUpdateWorkspace(txn *firestore.Transaction, ws *adminpb.Workspace) error {

	if err := s.Serialize(txn, CollectionWorkspace, ws); err != nil {
		return err
	}
	return nil
}

func (s *Manager) CreateOrUpdateSubscription(txn *firestore.Transaction, subscription *stripe.Subscription) error {

	var items []*pb.SubscriptionItem
	if subscription.Items != nil {
		for _, item := range subscription.Items.Data {
			items = append(items, &pb.SubscriptionItem{
				Id:       item.ID,
				Metadata: item.Metadata,
				Plan:     item.Plan.ID,
			})
		}
	}
	state := &pb.Subscription{
		CurrentPeriodStart: subscription.CurrentPeriodStart,
		CurrentPeriodEnd:   subscription.CurrentPeriodEnd,
		Id:                 subscription.ID,
		Name:               subscription.ID,
		Customer:           subscription.Customer.ID,
		Items:              items,
	}
	if subscription.Plan != nil {
		state.Active = subscription.Plan.Active
	}
	if subscription.Discount != nil && subscription.Discount.Coupon != nil {
		state.Coupon = subscription.Discount.Coupon.ID
	}
	if subscription.Customer.Delinquent {
		state.State = pb.SubscriptionState_DELINQUENT
	}
	if subscription.CanceledAt != 0 {
		state.State = pb.SubscriptionState_CANCELLED
	}
	if err := s.Serialize(txn, CollectionSubscription, state); err != nil {
		return err
	}

	return nil
}

func (s *Manager) CreateOrUpdateCustomer(txn *firestore.Transaction, customer *stripe.Customer) error {

	var subscriptions []string
	for _, sub := range customer.Subscriptions.Data {
		subscriptions = append(subscriptions, sub.ID)
	}
	state := &pb.Customer{
		Id:            customer.ID,
		Description:   customer.Description,
		Email:         customer.Email,
		Metadata:      customer.Metadata,
		Name:          customer.Name,
		Phone:         customer.Phone,
		Subscriptions: subscriptions,
		Address: &pb.Address{
			City:       customer.Address.City,
			Country:    customer.Address.Country,
			Line1:      customer.Address.Line1,
			Line2:      customer.Address.Line2,
			PostalCode: customer.Address.PostalCode,
			State:      customer.Address.State,
		},
	}
	if customer.DefaultSource != nil {
		state.DefaultSource = customer.DefaultSource.ID
	}
	if err := s.Serialize(txn, CollectionCustomer, state); err != nil {
		return err
	}

	return nil
}

func (s *Manager) DeleteProduct(txn *firestore.Transaction, product *stripe.Product) error {

	state := &pb.Product{
		Id: product.ID,
	}
	err := s.removeSerialized(txn, CollectionProducts, state)
	if err != nil {
		return err
	}

	return nil
}

func (s *Manager) DeletePlan(txn *firestore.Transaction, plan *stripe.Plan) error {

	state := &pb.Plan{
		Id: plan.ID,
	}
	err := s.removeSerialized(txn, CollectionPlan, state)
	if err != nil {
		return err
	}
	return nil
}

func (s *Manager) DeleteWorkspace(txn *firestore.Transaction, workspace *adminpb.Workspace) error {

	state := &adminpb.Workspace{
		Name: workspace.Name,
	}
	err := s.removeSerializedNamed(txn, CollectionWorkspace, state)
	if err != nil {
		return err
	}
	return nil
}

func (s *Manager) DeleteSubscription(txn *firestore.Transaction, subscription *stripe.Subscription) error {

	state := &pb.Subscription{
		Id:   subscription.ID,
		Name: subscription.ID,
	}
	err := s.removeSerialized(txn, CollectionSubscription, state)
	if err != nil {
		return err
	}
	return nil
}

func (s *Manager) DeleteCustomer(txn *firestore.Transaction, customer *stripe.Customer) error {

	const collection = "customers"

	return nil
}

func (s *Manager) CreateOrUpdate(ctx context.Context, data json.RawMessage, obj interface{}) error {

	if err := s.firestore.RunTransaction(ctx, func(c context.Context, txn *firestore.Transaction) error {
		err := s.createOrUpdate(txn, data, obj)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Manager) createOrUpdate(txn *firestore.Transaction, data json.RawMessage, obj interface{}) error {

	err := json.Unmarshal(data, obj)
	if err != nil {
		return fmt.Errorf("Error parsing webhook JSON for type %T: %v\n", obj, err)
	}

	switch t := obj.(type) {
	case *stripe.Subscription:
		return s.CreateOrUpdateSubscription(txn, t)
	case *stripe.Customer:
		return s.CreateOrUpdateCustomer(txn, t)
	case *stripe.Plan:
		return s.CreateOrUpdatePlan(txn, t)
	case *stripe.Product:
		return s.CreateOrUpdateProduct(txn, t)
	}
	return fmt.Errorf("CreateOrUpdate unhandled type %T", obj)
}

func (s *Manager) Delete(ctx context.Context, data json.RawMessage, obj interface{}) error {

	if err := s.firestore.RunTransaction(ctx, func(c context.Context, txn *firestore.Transaction) error {
		err := s.delete(txn, data, obj)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Manager) delete(txn *firestore.Transaction, data json.RawMessage, stripeObj interface{}) error {

	err := json.Unmarshal(data, stripeObj)
	if err != nil {
		return fmt.Errorf("Error parsing webhook JSON for type %T: %v\n", stripeObj, err)
	}

	switch t := stripeObj.(type) {
	case *stripe.Subscription:
		return s.DeleteSubscription(txn, t)
	case *stripe.Customer:
		return s.DeleteCustomer(txn, t)
	case *stripe.Plan:
		return s.DeletePlan(txn, t)
	case *stripe.Product:
		return s.DeleteProduct(txn, t)
	}
	return fmt.Errorf("Delete unhandled type %T", stripeObj)
}

func defaultClientOptions() []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint("firestore.googleapis.com:443"),
		option.WithGRPCDialOption(grpc.WithDisableServiceConfig()),
		option.WithScopes(firestorev1.DefaultAuthScopes()...),
		option.WithGRPCDialOption(grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(math.MaxInt32))),
	}
}

func mustRegisterAPIS(scheme *runtime.Scheme) {
	// This server must know about the billing and org APIS
	err := orgapi.AddToScheme(scheme)
	if err != nil {
		klog.Fatal(err)
	}
}

func NewManager(firestore *firestore.Client, stripe *client.API, config *restclient.Config) *Manager {

	host := os.Getenv("REDIS_HOST")
	if host == "" {
		klog.Fatal("REDIS_HOST env must be set")
	}

	rc := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	if _, err := rc.Ping(context.Background()).Result(); err != nil {
		klog.Fatalf("failed to ping redis: %v", err)
	}
	locker := redislock.New(rc)

	connPool, err := gtransport.DialPool(context.Background(), defaultClientOptions()...)
	if err != nil {
		klog.Fatal(err)
	}

	scheme := runtime.NewScheme()
	mustRegisterAPIS(scheme)
	crClient, err := crclient.New(config, crclient.Options{
		Scheme: scheme,
	})

	s := &Manager{
		crClient:      crClient,
		firestore:     firestore,
		stripe:        stripe,
		redis:         rc,
		locker:        locker,
		firestoreGRPC: firestorepb.NewFirestoreClient(connPool),
	}
	//FIXME: We should serve and then list
	// Removed to reduce firestore costs
	// s.init()

	return s
}
