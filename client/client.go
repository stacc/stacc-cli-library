package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"

	// IMPORT REQUIRED TO REGISTER OIDC AS AN AUTH PROVIDER
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Client is used to load .kubeconfig files and
// communicate with the Kubernetes API
type Client struct {
	corev1.CoreV1Interface
	KubeconfigPath           string
	Clusters                 map[string]*clientcmdapi.Cluster
	AuthInfos                map[string]*clientcmdapi.AuthInfo
	Contexts                 map[string]*clientcmdapi.Context
	RESTConfig               *restclient.Config
	kubeconfigCurrentContext string
}

const DEFAULT_KUBECONFIG_PATH = ".kubeconfig"

// GetKubeconfigPath returns the path to a kubeconfig file.
// Returns the value of the "STACC_KUBECONFIG" environment variable
// if it exists. Defaults to the absolute path to "<current-directory>/.kubeconfig".
//
// NOTE: This function doesn't check if the file actually exists.
func GetKubeconfigPath() (string, error) {
	if staccKubeconfigEnv, ok := os.LookupEnv("STACC_KUBECONFIG"); ok {
		return staccKubeconfigEnv, nil
	}

	kubeconfigPath, err := filepath.Abs(DEFAULT_KUBECONFIG_PATH)
	if err != nil {
		return "", err
	}

	return kubeconfigPath, nil
}

// CreateDefaultClient creates a default client for communicating with the Kubernetes cluster
// The client will return an error if unable to load a local .kubeconfig file
func CreateDefaultClient() (*Client, error) {
	var configOverrides *clientcmd.ConfigOverrides = nil

	kubeconfigPath, err := GetKubeconfigPath()
	if err != nil {
		return nil, err
	}

	return CreateClient(kubeconfigPath, configOverrides)
}

// CreateClient creates a new client for communicating with the Kubernetes cluster
func CreateClient(kubeconfigPath string, overrides *clientcmd.ConfigOverrides) (*Client, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		overrides,
	)

	k8sConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	// Refresh OIDC tokens
	if ctx, ok := k8sConfig.Contexts[k8sConfig.CurrentContext]; ok {
		if authInfo, ok := k8sConfig.AuthInfos[ctx.AuthInfo]; ok && authInfo.AuthProvider != nil && authInfo.AuthProvider.Name == "oidc" {
			conf := authInfo.AuthProvider.Config
			isTokenValid, err := isTokenValid(conf)
			if err != nil {
				return nil, err
			}

			if refreshToken, ok := conf["refresh-token"]; ok && !isTokenValid {
				oauthConfig := oauth2.Config{
					ClientID:     conf["client-id"],
					ClientSecret: conf["client-secret"],
					Endpoint:     oauth2.Endpoint{TokenURL: fmt.Sprintf("%s/connect/token", conf["idp-issuer-url"])},
				}

				token, err := oauthConfig.TokenSource(context.TODO(), &oauth2.Token{RefreshToken: refreshToken}).Token()
				if err != nil {
					var e *oauth2.RetrieveError
					if errors.As(err, &e) {
						if strings.Contains(err.Error(), "invalid_grant") {
							return nil, fmt.Errorf("failed to refresh credentials for environment, please run 'stacc connect'")
						}
					}

					return nil, err
				}

				newConfig := make(map[string]string)
				for k, v := range conf {
					newConfig[k] = v
				}

				if token.RefreshToken != "" {
					newConfig["refresh-token"] = token.RefreshToken
				}

				newConfig["id-token"] = token.Extra("id_token").(string)
				persister := clientcmd.PersisterForUser(clientConfig.ConfigAccess(), ctx.AuthInfo)
				if err = persister.Persist(newConfig); err != nil {
					return nil, err
				}

				authInfo.AuthProvider.Config = newConfig
			}
		}
	}

	restClientConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	k8sClient, err := k8s.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	var client *Client = new(Client)
	client.KubeconfigPath = kubeconfigPath
	client.Clusters = k8sConfig.Clusters
	client.AuthInfos = k8sConfig.AuthInfos
	client.Contexts = k8sConfig.Contexts
	client.CoreV1Interface = k8sClient.CoreV1()
	client.kubeconfigCurrentContext = k8sConfig.CurrentContext
	client.RESTConfig = restClientConfig
	return client, nil
}

// Forward port forwards a pod with the given ports
func (c *Client) Forward(podName string, ports []string, out, errOut io.Writer) error {
	if out == nil {
		out = os.Stdout
	}

	if errOut == nil {
		errOut = os.Stderr
	}

	roundTripper, upgrader, err := spdy.RoundTripperFor(c.RESTConfig)
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

	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
		return err
	}

	return nil
}

// Watch returns a watch.Interface that watches the requested pods.
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

// SetContext sets the current context (does not save to file)
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

	c.RESTConfig = restClientConfig
	c.CoreV1Interface = k8sClient.CoreV1()

	return nil
}

// GetCurrentContext gets current context from .kubeconfig
func (c *Client) GetCurrentContext() string {
	return c.kubeconfigCurrentContext
}

// GetCurrentNamespace gets the namespace of the current context from .kubeconfig
func (c *Client) GetCurrentNamespace() string {
	return c.Contexts[c.GetCurrentContext()].Namespace
}

// GetCurrentCluster gets the name of the current cluster from .kubeconfig
func (c *Client) GetCurrentCluster() string {
	return c.Contexts[c.GetCurrentContext()].Cluster
}

func isTokenValid(conf map[string]string) (bool, error) {
	idToken, ok := conf["id-token"]
	if !ok {
		return false, nil
	}

	split := strings.Split(idToken, ".")

	data, err := base64.RawURLEncoding.DecodeString(split[1])
	if err != nil {
		return false, err
	}

	var dataStruct struct {
		Expiry int64 `json:"exp"`
	}
	if err := json.Unmarshal(data, &dataStruct); err != nil {
		return false, err
	}

	return time.Now().Add(10 * time.Second).Before(time.Unix(dataStruct.Expiry, 0)), nil
}
