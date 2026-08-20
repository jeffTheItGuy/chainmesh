FROM node:20-alpine
WORKDIR /app
COPY ./web/package*.json ./
RUN npm ci
EXPOSE 5173