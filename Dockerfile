# Dockerfile multi-architettura
# Build completa all'interno del container per AMD64 e ARM64

# Build stage
FROM golang:1.21-alpine AS builder

# Installa dipendenze per build
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY hack/ hack/

# Aggiorna dipendenze e genera manifesti
RUN go mod tidy
RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.13.0

# Genera i file deepcopy e registrazione (rimuovi prima se esistono)
RUN rm -f api/v1alpha1/zz_generated.deepcopy.go api/v1alpha1/zz_generated.register.go
RUN controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/v1alpha1/..." output:object:dir=./api/v1alpha1

# Verifica che i file siano stati generati
RUN test -f api/v1alpha1/zz_generated.deepcopy.go || (echo "ERROR: zz_generated.deepcopy.go non generato!" && ls -la api/v1alpha1/ && exit 1)

# Se controller-gen non genera register.go, la registrazione manuale in groupversion_info.go dovrebbe funzionare
RUN echo "Verificando registrazione tipi..." && ls -la api/v1alpha1/

# Genera RBAC e CRD
RUN controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Verifica che il codice compili prima di buildare
RUN go build -o /tmp/test-build ./api/v1alpha1/... || (echo "ERROR: Compilazione api/v1alpha1 fallita!" && exit 1)

# Build per architettura target
ARG TARGETOS
ARG TARGETARCH
# Limita l'uso di memoria durante la compilazione per evitare crash
ENV GOGC=400
# Rimuovi -a per evitare ricompilazione completa che può causare crash
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -installsuffix cgo -o main main.go

# Runtime stage
FROM alpine:3.18

# Installa runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    procps \
    sysstat \
    iputils \
    iproute2 \
    ethtool \
    smartmontools \
    lm-sensors \
    ipmitool \
    util-linux \
    lvm2 \
    curl

# Crea utente non-root
RUN adduser -D -s /bin/sh nodecheck

# Copia binary
COPY --from=builder /workspace/main /manager

# Imposta permessi
RUN chmod +x /manager

# Cambia utente (ma manteniamo root per privilegi)
# USER nodecheck

ENTRYPOINT ["/manager"]
