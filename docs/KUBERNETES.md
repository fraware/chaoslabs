# Kubernetes Deployment Guide for ChaosLabs

See also the [documentation index](README.md) and [ARCHITECTURE.md](ARCHITECTURE.md).

This document outlines the steps required to deploy ChaosLabs on a Kubernetes cluster. It covers setting up namespaces, deploying the controller, agent, and dashboard components, as well as verifying and troubleshooting your deployment.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Overview of Components](#overview-of-components)
3. [Deployment Instructions](#deployment-instructions)
   - [1. Create a Namespace](#1-create-a-namespace)
   - [2. Deploy the Controller](#2-deploy-the-controller)
   - [3. Deploy the Agent](#3-deploy-the-agent)
   - [4. Deploy the Dashboard](#4-deploy-the-dashboard)
   - [5. (Optional) Deploy Horizontal Pod Autoscaler](#5-optional-deploy-horizontal-pod-autoscaler)
4. [Exposing Services](#exposing-services)
5. [Verification & Troubleshooting](#verification--troubleshooting)
6. [Observability & Scaling](#observability--scaling)
7. [Cleanup](#cleanup)
8. [Additional Resources](#additional-resources)

---

## Prerequisites

- A working Kubernetes cluster (local or cloud-based, e.g., Minikube, Docker Desktop Kubernetes, GKE, EKS, etc.)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed and configured for your cluster.
- Access to your container images. Ensure that your images (for the controller, agent, and dashboard) are pushed to a registry accessible by your cluster.
- (Optional) Prometheus, Grafana, and an **OTLP**-compatible collector or tracing backend (OpenTelemetry Collector, Jaeger v2, Tempo, etc.).

---

## Overview of Components

**Controller:**
- Receives experiment requests via HTTP endpoints.
- Schedules experiments (immediately or at a future time) and dispatches them to one or more agents.
- Exposes Prometheus metrics and OpenTelemetry traces via **OTLP/HTTP** (`OTEL_EXPORTER_OTLP_ENDPOINT` on the Deployment).

**Agent:**
- Listens for fault injection commands on its `/inject` endpoint.
- Implements various fault injection techniques (network latency/loss via `tc`, CPU/memory stress using `stress-ng`, process kill).
- Exposes Prometheus metrics and OTLP traces (same env vars as the controller).

**Dashboard:**
- Provides a web interface for monitoring experiments in real time.
- Visualizes metrics using Grafana.

---

## Deployment Instructions

### 1. Create a Namespace

It's a good practice to deploy ChaosLabs in its own namespace.

```bash
kubectl create namespace chaoslab
```

### 2. Deploy the Controller

Apply the controller deployment manifest:
```bash
kubectl apply -f infrastructure/k8s/controller-deployment.yaml -n chaoslab
```
This YAML file contains the necessary configuration and Prometheus annotations (e.g., `prometheus.io/scrape: "true"`) to allow for metrics scraping.

### 3. Deploy the Agent
Apply the agent deployment manifest:
```bash
kubectl apply -f infrastructure/k8s/agent-deployment.yaml -n chaoslab
```
The agent is configured to run with multiple replicas (for horizontal scaling) and includes necessary privileges for fault injection commands.

### 4. Deploy the Dashboard
Apply the dashboard deployment manifest:
```bash
kubectl apply -f infrastructure/k8s/dashboard-deployment.yaml -n chaoslab
```
This deploys the web UI used to monitor experiments in real time.

### 5. (Optional) Deploy Horizontal Pod Autoscaler
To simulate scaling under load, you can deploy an HPA for the agent:
```bash
kubectl apply -f infrastructure/k8s/agent-hpa.yaml -n chaoslab
```
This resource will automatically adjust the number of agent replicas based on CPU utilization (or other configured metrics).

## Exposing Services
For local testing or external access, you can expose services using `kubectl port-forward`:

- **Controller:**
```bash
kubectl port-forward deployment/chaos-controller 8080:8080 -n chaoslab
```

- **Dashboard:**
```bash
kubectl port-forward deployment/chaos-dashboard 5000:5000 -n chaoslab
```
Alternatively, you can create Kubernetes Service objects (LoadBalancer or NodePort) if you need external access.

## Verification & Troubleshooting
### 1. Verify Deployments
Check the status of your deployments:
```bash
kubectl get deployments -n chaoslab
kubectl get pods -n chaoslab
```
Ensure all pods are in the `Running` state.


### 2. Inspect Logs
If any pod is not running as expected, check its logs:
```bash
kubectl logs <pod-name> -n chaoslab
```
For example, to check the controller logs:
```bash
kubectl logs deployment/chaos-controller -n chaoslab
```

### 3. Verify Metrics
Access the `/metrics` endpoints for the controller and agent (via port-forward or Service) to ensure Prometheus metrics are exposed:
```bash
curl http://localhost:8080/metrics
curl http://localhost:9090/metrics
```

### 4. Troubleshooting Common Issues
- **Image Pull Errors:**
Ensure your images are correctly tagged and accessible from your container registry.
- **Fault Injection Failures:**
Verify that the agent pods are running in privileged mode if required (check your deployment YAML).
- **Service Connectivity:**
Confirm that the controller can reach the agent endpoints (verify the `AGENT_ENDPOINTS` environment variable in the controller).
For more detailed troubleshooting, refer to the TROUBLESHOOTING.md document.

## Observability & Scaling
- **Prometheus & Grafana:**
With the annotations in place, Prometheus should automatically scrape metrics from the controller and agent. Grafana dashboards (provided in the repository) can be imported to visualize these metrics.
- **Distributed tracing:**
Both components export traces with **OTLP/HTTP**. Point `OTEL_EXPORTER_OTLP_ENDPOINT` at your OpenTelemetry Collector or compatible backend (see sample env in `infrastructure/k8s/*-deployment.yaml`).
- **Scaling:**
The Horizontal Pod Autoscaler for the agent will help you test scalability under load. Monitor scaling behavior using:
```bash
kubectl get hpa -n chaoslab
```

## Cleanup
To remove all ChaosLabs resources from your cluster:
```bash
kubectl delete namespace chaoslab
```

## Additional Resources
- [Kubernetes Official Documentation](https://kubernetes.io/docs/home/)
- [Prometheus Documentation](https://prometheus.io/docs/introduction/overview/)
- [Grafana Documentation](https://grafana.com/docs/)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)

For questions or issues, open a ticket on [github.com/fraware/chaoslabs/issues](https://github.com/fraware/chaoslabs/issues).
