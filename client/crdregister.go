package client

import (
	hostv1 "bitbucket.org/staccas/database-controller/apis/solution/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(hostv1.GroupVersion,
		&hostv1.Host{},
		&hostv1.HostList{},
	)

	metav1.AddToGroupVersion(scheme, hostv1.GroupVersion)
	return nil
}
