package context

import (
	"context"
	"strings"

	fbauth "firebase.google.com/go/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	//AuthHeader the header we expect to contain our firebase bearer token
	AuthHeader = "x-auth-token"
	//WorkspaceHeader shall indicate the workspace for the request
	WorkspaceHeader = "x-ddev-workspace"

	// Request Context Values
	ContextKeyWorkspace    = "ddev-workspace"
	ContextKeyNamespace    = "ddev-namespace"
	ContextKeySubscription = "ddev-subscription"
	ContextKeyUser         = "ddev-user"
	ContextKeyToken        = "ddev-token"
	ContextKeyProcedure    = "ddev-procedure"
)

func WorkspaceFromMeta(meta metadata.MD) (string, error) {
	getElement := func(elem []string) (string, error) {
		if len(elem) == 0 || elem[0] == "" {
			return "", status.Errorf(codes.InvalidArgument, "no workspace details supplied")
		}
		return strings.TrimSpace(elem[0]), nil
	}

	if elem, ok := meta[WorkspaceHeader]; ok {
		return getElement(elem)
	}

	return "", status.Errorf(codes.InvalidArgument, "no workspace details supplied")
}

func AuthTokenFromMeta(meta metadata.MD) (string, error) {
	getElement := func(elem []string) (string, error) {
		if len(elem) == 0 {
			return "", status.Errorf(codes.InvalidArgument, "no auth details supplied")
		}
		return strings.TrimSpace(elem[0]), nil
	}

	if elem, ok := meta[AuthHeader]; ok {
		return getElement(elem)
	}
	// Deprecate
	if authorization, ok := meta["authorization"]; ok {
		return getElement(authorization)
	}

	return "", status.Errorf(codes.InvalidArgument, "no auth details supplied")
}

func NamespaceFromContext(ctx context.Context) (string, error) {
	iface := ctx.Value(ContextKeyNamespace)
	if iface != nil {
		if ws, ok := iface.(string); ok {
			return ws, nil
		}
	}
	// message for the user
	return "", status.Error(codes.NotFound, "unable to determine workspace for request")
}

func WorkspaceFromContext(ctx context.Context) (string, error) {
	iface := ctx.Value(ContextKeyWorkspace)
	if iface != nil {
		if ws, ok := iface.(string); ok {
			return ws, nil
		}
	}
	return "", status.Error(codes.NotFound, "unable to determine workspace for request")
}

func SubscriptionFromContext(ctx context.Context) (string, error) {
	iface := ctx.Value(ContextKeySubscription)
	if iface != nil {
		if sub, ok := iface.(string); ok {
			return sub, nil
		}
	}
	return "", status.Error(codes.NotFound, "unable to determine subscription for request")
}

func UserFromContext(ctx context.Context) (string, error) {
	iface := ctx.Value(ContextKeyUser)
	if iface != nil {
		if user, ok := iface.(string); ok {
			return user, nil
		}
	}
	return "", status.Error(codes.NotFound, "unable to determine user for request")
}

func ProcedureFromContext(ctx context.Context) (string, error) {
	iface := ctx.Value(ContextKeyProcedure)
	if iface != nil {
		if procedure, ok := iface.(string); ok {
			return procedure, nil
		}
	}
	return "", status.Error(codes.NotFound, "unable to determine procedure for request")
}

func AuthTokenFromContext(ctx context.Context) (*fbauth.Token, error) {
	iface := ctx.Value(ContextKeyToken)
	if iface != nil {
		if token, ok := iface.(*fbauth.Token); ok {
			return token, nil
		}
	}
	return nil, status.Error(codes.NotFound, "unable to determine token for request")
}
