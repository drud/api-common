package utils

import (
	"fmt"

	"github.com/drud/api-common/metadata"
	corev1 "k8s.io/api/core/v1"
)

func WorkspaceNameFromNamespace(ns *corev1.Namespace) string {
	var fqwn string

	if name, ok := ns.Labels[metadata.LabelKeyWorkspace]; ok {
		fqwn = name
	}
	if stub, ok := ns.Labels[metadata.LabelKeySubscriptionStub]; ok {
		fqwn = fmt.Sprintf("%s.%s", stub, fqwn)
	}

	if fqwn == "" {
		return ns.Name
	}

	return fqwn
}
