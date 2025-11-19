#!/bin/bash

# Node Check Operator - Unified Build Script
# Multi-architecture build for AMD64 and ARM64

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Default configuration
REGISTRY=${REGISTRY:-"quay.io"}
IMAGE_NAME=${IMAGE_NAME:-"node-check-operator"}
VERSION=${VERSION:-"latest"}
PUSH=${PUSH:-"false"}
NO_CACHE=${NO_CACHE:-"false"}
COMPONENT=${COMPONENT:-"all"}  # all, operator, console-plugin
AMD64_ONLY=${AMD64_ONLY:-"false"}  # Build only for AMD64

# Show help
show_help() {
    echo "Node Check Operator - Build Script"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --registry REGISTRY    Registry for image push (default: quay.io)"
    echo "  --image-name NAME      Image name (default: node-check-operator)"
    echo "  --version VERSION      Image version (default: latest)"
    echo "  --component COMPONENT  Component to build: all, operator, console-plugin (default: all)"
    echo "  --push                 Push images to registry"
    echo "  --no-cache             Force rebuild without using cache"
    echo "  --amd64-only           Build only for AMD64 (skip ARM64)"
    echo "  --help                 Show this help"
    echo
    echo "Examples:"
    echo "  $0"
    echo "  $0 --registry docker.io --image-name my-org/node-check-operator --version v1.0.0 --push"
    echo "  $0 --component operator --version v1.0.0"
    echo "  $0 --component console-plugin --version v1.0.0 --push"
    echo "  $0 --amd64-only --version v1.0.0  # Build only for AMD64 (avoids memory issues with ARM64)"
    echo
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --registry)
                REGISTRY="$2"
                shift 2
                ;;
            --image-name)
                IMAGE_NAME="$2"
                shift 2
                ;;
            --version)
                VERSION="$2"
                shift 2
                ;;
            --component)
                COMPONENT="$2"
                if [[ ! "$COMPONENT" =~ ^(all|operator|console-plugin)$ ]]; then
                    log_error "Invalid component: $COMPONENT. Use: all, operator, or console-plugin"
                    exit 1
                fi
                shift 2
                ;;
            --push)
                PUSH="true"
                shift
                ;;
            --no-cache)
                NO_CACHE="true"
                shift
                ;;
            --amd64-only)
                AMD64_ONLY="true"
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Argomento sconosciuto: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Verifica prerequisiti
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker or Podman
    if command -v docker &> /dev/null; then
        CONTAINER_CMD="docker"
        log_info "Found Docker"
    elif command -v podman &> /dev/null; then
        CONTAINER_CMD="podman"
        log_info "Found Podman"
    else
        log_error "Neither Docker nor Podman is installed. Install one of them."
        exit 1
    fi
    
    # Check buildx for Docker or build for Podman
    if [ "$CONTAINER_CMD" = "docker" ]; then
        if ! docker buildx version &> /dev/null; then
            log_error "Docker buildx is not available. Update Docker."
            exit 1
        fi
    elif [ "$CONTAINER_CMD" = "podman" ]; then
        if ! podman build --help &> /dev/null; then
            log_error "Podman build is not available. Update Podman."
            exit 1
        fi
    fi
    
    log_info "Prerequisites verified"
}

# Configure buildx or build
setup_build() {
    log_step "Configuring build environment..."
    
    if [ "$CONTAINER_CMD" = "docker" ]; then
        # Create multi-arch builder if it doesn't exist
        if ! docker buildx ls | grep -q "multiarch"; then
            log_info "Creating multi-architecture builder..."
            docker buildx create --name multiarch --use
        else
            log_info "Usando builder multi-architettura esistente..."
            docker buildx use multiarch
        fi
        
        # Verifica che il builder supporti le architetture target
        log_info "Architetture supportate dal builder:"
        docker buildx inspect --bootstrap
    elif [ "$CONTAINER_CMD" = "podman" ]; then
        log_info "Usando Podman per build multi-architettura..."
        # Podman supporta nativamente multi-arch
    fi
}

# Build dell'operator multi-architettura
build_operator() {
    log_step "Building Node Check Operator per multiple architetture..."
    
    # Nomi immagini
    OPERATOR_IMAGE="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
    OPERATOR_IMAGE_AMD64="${REGISTRY}/${IMAGE_NAME}:${VERSION}-amd64"
    OPERATOR_IMAGE_ARM64="${REGISTRY}/${IMAGE_NAME}:${VERSION}-arm64"
    
    log_info "Building operator: ${OPERATOR_IMAGE}"
    
    # Build dell'operator per multiple architettura
    if [ "$CONTAINER_CMD" = "docker" ]; then
        # Prepara flag no-cache se richiesto
        NO_CACHE_FLAG=""
        if [ "${NO_CACHE}" = "true" ]; then
            NO_CACHE_FLAG="--no-cache"
            log_info "Forzando rebuild senza cache..."
        fi
        
        # Determina le piattaforme da buildare
        if [ "${AMD64_ONLY}" = "true" ]; then
            PLATFORMS="linux/amd64"
            TAGS="${OPERATOR_IMAGE} ${OPERATOR_IMAGE_AMD64}"
            log_info "Building solo per AMD64 (--amd64-only)"
        else
            PLATFORMS="linux/amd64,linux/arm64"
            TAGS="${OPERATOR_IMAGE} ${OPERATOR_IMAGE_AMD64} ${OPERATOR_IMAGE_ARM64}"
        fi
        
        if [ "${PUSH}" = "true" ]; then
            docker buildx build \
                ${NO_CACHE_FLAG} \
                --platform ${PLATFORMS} \
                --tag "${OPERATOR_IMAGE}" \
                --tag "${OPERATOR_IMAGE_AMD64}" \
                $(if [ "${AMD64_ONLY}" != "true" ]; then echo "--tag ${OPERATOR_IMAGE_ARM64}"; fi) \
                --push \
                -f Dockerfile \
                .
        else
            docker buildx build \
                ${NO_CACHE_FLAG} \
                --platform ${PLATFORMS} \
                --tag "${OPERATOR_IMAGE}" \
                --tag "${OPERATOR_IMAGE_AMD64}" \
                $(if [ "${AMD64_ONLY}" != "true" ]; then echo "--tag ${OPERATOR_IMAGE_ARM64}"; fi) \
                --load \
                -f Dockerfile \
                .
        fi
    elif [ "$CONTAINER_CMD" = "podman" ]; then
        # Prepara flag no-cache se richiesto
        NO_CACHE_FLAG=""
        if [ "${NO_CACHE}" = "true" ]; then
            NO_CACHE_FLAG="--no-cache"
            log_info "Forzando rebuild senza cache..."
        fi
        
        # Podman build per AMD64
        podman build \
            ${NO_CACHE_FLAG} \
            --platform linux/amd64 \
            --tag "${OPERATOR_IMAGE_AMD64}" \
            -f Dockerfile \
            .
        
        # Podman build for ARM64 only if not --amd64-only
        if [ "${AMD64_ONLY}" != "true" ]; then
            log_info "Building per ARM64..."
            podman build \
                ${NO_CACHE_FLAG} \
                --platform linux/arm64 \
                --tag "${OPERATOR_IMAGE_ARM64}" \
                -f Dockerfile \
                .
        else
            log_info "Skippando build ARM64 (--amd64-only)"
        fi
        
        # Crea manifest per immagine multi-arch
        if [ "${PUSH}" = "true" ]; then
            if [ "${AMD64_ONLY}" = "true" ]; then
                # Se solo AMD64, tagga direttamente senza manifest
                podman tag "${OPERATOR_IMAGE_AMD64}" "${OPERATOR_IMAGE}"
                podman push "${OPERATOR_IMAGE}"
            else
                # Rimuovi manifest esistente se presente
                podman manifest rm "${OPERATOR_IMAGE}" 2>/dev/null || true
                # Crea nuovo manifest
                podman manifest create "${OPERATOR_IMAGE}" "${OPERATOR_IMAGE_AMD64}" "${OPERATOR_IMAGE_ARM64}"
                podman manifest push "${OPERATOR_IMAGE}" "docker://${OPERATOR_IMAGE}"
            fi
        else
            # Per build locale, tagga l'immagine per l'architettura corrente
            CURRENT_ARCH=$(uname -m)
            if [ "$CURRENT_ARCH" = "x86_64" ]; then
                podman tag "${OPERATOR_IMAGE_AMD64}" "${OPERATOR_IMAGE}"
            elif [ "$CURRENT_ARCH" = "aarch64" ] && [ "${AMD64_ONLY}" != "true" ]; then
                podman tag "${OPERATOR_IMAGE_ARM64}" "${OPERATOR_IMAGE}"
            else
                # Se solo AMD64, tagga comunque l'immagine AMD64
                podman tag "${OPERATOR_IMAGE_AMD64}" "${OPERATOR_IMAGE}"
            fi
        fi
    fi
    
    log_info "Operator build completed"
}

# Build del console plugin multi-architettura
build_console_plugin() {
    log_step "Building OpenShift Console Plugin per multiple architetture..."
    
    # Nomi immagini
    PLUGIN_IMAGE="${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}"
    PLUGIN_IMAGE_AMD64="${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}-amd64"
    PLUGIN_IMAGE_ARM64="${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}-arm64"
    
    log_info "Building console plugin: ${PLUGIN_IMAGE}"
    
    # Build del plugin per multiple architetture
    if [ "$CONTAINER_CMD" = "docker" ]; then
        # Prepara flag no-cache se richiesto
        NO_CACHE_FLAG=""
        if [ "${NO_CACHE}" = "true" ]; then
            NO_CACHE_FLAG="--no-cache"
        fi
        
        # Determina le piattaforme da buildare
        if [ "${AMD64_ONLY}" = "true" ]; then
            PLATFORMS="linux/amd64"
            log_info "Building console plugin solo per AMD64 (--amd64-only)"
        else
            PLATFORMS="linux/amd64,linux/arm64"
        fi
        
        if [ "${PUSH}" = "true" ]; then
            docker buildx build \
                ${NO_CACHE_FLAG} \
                --platform ${PLATFORMS} \
                --tag "${PLUGIN_IMAGE}" \
                --tag "${PLUGIN_IMAGE_AMD64}" \
                $(if [ "${AMD64_ONLY}" != "true" ]; then echo "--tag ${PLUGIN_IMAGE_ARM64}"; fi) \
                --push \
                -f console-plugin/Dockerfile \
                console-plugin/
        else
            docker buildx build \
                ${NO_CACHE_FLAG} \
                --platform ${PLATFORMS} \
                --tag "${PLUGIN_IMAGE}" \
                --tag "${PLUGIN_IMAGE_AMD64}" \
                $(if [ "${AMD64_ONLY}" != "true" ]; then echo "--tag ${PLUGIN_IMAGE_ARM64}"; fi) \
                --load \
                -f console-plugin/Dockerfile \
                console-plugin/
        fi
    elif [ "$CONTAINER_CMD" = "podman" ]; then
        # Prepara flag no-cache se richiesto
        NO_CACHE_FLAG=""
        if [ "${NO_CACHE}" = "true" ]; then
            NO_CACHE_FLAG="--no-cache"
        fi
        
        # Podman build per AMD64
        podman build \
            ${NO_CACHE_FLAG} \
            --platform linux/amd64 \
            --tag "${PLUGIN_IMAGE_AMD64}" \
            -f console-plugin/Dockerfile \
            console-plugin/
        
        # Podman build for ARM64 only if not --amd64-only
        if [ "${AMD64_ONLY}" != "true" ]; then
            log_info "Building console plugin per ARM64..."
            podman build \
                ${NO_CACHE_FLAG} \
                --platform linux/arm64 \
                --tag "${PLUGIN_IMAGE_ARM64}" \
                -f console-plugin/Dockerfile \
                console-plugin/
        else
            log_info "Skippando build console plugin ARM64 (--amd64-only)"
        fi
        
        # Crea manifest per immagine multi-arch
        if [ "${PUSH}" = "true" ]; then
            if [ "${AMD64_ONLY}" = "true" ]; then
                # Se solo AMD64, tagga direttamente senza manifest
                podman tag "${PLUGIN_IMAGE_AMD64}" "${PLUGIN_IMAGE}"
                podman push "${PLUGIN_IMAGE}"
            else
                # Rimuovi manifest esistente se presente
                podman manifest rm "${PLUGIN_IMAGE}" 2>/dev/null || true
                # Crea nuovo manifest
                podman manifest create "${PLUGIN_IMAGE}" "${PLUGIN_IMAGE_AMD64}" "${PLUGIN_IMAGE_ARM64}"
                podman manifest push "${PLUGIN_IMAGE}" "docker://${PLUGIN_IMAGE}"
            fi
        else
            # Per build locale, tagga l'immagine per l'architettura corrente
            CURRENT_ARCH=$(uname -m)
            if [ "$CURRENT_ARCH" = "x86_64" ]; then
                podman tag "${PLUGIN_IMAGE_AMD64}" "${PLUGIN_IMAGE}"
            elif [ "$CURRENT_ARCH" = "aarch64" ] && [ "${AMD64_ONLY}" != "true" ]; then
                podman tag "${PLUGIN_IMAGE_ARM64}" "${PLUGIN_IMAGE}"
            else
                # Se solo AMD64, tagga comunque l'immagine AMD64
                podman tag "${PLUGIN_IMAGE_AMD64}" "${PLUGIN_IMAGE}"
            fi
        fi
    fi
    
    log_info "Console plugin build completed"
}

# Aggiorna manifesti con immagini corrette
update_manifests() {
    log_step "Aggiornando manifesti con immagini corrette..."
    
    # Update manager.yaml only if building operator
    # Note: Console plugin deployment is managed by the operator, so we don't update it here
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "operator" ]]; then
        sed -i.bak "s|image: controller:latest|image: ${REGISTRY}/${IMAGE_NAME}:${VERSION}|g" config/manager/manager.yaml
        rm -f config/manager/manager.yaml.bak
    fi
    
    log_info "Manifests updated"
}

# Verifica build
verify_build() {
    log_step "Verificando build..."
    
    # Verifica immagini Docker
    log_info "Immagini create:"
    podman images | grep "${IMAGE_NAME}" || true
    
    # Verifica architetture se non push
    if [ "${PUSH}" = "false" ]; then
        log_info "Verificando architetture supportate..."
        docker buildx imagetools inspect "${REGISTRY}/${IMAGE_NAME}:${VERSION}" 2>/dev/null || true
        docker buildx imagetools inspect "${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}" 2>/dev/null || true
    fi
}

# Mostra informazioni post-build
show_build_info() {
    log_info "Build completato!"
    echo
    echo "Immagini Docker create:"
    
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "operator" ]]; then
        if [ "${AMD64_ONLY}" = "true" ]; then
            echo "  - ${REGISTRY}/${IMAGE_NAME}:${VERSION} (AMD64 only)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}:${VERSION}-amd64 (AMD64 only)"
        else
            echo "  - ${REGISTRY}/${IMAGE_NAME}:${VERSION} (AMD64 + ARM64)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}:${VERSION}-amd64 (AMD64 only)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}:${VERSION}-arm64 (ARM64 only)"
        fi
    fi
    
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "console-plugin" ]]; then
        if [ "${AMD64_ONLY}" = "true" ]; then
            echo "  - ${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION} (AMD64 only)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}-amd64 (AMD64 only)"
        else
            echo "  - ${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION} (AMD64 + ARM64)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}-amd64 (AMD64 only)"
            echo "  - ${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}-arm64 (ARM64 only)"
        fi
    fi
    
    echo
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "operator" ]]; then
        echo "Per installare l'operator:"
        echo "  ./scripts/install.sh --registry ${REGISTRY} --image-name ${IMAGE_NAME} --version ${VERSION}"
        echo
    fi
    if [ "${PUSH}" = "true" ]; then
        echo "Immagini pushate su registry: ${REGISTRY}"
    else
        echo "Immagini disponibili localmente"
    fi
}

# Funzione principale
main() {
    log_info "Iniziando build Node Check Operator..."
    
    parse_args "$@"
    check_prerequisites
    setup_build
    
    # Build componenti selezionati
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "operator" ]]; then
        build_operator
    fi
    
    if [[ "$COMPONENT" == "all" || "$COMPONENT" == "console-plugin" ]]; then
        build_console_plugin
    fi
    
    update_manifests
    verify_build
    show_build_info
    
    log_info "Build completato con successo! 🚀"
}

# Esegui main
main "$@"