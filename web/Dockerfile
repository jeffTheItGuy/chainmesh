# Dockerfile (production)
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Copy openapi.yaml into the build output
COPY ../backend/api/openapi.yaml ./dist/openapi.yaml

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/openapi.yaml /usr/share/nginx/html/openapi.yaml
EXPOSE 80