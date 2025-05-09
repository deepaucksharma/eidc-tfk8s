#!/bin/bash
# NRDOT+ Internal Dev-Lab PVC Restore Script
# This script restores backups to persistent volumes used by the Function Blocks

set -e

# Configuration
NAMESPACE=${NAMESPACE:-"nrdot-devlab"}
KUBECONFIG=${KUBECONFIG:-"$HOME/.kube/config"}

# Check for required arguments
if [ $# -lt 2 ]; then
  echo "Usage: $0 <pvc-name> <backup-file>"
  echo "Example: $0 fb-dp-data /tmp/nrdot-backups/fb-dp-data-20250510-120000.tar.gz"
  exit 1
fi

PVC=$1
BACKUP_FILE=$2

if [ ! -f "${BACKUP_FILE}" ]; then
  echo "Error: Backup file ${BACKUP_FILE} not found!"
  exit 1
fi

echo "Restoring ${BACKUP_FILE} to PVC ${PVC}..."

# Verify the namespace and PVC exist
kubectl get namespace ${NAMESPACE} > /dev/null || {
  echo "Error: Namespace ${NAMESPACE} not found!"
  exit 1
}

kubectl get pvc ${PVC} -n ${NAMESPACE} > /dev/null || {
  echo "Error: PVC ${PVC} not found in namespace ${NAMESPACE}!"
  exit 1
}

# Find deployments using this PVC
echo "Finding deployments using PVC ${PVC}..."
DEPLOYMENTS=$(kubectl get deployment -n ${NAMESPACE} -o json | jq -r '.items[] | select(.spec.template.spec.volumes[]?.persistentVolumeClaim.claimName == "'${PVC}'") | .metadata.name')

if [ -n "${DEPLOYMENTS}" ]; then
  echo "Found deployments using the PVC: ${DEPLOYMENTS}"
  echo "Scaling down deployments..."
  
  for deployment in ${DEPLOYMENTS}; do
    kubectl scale deployment ${deployment} -n ${NAMESPACE} --replicas=0
    echo "Scaled down ${deployment}"
  done
  
  # Wait for pods to terminate
  echo "Waiting for pods to terminate..."
  sleep 10
fi

# Create a restore pod
echo "Creating restore pod..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: restore-${PVC}
  namespace: ${NAMESPACE}
spec:
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ${PVC}
  containers:
    - name: restore
      image: alpine:3.18
      command: ["sleep", "3600"]
      volumeMounts:
        - mountPath: /data
          name: data
  restartPolicy: Never
EOF

# Wait for the pod to be ready
echo "Waiting for restore pod to be ready..."
kubectl wait --for=condition=Ready pod/restore-${PVC} -n ${NAMESPACE} --timeout=60s

# Copy the backup file to the pod
echo "Copying backup file to pod..."
kubectl cp "${BACKUP_FILE}" ${NAMESPACE}/restore-${PVC}:/tmp/backup.tar.gz

# Perform the restore
echo "Restoring data..."
kubectl exec restore-${PVC} -n ${NAMESPACE} -- sh -c "rm -rf /data/* && tar -xzf /tmp/backup.tar.gz -C /data"

# Clean up the pod
kubectl delete pod restore-${PVC} -n ${NAMESPACE}

# Scale deployments back up if any were scaled down
if [ -n "${DEPLOYMENTS}" ]; then
  echo "Scaling deployments back up..."
  
  for deployment in ${DEPLOYMENTS}; do
    # Get the original replica count
    REPLICAS=$(kubectl get deployment ${deployment} -n ${NAMESPACE} -o jsonpath='{.spec.replicas}')
    if [ -z "${REPLICAS}" ] || [ "${REPLICAS}" -eq "0" ]; then
      REPLICAS=1  # Default to 1 if not set or was 0
    fi
    
    kubectl scale deployment ${deployment} -n ${NAMESPACE} --replicas=${REPLICAS}
    echo "Scaled up ${deployment} to ${REPLICAS} replicas"
  done
fi

echo "Restore of ${PVC} completed successfully!"
