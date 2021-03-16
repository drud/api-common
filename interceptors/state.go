package interceptors

import (
	"context"
	"fmt"
	"os"
	"strings"

	fbauth "firebase.google.com/go/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apictx "github.com/drud/api-common/context"
	apierr "github.com/drud/api-common/errors"
)

/*
This compilation unit sets state carried with over the lifetime of the context
*/

//TODO: This compilation unit can be shared among all API servers
//TODO: Requests can be mutated by an upstream service by something such as an ambassador plugin

func getStatefulContext(parent context.Context, firebaseClient *fbauth.Client, crClient client.Client) (context.Context, error) {

	md, ok := metadata.FromIncomingContext(parent)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "retrieving metadata failed")
	}

	bearer, err := apictx.AuthTokenFromMeta(md)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error retrieving authorization state: %v", err)
	}

	token, err := firebaseClient.VerifyIDToken(context.Background(), bearer)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error validiating token: %v", err.Error())
	}
	// Save state provided by the requests token
	ctx := context.WithValue(parent, apictx.ContextKeyToken{}, token)
	printMetadata("3", ctx)
	ctx = context.WithValue(ctx, apictx.ContextKeyUser{}, token.UID)
	printMetadata("4", ctx)

	// Save the derived workspace for any downstream methods
	ws, err := apictx.WorkspaceFromMeta(md)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to determine workspace for request: %v", err)
	}
	ctx = context.WithValue(ctx, apictx.ContextKeyWorkspace{}, ws)
	printMetadata("5", ctx)
	wsSplit := strings.Split(ws, ".")
	if len(wsSplit) > 1 {
		subscription := wsSplit[0]
		ctx = context.WithValue(ctx, apictx.ContextKeySubscription{}, subscription)
		workspace := wsSplit[1]
		ctx = context.WithValue(ctx, apictx.ContextKeyWorkspace{}, workspace)

		selector := labels.NewSelector()
		displayReqs, err := labels.ParseToRequirements(fmt.Sprintf("ddev.live/displayname==%s", workspace))
		if err != nil {
			return nil, apierr.AbstractError(ctx, codes.Internal, "unable to determine workspace for request", err)
		}

		subscriptionReqs, err := labels.ParseToRequirements(fmt.Sprintf("ddev.live/subscription==%s", subscription))
		if err != nil {
			return nil, apierr.AbstractError(ctx, codes.Internal, "unable to determine workspace for request", err)
		}
		selector = selector.Add(displayReqs...)
		selector = selector.Add(subscriptionReqs...)

		var namespaceList v1.NamespaceList
		if err := crClient.List(ctx, &namespaceList, &client.ListOptions{
			LabelSelector: selector,
		}); err != nil {
			return nil, apierr.AbstractError(ctx, codes.Internal, "an internal error occured retrieving workspaces", err)
		}
		if len(namespaceList.Items) > 1 {
			return nil, status.Errorf(codes.NotFound, "ambiguous workspace for request")
		}
		if len(namespaceList.Items) == 0 {
			return nil, status.Errorf(codes.NotFound, "no valid workspace found for request")
		}
		ctx = context.WithValue(ctx, apictx.ContextKeyNamespace{}, namespaceList.Items[0].Name)
		printMetadata("6", ctx)
	} else {
		ctx = context.WithValue(ctx, apictx.ContextKeyNamespace{}, ws)
		printMetadata("7", ctx)
	}
	return ctx, nil
}

type StreamContextWrapper interface {
	grpc.ServerStream
	SetContext(context.Context)
}

type wrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrapper) Context() context.Context {
	return w.ctx
}

func (w *wrapper) SetContext(ctx context.Context) {
	w.ctx = ctx
}

func newStreamContextWrapper(inner grpc.ServerStream) StreamContextWrapper {
	ctx := inner.Context()
	return &wrapper{
		inner,
		ctx,
	}
}

type StateInterceptors interface {
	StreamingServerInterceptor() grpc.StreamServerInterceptor
	UnaryServerInterceptor() grpc.UnaryServerInterceptor
}

type interceptor struct {
	firebaseClient *fbauth.Client
	crClient       client.Client
}

func NewStateInterceptor(firebaseClient *fbauth.Client, crClient client.Client) StateInterceptors {
	return &interceptor{
		firebaseClient: firebaseClient,
		crClient:       crClient,
	}
}

func (i *interceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Save the procedure as state for logging/analytics
		printMetadata("1", ctx)
		ctx = context.WithValue(ctx, apictx.ContextKeyProcedure{}, info.FullMethod)
		printMetadata("2", ctx)
		ctx, err := getStatefulContext(ctx, i.firebaseClient, i.crClient)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (i *interceptor) StreamingServerInterceptor() grpc.StreamServerInterceptor {

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		w := newStreamContextWrapper(ss)
		ctx := context.WithValue(w.Context(), apictx.ContextKeyProcedure{}, info.FullMethod)
		ctx, err := getStatefulContext(ctx, i.firebaseClient, i.crClient)
		if err != nil {
			return err
		}
		w.SetContext(ctx)
		return handler(srv, w)
	}
}

func printMetadata(id string, ctx context.Context) {
	fmt.Fprintf(os.Stdout, "%s:\n", id)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for k, v := range md {
			fmt.Fprintf(os.Stdout, "\t%s: %v\n", k, v)
		}
	}
	fmt.Fprintf(os.Stdout, "values:\n")
	if iface := ctx.Value(apictx.ContextKeyNamespace{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextValueNamespace: %v\n", iface)
	}
	if iface := ctx.Value(apictx.ContextKeyProcedure{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextKeyProcedure: %v\n", iface)
	}
	if iface := ctx.Value(apictx.ContextKeySubscription{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextKeySubscription: %v\n", iface)
	}
	if iface := ctx.Value(apictx.ContextKeyToken{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextKeyToken: %v\n", iface)
	}
	if iface := ctx.Value(apictx.ContextKeyUser{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextKeyUser: %v\n", iface)
	}
	if iface := ctx.Value(apictx.ContextKeyWorkspace{}); iface != nil {
		fmt.Fprintf(os.Stdout, "\tContextKeyWorkspace: %v\n", iface)
	}
	fmt.Fprintf(os.Stdout, "\n")
}
