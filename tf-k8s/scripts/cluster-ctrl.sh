#!/bin/bash
# tf-k8s/scripts/cluster-ctrl.sh - Kubernetes cluster setup and teardown script

set -e

ACTION=$1
CLUSTER_TYPE=$2

function setup_kind() {
    echo "Setting up Kind cluster for TF-K8s..."
    cat > kind-config.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF
    
    kind create cluster --config=kind-config.yaml
    
    # Install metrics-server for resource monitoring
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    echo "Kind cluster ready for TF-K8s validation."
}

function setup_k3d_duo() {
    echo "Setting up K3d dual-node cluster for TF-K8s..."
    
    k3d cluster create tf-k8s-duo \
        --agents 2 \
        --k3s-arg '--no-deploy=traefik@server:*' \
        --port "80:80@loadbalancer" \
        --port "443:443@loadbalancer"
    
    # Install metrics-server for resource monitoring
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    echo "K3d dual-node cluster ready for TF-K8s validation."
}

function teardown_kind() {
    echo "Tearing down Kind cluster..."
    kind delete cluster
    echo "Kind cluster deleted."
}

function teardown_k3d_duo() {
    echo "Tearing down K3d dual-node cluster..."
    k3d cluster delete tf-k8s-duo
    echo "K3d dual-node cluster deleted."
}

# Main execution
case $ACTION in
    setup)
        case $CLUSTER_TYPE in
            kind)
                setup_kind
                ;;
            k3d-duo)
                setup_k3d_duo
                ;;
            *)
                echo "Unknown cluster type: $CLUSTER_TYPE"
                echo "Supported types: kind, k3d-duo"
                exit 1
                ;;
        esac
        ;;
    teardown)
        case $CLUSTER_TYPE in
            kind)
                teardown_kind
                ;;
            k3d-duo)
                teardown_k3d_duo
                ;;
            *)
                echo "Unknown cluster type: $CLUSTER_TYPE"
                echo "Supported types: kind, k3d-duo"
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Unknown action: $ACTION"
        echo "Usage: $0 [setup|teardown] [kind|k3d-duo]"
        exit 1
        ;;
esac
