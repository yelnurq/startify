# syntax=docker/dockerfile:1.7
# Builds the single-page application and serves it with an unprivileged Nginx
# that also proxies /api to the API service.

FROM node:24-alpine AS build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-fund --no-audit
COPY backend/api/openapi.yaml /src/backend/api/openapi.yaml
COPY frontend/ ./
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.29-alpine
COPY deploy/nginx/web.conf /etc/nginx/conf.d/default.conf
COPY deploy/nginx/security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY deploy/nginx/realip.conf /etc/nginx/snippets/realip.conf
COPY --from=build /src/frontend/dist /usr/share/nginx/html
EXPOSE 8080
# The web server's own file: "/" goes to the API, which serves only known
# university hosts, so a probe by IP would test the API instead.
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -q -O /dev/null http://127.0.0.1:8080/index.html || exit 1
