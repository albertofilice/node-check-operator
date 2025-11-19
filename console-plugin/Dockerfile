# Dockerfile multi-architettura per Console Plugin
# Build completa all'interno del container per AMD64 e ARM64

# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Installa dipendenze per build
RUN apk add --no-cache python3 make g++

# Copy package files
COPY package*.json ./
RUN npm install --legacy-peer-deps

# Copy source code
COPY . .

# Build del plugin
RUN npm run build

# Verifica che il manifest sia stato generato
RUN test -f /app/dist/plugin-manifest.json || (echo "ERROR: plugin-manifest.json non generato!" && ls -la /app/dist/ && exit 1)
RUN echo "Manifest generato correttamente:" && cat /app/dist/plugin-manifest.json | head -20

# Production stage
FROM nginx:alpine

# Copy built plugin (include plugin-manifest.json generato da webpack)
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Crea directory per certificati SSL (verranno montati da OpenShift Secret)
RUN mkdir -p /etc/nginx/ssl && \
    chmod 755 /etc/nginx/ssl

# Crea directory per nginx con permessi corretti
RUN mkdir -p /var/cache/nginx/client_temp \
    /var/cache/nginx/proxy_temp \
    /var/cache/nginx/fastcgi_temp \
    /var/cache/nginx/uwsgi_temp \
    /var/cache/nginx/scgi_temp \
    /var/run/nginx && \
    chown -R root:root /var/cache/nginx \
    /var/run/nginx \
    /usr/share/nginx/html \
    /etc/nginx/conf.d \
    /var/log/nginx && \
    chmod -R 755 /var/cache/nginx \
    /var/run/nginx \
    /usr/share/nginx/html

# Il nostro nginx.conf già configura user root

EXPOSE 80 443

# Esegui nginx come root
CMD ["nginx", "-g", "daemon off;"]
