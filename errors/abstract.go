package errors

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"

	apictx "github.com/drud/api-common/context"
)

// Abstract error will log the error however return the message without the error body
func AbstractError(ctx context.Context, code codes.Code, message string, err error) error {
	var buf bytes.Buffer
	if user, err := apictx.UserFromContext(ctx); err == nil {
		buf.WriteString(fmt.Sprintf("UID: %s ", user))
	}
	if procudure, err := apictx.ProcedureFromContext(ctx); err == nil {
		buf.WriteString(fmt.Sprintf("Procedure: %s ", procudure))
	}
	if ns, err := apictx.NamespaceFromContext(ctx); err == nil {
		buf.WriteString(fmt.Sprintf("Namespace: %s ", ns))
	}
	buf.WriteString(fmt.Sprintf("Code: %s Message: %s Error: %v", code.String(), message, err))
	klog.Errorf(buf.String())
	return status.Error(code, message)
}
