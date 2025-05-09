#!/bin/bash
# NRDOT+ Internal Dev-Lab PVC Backup Script
# This script creates backups of persistent volumes used by the Function Blocks

set -e

# Configuration
NAMESPACE=${NAMESPACE:-"nrdot-devlab"}
BACKUP_DIR=${BACKUP_DIR:-"/tmp/nrdot-backups"}
DATE_SUFFIX=$(date +%Y%m%d-%H%M%S)
KUBECONFIG=${KUBECONFIG:-"$HOME/.kube/config"}

# List of PVCs to backup
PVCS=(
  "fb-dp-data"
  "fb-gw-pre-data" 
  "fb-dlq-data"
)

# Create backup directory
mkdir -p "${BACKUP_DIR}"
echo "Creating backups in ${BACKUP_DIR}"

# Function to backup a PVC
backup_pvc() {
  local pvc=$1
  local backup_file="${BACKUP_DIR}/${pvc}-${DATE_SUFFIX}.tar.gz"
  
  echo "Backing up PVC ${pvc} to ${backup_file}..."
  
  # Create a temporary pod to access the PVC
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: backup-${pvc}
  namespace: ${NAMESPACE}
spec:
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ${pvc}
  containers:
    - name: backup
      image: alpine:3.18
      command: ["sleep", "3600"]
      volumeMounts:
        - mountPath: /data
          name: data
  restartPolicy: Never
EOF
  
  # Wait for the pod to be ready
  echo "Waiting for backup pod to be ready..."
  kubectl wait --for=condition=Ready pod/backup-${pvc} -n ${NAMESPACE} --timeout=60s
  
  # Create the tarball
  echo "Creating tarball..."
  kubectl exec backup-${pvc} -n ${NAMESPACE} -- tar -czf /tmp/backup.tar.gz -C /data .
  
  # Copy the tarball to local machine
  echo "Copying backup file..."
  kubectl cp ${NAMESPACE}/backup-${pvc}:/tmp/backup.tar.gz "${backup_file}"
  
  # Clean up the pod
  kubectl delete pod backup-${pvc} -n ${NAMESPACE}
  
  echo "Backup of ${pvc} completed: ${backup_file}"
}

# Backup each PVC
for pvc in "${PVCS[@]}"; do
  backup_pvc "${pvc}"
done

echo "All backups completed successfully!"
echo "Backup files are stored in ${BACKUP_DIR}"
