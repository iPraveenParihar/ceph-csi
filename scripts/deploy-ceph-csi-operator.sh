#!/bin/bash -E

SCRIPT_DIR="$(dirname "${0}")"
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

OPERATOR_VERSION=${OPERATOR_VERSION:-"main"}
OPERATOR_URL="https://raw.githubusercontent.com/ceph/ceph-csi-operator/${OPERATOR_VERSION}"

# operator deployment files
OPERATOR_CRD="${OPERATOR_URL}/deploy/multifile/crd.yaml"
OPERATOR="${OPERATOR_URL}/deploy/multifile/operator.yaml"
OPERATOR_CSI_RBAC="${OPERATOR_URL}/deploy/multifile/csi-rbac.yaml"
OPERATOR_INSTALL="${OPERATOR_URL}/deploy/all-in-one/install.yaml"

OPERATOR_NAMESPACE="ceph-csi-operator-system"
IMAGESET_CONFIGMAP_NAME="ceph-csi-imageset"
ENCRYPTION_CONFIGMAP_NAME="ceph-csi-encryption-kms-config"

# csi drivers
RBD_DRIVER_NAME="rbd.csi.ceph.com"
CEPHFS_DRIVER_NAME="cephfs.csi.ceph.com"
NFS_DRIVER_NAME="nfs.csi.ceph.com"

TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

function create_or_delete_imageset_configmap() {
    local operation=$1
    temp_file=$(mktemp "${TEMP_DIR}/imageset-configmap.XXXXXX.yaml")
    cat <<EOF > "${temp_file}"
apiVersion: v1
kind: ConfigMap
metadata:
    name: ${IMAGESET_CONFIGMAP_NAME}
    namespace: ${OPERATOR_NAMESPACE}
data:
    "plugin": "quay.io/cephcsi/cephcsi:canary"  # test image
EOF
    kubectl_retry $operation -f "${temp_file}"
}

function create_or_delete_encryption_configmap() {
    local operation=$1
    temp_file=$(mktemp "${TEMP_DIR}/encryption-configmap.XXXXXX.yaml")
    cat <<EOF > "${temp_file}"
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: ${OPERATOR_NAMESPACE}
  name: ${ENCRYPTION_CONFIGMAP_NAME}
data:
    config.json: ""
EOF
    kubectl_retry $operation -f "${temp_file}"
}

function create_or_delete_operator_config() {
    # TODO: encryption config
    local operation=$1
    create_or_delete_imageset_configmap $operation
    create_or_delete_encryption_configmap $operation

    temp_file=$(mktemp "${TEMP_DIR}/operatorconfig.XXXXXX.yaml")
    cat <<EOF > "${temp_file}"
apiVersion: csi.ceph.io/v1alpha1
kind: OperatorConfig
metadata:
    name: ceph-csi-operator-config
    namespace: ${OPERATOR_NAMESPACE}
spec:
    driverSpecDefaults:
        log:
            verbosity: 5 # csi pods log level
        imageSet:
            name: ${IMAGESET_CONFIGMAP_NAME}
        encryption:
            configMapName:
                name: ${ENCRYPTION_CONFIGMAP_NAME}
    log:
        verbosity: 3 # operator log level
EOF
    kubectl_retry $operation -f "${temp_file}"
}

function deploy_or_delete_driver() {
    local operator=$1
    local driver_name=$2
    temp_file=$(mktemp "${TEMP_DIR}/${driver_name}.XXXXXX.yaml")
    cat <<EOF > "${temp_file}"
apiVersion: csi.ceph.io/v1alpha1
kind: Driver
metadata:
  name: ${driver_name}
  namespace: ${OPERATOR_NAMESPACE}
EOF
    kubectl_retry $operator -f "${temp_file}"
}

function deploy_operator() {
    kubectl_retry create -f "${OPERATOR_INSTALL}"
    create_or_delete_operator_config "create"
    deploy_or_delete_driver "create" $RBD_DRIVER_NAME
    deploy_or_delete_driver "create" $CEPHFS_DRIVER_NAME
    deploy_or_delete_driver "create" $NFS_DRIVER_NAME
}

function cleanup() {
    deploy_or_delete_driver "delete" $RBD_DRIVER_NAME
    deploy_or_delete_driver "delete" $CEPHFS_DRIVER_NAME
    deploy_or_delete_driver "delete" $NFS_DRIVER_NAME
    create_or_delete_operator_config "delete"
    kubectl_retry delete -f "${OPERATOR_INSTALL}"
}

case "${1:-}" in
    deploy)
        deploy_operator
        ;;
    cleanup)
        cleanup
        ;;
    *)
        echo "Usage:" >&2
        echo "  $0 deploy" >&2
        echo "  $0 cleanup" >&2
        exit 1
        ;;
esac