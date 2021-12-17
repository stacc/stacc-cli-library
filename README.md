# stacc-cli-library

A public library consisting of Go packages used by the Stacc CLI and it's plugins to communicate with Kubernetes

## Install

Make sure go mod is being used and the environment variable `GO111MODULE` is set to `on` (auto will also work if the current working directory is outside of the GOPATH directory)

Then install with `$ go get github.com/stacc/stacc-cli-library`

## Examples

```
// Create a default client for communicating with the Kubernetes cluster by loading local .kubeconfig files
client, err := CreateDefaultClient()
if err != nil {
  log.Fatal(err.Error())
}

// Log the current namespace
namespace := client.GetCurrentNamespace()
log.Println(namespace)

// List all pods in the current namespace
pods, err := client.Pods(namespace).List(context.TODO(), v1.ListOptions{})
if err != nil {
  log.Fatal(err)
}
log.Println(pods)

// Find all stacc-cloud pods in the current namespace
staccCloudPods, err := client.Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=stacc-cloud"})
if err != nil {
  log.Fatal(err)
}

// Proxy a healthz HTTP request to stacc-cloud
staccCloudRes, err := client.Proxy(namespace, staccCloudPods.Items[0].Name).Get("/healthz", nil).Raw()
if err != nil {
  log.Fatal(err)
}
log.Println(staccCloudRes)
```
