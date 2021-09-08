package client

// FlowRC refers to the structure of a .flowrc file, it contains
// context and namespace information for a Kubernetes cluster
type FlowRC struct {
	Context   string `json:"context"`   // Kubernetes cluster context
	Namespace string `json:"namespace"` // Kubernetes cluster namespace
}
