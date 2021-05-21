package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	corev1.CoreV1Interface
	KubeconfigPath           string
	Clusters                 map[string]*clientcmdapi.Cluster
	AuthInfos                map[string]*clientcmdapi.AuthInfo
	Contexts                 map[string]*clientcmdapi.Context
	kubeconfigCurrentContext string
	flowRC                   *FlowRC
	restclient               *restclient.Config
}

// Creates a default client for communicating with the Kubernetes cluster
// The client will return an error if unable to load a local .kubeconfig or .flowrc file
func CreateDefaultClient() (*Client, error) {
	var kubeconfigPath string
	var flowRC *FlowRC = nil
	var configOverrides *clientcmd.ConfigOverrides = nil

	localKubeconfigAbsPath, err := filepath.Abs(".kubeconfig")
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(localKubeconfigAbsPath)
	if err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("client: %s is dir", localKubeconfigAbsPath)
		}

		kubeconfigPath = localKubeconfigAbsPath
	} else if os.IsNotExist(err) {
		home := homedir.HomeDir()
		kubeconfigPath = filepath.Join(home, ".kube", "config")

		flowRCAbsPath, err := filepath.Abs(".flowrc")
		flowRCInfo, err := os.Stat(flowRCAbsPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("client: run flow login")
			}

			return nil, err
		}

		if flowRCInfo.IsDir() {
			return nil, fmt.Errorf("client: %s is dir", flowRCAbsPath)
		}

		flowRCBytes, err := ioutil.ReadFile(flowRCAbsPath)
		if err != nil {
			return nil, err
		}

		flowRC = &FlowRC{}
		err = json.Unmarshal(flowRCBytes, flowRC)
		if err != nil {
			return nil, err
		}

		if flowRC.Context == "" {
			return nil, fmt.Errorf("client: .flowrc has no configured context")
		}

		if flowRC.Namespace == "" {
			return nil, fmt.Errorf("client: .flowrc has no configured namespace")
		}

		configOverrides = &clientcmd.ConfigOverrides{CurrentContext: flowRC.Context}
	} else {
		return nil, err
	}

	return CreateClient(kubeconfigPath, flowRC, configOverrides)
}

// Creates a new client for communicating with the Kubernetes cluster
func CreateClient(kubeconfigPath string, flowRC *FlowRC, overrides *clientcmd.ConfigOverrides) (*Client, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		overrides,
	)

	k8sConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	restClientConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	k8sClient, err := k8s.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	var client *Client
	client = new(Client)
	client.KubeconfigPath = kubeconfigPath
	client.Clusters = k8sConfig.Clusters
	client.AuthInfos = k8sConfig.AuthInfos
	client.Contexts = k8sConfig.Contexts
	client.CoreV1Interface = k8sClient.CoreV1()
	client.kubeconfigCurrentContext = k8sConfig.CurrentContext
	client.flowRC = flowRC
	client.restclient = restClientConfig
	return client, nil
}

// Portforward a pod with the given ports
func (c *Client) Forward(podName string, ports []string) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(c.restclient)
	if err != nil {
		return err
	}

	req := c.RESTClient().Get().
		Resource("pods").
		Namespace(c.GetCurrentNamespace()).
		Name(podName).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, req.URL())

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)

	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
		return err
	}

	return nil
}

func (c *Client) Watch(listOptions metav1.ListOptions, f func(*v1.Pod, watch.EventType) error) error {
	pods, err := c.Pods(c.GetCurrentNamespace()).Watch(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	for {
		podData := <-pods.ResultChan()
		podObject := podData.Object
		pod, ok := podObject.(*v1.Pod)
		if !ok {
			continue
		}

		err = f(pod, podData.Type)
		if err != nil {
			return err
		}
	}
}

// Set the current context (does not save to file)
func (c *Client) SetContext(ctx string) error {
	restClientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: c.KubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: ctx,
		},
	).ClientConfig()
	if err != nil {
		return err
	}

	k8sClient, err := k8s.NewForConfig(restClientConfig)
	if err != nil {
		return err
	}

	c.restclient = restClientConfig
	c.CoreV1Interface = k8sClient.CoreV1()

	return nil
}

// Get current context from .kubeconfig or .flowrc
func (c *Client) GetCurrentContext() string {
	if c.flowRC != nil {
		return c.flowRC.Context
	}

	return c.kubeconfigCurrentContext
}

// Get the namespace of the current context from .kubeconfig or .flowrc
func (c *Client) GetCurrentNamespace() string {
	if c.flowRC != nil {
		return c.flowRC.Namespace
	}

	return c.Contexts[c.GetCurrentContext()].Namespace
}

// Get the name of the current cluster from .kubeconfig or .flowrc
func (c *Client) GetCurrentCluster() string {
	return c.Contexts[c.GetCurrentContext()].Cluster
}

type ProxyClient struct {
	config     *restclient.Config
	restclient restclient.Interface
	server     string
	Namespace  string
	Name       string
	Resource   string
}

// Create a proxy allowing you to proxy HTTP requests to a given pod
func (c *Client) Proxy(namespace, resource, name string) *ProxyClient {
	return &ProxyClient{
		config:     c.restclient,
		restclient: c.RESTClient(),
		server:     c.Clusters[c.Contexts[c.GetCurrentContext()].Cluster].Server,
		Namespace:  namespace,
		Name:       name,
		Resource:   resource,
	}
}

func (c *Client) ProxyPod(namespace, podName string) *ProxyClient {
	return &ProxyClient{
		config:     c.restclient,
		restclient: c.RESTClient(),
		server:     c.Clusters[c.Contexts[c.GetCurrentContext()].Cluster].Server,
		Namespace:  namespace,
		Name:       podName,
		Resource:   "pods",
	}
}

func (c *Client) ProxyService(namespace, serviceName string) *ProxyClient {
	return &ProxyClient{
		config:     c.restclient,
		restclient: c.RESTClient(),
		server:     c.Clusters[c.Contexts[c.GetCurrentContext()].Cluster].Server,
		Namespace:  namespace,
		Name:       serviceName,
		Resource:   "services",
	}
}

func setAdditionalRequestHeaders(req *rest.Request, additionalHeaders map[string]string) *rest.Request {
	if additionalHeaders != nil {
		for key, value := range additionalHeaders {
			req.SetHeader(key, value)
		}
	}

	return req
}

func (p *ProxyClient) Get(endpoint string, additionalHeaders map[string]string) restclient.Result {
	req := p.restclient.Get().Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

func (p *ProxyClient) Put(endpoint string, body interface{}, additionalHeaders map[string]string) restclient.Result {
	// FIXME: Throttle should not be forced to nil here
	req := p.restclient.Put().Throttle(nil).Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).Body(body)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

func (p *ProxyClient) Post(endpoint string, body interface{}, additionalHeaders map[string]string) restclient.Result {
	req := p.restclient.Post().Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).Body(body)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

func (p *ProxyClient) Delete(endpoint string, additionalHeaders map[string]string) restclient.Result {
	req := p.restclient.Delete().Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

type extractRT struct {
	http.Header
}

func (rt *extractRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.Header = req.Header
	return nil, nil
}

// Create a new websocket connection to a given pod
func (p *ProxyClient) Websocket(endpoint string) (*websocket.Conn, error) {
	proxyUrl := p.restclient.Get().Resource("pods").Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).URL().String()
	websocketUrl := fmt.Sprintf("wss://%s", strings.TrimPrefix(strings.TrimPrefix(proxyUrl, "https://"), "http://"))

	wsc, err := websocket.NewConfig(websocketUrl, websocketUrl)
	if err != nil {
		return nil, err
	}
	wsc.TlsConfig, err = rest.TLSConfigFor(p.config)
	if err != nil {
		return nil, err
	}

	extract := &extractRT{}
	rt, err := rest.HTTPWrappersForConfig(p.config, extract)
	if err != nil {
		return nil, err
	}

	_, err = rt.RoundTrip(&http.Request{})
	if err != nil {
		return nil, err
	}

	wsc.Header = extract.Header

	wsConn, err := websocket.DialConfig(wsc)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}
