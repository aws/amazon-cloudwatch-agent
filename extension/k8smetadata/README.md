# Kubernetes Metadata

The Kubernetes Metadata utilizes a Kubernetes client to start an informer, which queries the Kubernetes API for EndpointSlices. The EndpointSlices are transformed to reduce storage and periodically updated.

> Kubernetes' EndpointSlice API provides a way to track network endpoints within a Kubernetes cluster. (https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)

These network endpoints expose relevant Kubernetes metadata for service-exposed applications.

Pod IP → {Workload, Namespace, Node} mappings are stored.
- Workload: This is the application's name.
- Namespace: This is the Kubernetes namespace the application is in.
- Node: This is the Kubernetes node the application is in.

