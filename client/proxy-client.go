package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
	"k8s.io/client-go/rest"
)

// ProxyClient is used to send proxy HTTP requests to a pod running in the cluster
type ProxyClient struct {
	config     *rest.Config
	restclient rest.Interface
	server     string
	Namespace  string
	Name       string
	Resource   string
}

// Proxy creates a ProxyClient allowing you to proxy HTTP requests to a resource running in the cluster
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

// ProxyPod creates a ProxyClient allowing you to proxy HTTP requests to a pod running in the cluster
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

// ProxyService creates a ProxyClient allowing you to proxy HTTP requests to a service running in the cluster
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

// Get sends a proxied HTTP GET request to a specific resource running in the cluster
func (p *ProxyClient) Get(endpoint string, additionalHeaders map[string]string) rest.Result {
	req := p.restclient.Get().Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

// Put sends a proxied HTTP PUT request to a specific resource running in the cluster
func (p *ProxyClient) Put(endpoint string, body interface{}, additionalHeaders map[string]string) rest.Result {
	// FIXME: Throttle should not be forced to nil here
	req := p.restclient.Put().Throttle(nil).Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).Body(body)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

// Post sends a proxied HTTP POST request to a specific resource running in the cluster
func (p *ProxyClient) Post(endpoint string, body interface{}, additionalHeaders map[string]string) rest.Result {
	req := p.restclient.Post().Resource(p.Resource).Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).Body(body)
	setAdditionalRequestHeaders(req, additionalHeaders)

	return req.Do(context.TODO())
}

// Delete sends a proxied HTTP DELETE request to a specific resource running in the cluster
func (p *ProxyClient) Delete(endpoint string, additionalHeaders map[string]string) rest.Result {
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

// Websocket creates a new websocket connection to a given pod
func (p *ProxyClient) Websocket(endpoint string) (*websocket.Conn, error) {
	proxyURL := p.restclient.Get().Resource("pods").Namespace(p.Namespace).Name(p.Name).SubResource("proxy").Suffix(endpoint).URL().String()
	websocketURL := fmt.Sprintf("wss://%s", strings.TrimPrefix(strings.TrimPrefix(proxyURL, "https://"), "http://"))

	wsc, err := websocket.NewConfig(websocketURL, websocketURL)
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
