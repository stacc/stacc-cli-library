package client

import (
	hostv1 "bitbucket.org/staccas/database-controller/apis/solution/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type HostV1Alpha1Interface interface {
	Hosts(namespace string) HostInterface
}

type HostV1Alpha1Client struct {
	client rest.Interface
}

func NewForConfig(c *rest.Config) (*HostV1Alpha1Client, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: hostv1.GroupVersion.Group, Version: hostv1.GroupVersion.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &HostV1Alpha1Client{client: client}, nil
}

func (c *HostV1Alpha1Client) Hosts(namespace string) *hostClient {
	return &hostClient{
		client: c.client,
		ns:     namespace,
	}
}
