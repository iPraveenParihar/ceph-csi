#!/bin/bash -e

# This script can be used to install/delete snapshotcontroller and snapshot CRD

SCRIPT_DIR="$(dirname "${0}")"

# shellcheck source=build.env
source "${SCRIPT_DIR}/../build.env"

# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

SNAPSHOT_VERSION=${SNAPSHOT_VERSION:-"v5.0.1"}

TEMP_DIR="$(mktemp -d)"
SNAPSHOTTER_URL="https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/${SNAPSHOT_VERSION}"

# controller
SNAPSHOT_RBAC="${SNAPSHOTTER_URL}/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml"
SNAPSHOT_CONTROLLER="${SNAPSHOTTER_URL}/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml"

# snapshot CRD
SNAPSHOTCLASS="${SNAPSHOTTER_URL}/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml"
VOLUME_SNAPSHOT_CONTENT="${SNAPSHOTTER_URL}/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml"
VOLUME_SNAPSHOT="${SNAPSHOTTER_URL}/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml"

# volumegroupsnapshot CRD
VOLUME_GROUP_SNAPSHOTCLASS="${SNAPSHOTTER_URL}/client/config/crd/groupsnapshot.storage.k8s.io_volumegroupsnapshotclasses.yaml"
VOLUME_GROUP_SNAPSHOT_CONTENT="${SNAPSHOTTER_URL}/client/config/crd/groupsnapshot.storage.k8s.io_volumegroupsnapshotcontents.yaml"
VOLUME_GROUP_SNAPSHOT="${SNAPSHOTTER_URL}/client/config/crd/groupsnapshot.storage.k8s.io_volumegroupsnapshots.yaml"

# snapshot metadata
SNAPSHOT_METADATA_URL="https://raw.githubusercontent.com/kubernetes-csi/external-snapshot-metadata/refs/heads/main"
SNAPSHOT_METADATA_SERVICE="${SNAPSHOT_METADATA_URL}/client/config/crd/cbt.storage.k8s.io_snapshotmetadataservices.yaml"

function install_snapshot_controller() {
    local namespace=$1
    if [ -z "${namespace}" ]; then
        namespace="kube-system"
    fi

    create_or_delete_resource "create" "${namespace}"

    pod_ready=$(kubectl_retry get pods -l app.kubernetes.io/name=snapshot-controller -n "${namespace}" -o jsonpath='{.items[0].status.containerStatuses[0].ready}')
    INC=0
    until [[ "${pod_ready}" == "true" || $INC -gt 20 ]]; do
        sleep 10
        ((++INC))
        pod_ready=$(kubectl_retry get pods -l app.kubernetes.io/name=snapshot-controller -n "${namespace}" -o jsonpath='{.items[0].status.containerStatuses[0].ready}')
        echo "snapshotter pod status: ${pod_ready}"
    done

    if [ "${pod_ready}" != "true" ]; then
        echo "snapshotter controller creation failed"
        kubectl_retry get pods -l app.kubernetes.io/name=snapshot-controller -n "${namespace}"
        kubectl_retry describe po -l app.kubernetes.io/name=snapshot-controller -n "${namespace}"
        exit 1
    fi

    echo "snapshot controller creation successful"
}

function cleanup_snapshot_controller() {
    local namespace=$1
    if [ -z "${namespace}" ]; then
        namespace="kube-system"
    fi
    create_or_delete_resource "delete" "${namespace}"
}

function create_or_delete_resource() {
    local operation=$1
    local namespace=$2
    temp_rbac=${TEMP_DIR}/snapshot-rbac.yaml
    temp_snap_controller=${TEMP_DIR}/snapshot-controller.yaml
    mkdir -p "${TEMP_DIR}"
    curl -o "${temp_rbac}" "${SNAPSHOT_RBAC}"
    curl -o "${temp_snap_controller}" "${SNAPSHOT_CONTROLLER}"
    sed -i "s/namespace: kube-system/namespace: ${namespace}/g" "${temp_rbac}"
    sed -i "s/namespace: kube-system/namespace: ${namespace}/g" "${temp_snap_controller}"
    sed -i -E "s/(image: registry\.k8s\.io\/sig-storage\/snapshot-controller:).*$/\1$SNAPSHOT_VERSION/g" "${temp_snap_controller}"

    if [ "${operation}" == "create" ]; then
        # Argument to add/update
        ARGUMENT="--enable-volume-group-snapshots=true"
        # Check if the argument is already present and set to false
        if grep -q -E "^\s+-\s+--enable-volume-group-snapshots=false" "${temp_snap_controller}"; then
            sed -i -E "s/^\s+-\s+--enable-volume-group-snapshots=false$/      - $ARGUMENT/" "${temp_snap_controller}"
            # Check if the argument is already present and set to true
        elif grep -q -E "^\s+-\s+--enable-volume-group-snapshots=true" "${temp_snap_controller}"; then
            echo "Argument already present and matching."
        else
            # Add the argument if it's not present
            sed -i -E "/^(\s+)args:/a\           \ - $ARGUMENT" "${temp_snap_controller}"
        fi
    fi

    kubectl_retry "${operation}" -f "${VOLUME_GROUP_SNAPSHOTCLASS}"
    kubectl_retry "${operation}" -f "${VOLUME_GROUP_SNAPSHOT_CONTENT}"
    kubectl_retry "${operation}" -f "${VOLUME_GROUP_SNAPSHOT}"
    kubectl_retry "${operation}" -f "${temp_rbac}"
    kubectl_retry "${operation}" -f "${temp_snap_controller}" -n "${namespace}"
    kubectl_retry "${operation}" -f "${SNAPSHOTCLASS}"
    kubectl_retry "${operation}" -f "${VOLUME_SNAPSHOT_CONTENT}"
    kubectl_retry "${operation}" -f "${VOLUME_SNAPSHOT}"
}

function install_snapshot_metadata() {
    local namespace=$1
    local driverName="csi-rbdplugin"
    # install snapshot metadata service crd
    # install openssl
    # provision TLS certs
    # create TLS secret
    # create snapshotMetadataService resource

    provision_tls_certs "${namespace}" "${driverName}"
    create_or_delete_tls_certs "create" "${namespace}" "${driverName}"
    create_or_delete_snapshot_metadata_service "create" "${namespace}" "${driverName}"
}

function cleanup_snapshot_metadata() {
    local namespace=$1
    local driverName="csi-rbdplugin"
    create_or_delete_tls_certs "delete" "${namespace}" "${driverName}"
    create_or_delete_snapshot_metadata_service "delete" "${namespace}" "${driverName}"
    rm -rf "${TEMP_DIR}"
}

function provision_tls_certs() {
    local namespace=$1
    local driverName=$2
    # TODO: install openssl if not present
    
    # 1. Create extension file
    echo "subjectAltName=DNS:.default,DNS:${driverName}.default,IP:0.0.0.0" > ${TEMP_DIR}/server-ext.cnf

    # 2. Generate CA's private key and self-signed certificate
    openssl req -x509 -newkey rsa:4096 -keyout ${TEMP_DIR}/ca-key.pem -out ${TEMP_DIR}/ca-cert.pem -days 365 -nodes -subj "/CN=${driverName}.${namespace}"
    openssl x509 -in ca-cert.pem -noout -text

    # 3. Generate web server's private key and certificate signing request (CSR)
    openssl req -newkey rsa:4096 -nodes -keyout ${TEMP_DIR}/server-key.pem -out ${TEMP_DIR}/server-req.pem -subj "/CN=${driverName}.${namespace}"

    # 4. Use CA's private key to sign web server's CSR and get back the signed certificate
    openssl x509 -req -in ${TEMP_DIR}/server-req.pem -days 60 -CA ${TEMP_DIR}/ca-cert.pem -CAkey ${TEMP_DIR}/ca-key.pem -CAcreateserial -out ${TEMP_DIR}/server-cert.pem -extfile ${TEMP_DIR}/server-ext.cnf
    openssl x509 -in ${TEMP_DIR}/server-cert.pem -noout -text
}

function create_or_delete_tls_certs() {
    local operation=$1
    local namespace=$2
    local driverName=$3
    local server_cert="${TEMP_DIR}/server-cert.pem"
    local server_key="${TEMP_DIR}/server-key.pem"
    kubectl_retry "${operation}" "secret tls ${driverName}-certs --namespace=${namespace} --cert=${server_cert} --key=${server-key}"
}

function create_or_delete_snapshot_metadata_service() {
    local operation=$1
    local namespace=$2
    local driverName=$3
    local generated_ca_cert
    if [ -Z "${TEMP_DIR}/ca-cert.pem" ]; then
        generated_ca_cert=$(base64 -i -w 0 ${TEMP_DIR}/ca-cert.pem)
    fi

    temp_file=$(mktemp "${TEMP_DIR}/snapshot-metadata-service.XXXXXX.yaml")
    cat <<EOF > "${temp_file}"
apiVersion: cbt.storage.k8s.io/v1alpha1
kind: SnapshotMetadataService
metadata:
  name: ${driverName}-snapshot-metadata-service
spec:
    address: ${driverName}.default:6443
    caCert: ${generated_ca_cert}
EOF
    kubectl_retry "${operation}" -f "${temp_file}"
}
    
case "${1:-}" in
install)
    install_snapshot_controller "$2"
    install_snapshot_metadata "$2"
    ;;
cleanup)
    cleanup_snapshot_controller "$2"
    cleanup_snapshot_metadata "$2"
    ;;
*)
    echo "usage:" >&2
    echo "  $0 install" >&2
    echo "  $0 cleanup" >&2
    ;;
esac
