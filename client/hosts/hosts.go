package hosts

import (
	"context"
	"time"

	hostv1 "bitbucket.org/staccas/database-controller/apis/solution/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type HostsGetter interface {
	Hosts(namespace string) HostInterface
}

type HostInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*hostv1.HostList, error)
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*hostv1.Host, error)
	Create(ctx context.Context, host *hostv1.Host, opts metav1.CreateOptions) (*hostv1.Host, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Update(ctx context.Context, host *hostv1.Host, opts metav1.UpdateOptions) (*hostv1.Host, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type hostClient struct {
	client rest.Interface
	ns     string
}

func (c *hostClient) List(ctx context.Context, opts metav1.ListOptions) (*hostv1.HostList, error) {
	result := hostv1.HostList{}
	err := c.client.
		Get().
		Namespace(c.ns).
		Resource("hosts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *hostClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*hostv1.Host, error) {
	result := hostv1.Host{}
	err := c.client.
		Get().
		Namespace(c.ns).
		Resource("hosts").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *hostClient) Create(ctx context.Context, host *hostv1.Host, opts metav1.CreateOptions) (*hostv1.Host, error) {
	result := hostv1.Host{}
	err := c.client.
		Post().
		Namespace(c.ns).
		Resource("hosts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(host).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *hostClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.
		Get().
		Namespace(c.ns).
		Resource("hosts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

func (c *hostClient) Update(ctx context.Context, host *hostv1.Host, opts metav1.UpdateOptions) (*hostv1.Host, error) {
	result := hostv1.Host{}
	err := c.client.
		Put().
		Namespace(c.ns).
		Resource("hosts").
		Name(host.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(host).
		Do(ctx).
		Into(&result)

	return &result, err
}

func (c *hostClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("hosts").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}
