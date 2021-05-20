package client

type FlowRC struct {
	Context   string `json:"context"`   // Kubernetes cluster context
	Namespace string `json:"namespace"` // Kubernetes cluster namespace
}
