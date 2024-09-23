#!/bin/bash -E

SCRIPT_DIR="$(dirname "${0}")"
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

OPERATOR_RELEASE=${OPERATOR_RELEASE:-"latest"}
OPERATOR_URL="https://raw.githubusercontent.com/ceph/ceph-csi-operator/releases/download/${OPERATOR_RELEASE}"

# operator deployment files
OPERATOR_CRD="${OPERATOR_URL}/deploy/multifile/crd.yaml"
OPERATOR="${OPERAOTR_URL}/deploy/operator.yaml"
OPERATOR_CSI_RBAC="${OPERATOR_URL}/deploy/multifile/csi-rbac.yaml"

TEMP_DIR="$(mktemp -d)"

function deploy_operator() {
    # local namespace=$1
    # if [ -z "${namespace}" ]; then
    #     namespace="ceph-csi"
    # fi

    create_or_delete_resources "create"
}

function cleanup() {
    create_or_delete_resources "delete"
}

function create_or_delete_resources() {
    local operation=$1

    # if [ "${operation}" == "create"]; then
    #     # TODO: replace namespace, or other values
    # fi

    kubectl_retry "${operation}" -f "${OPERATOR_CRD}"
    kubectl_retry "${operation}" -f "${OPERATOR}"
    kubectl_retry "${operation}" -f "${OPERATOR_CSI_RBAC}"

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