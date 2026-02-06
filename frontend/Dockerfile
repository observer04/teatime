# Stage 1: Build the React application
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Set environment variables for build time replacement (if using specific env file loading or replace plugin)
# However, Vite replaces import.meta.env.* at build time. 
# We'll use a placeholder or assume these are set via build-args if strictly needed,
# but for runtime config in container, we might need a different approach.
# For now, let's assume standard build.
ARG VITE_API_BASE
ARG VITE_WS_BASE
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_WS_BASE=$VITE_WS_BASE
RUN npm run build

# Stage 2: Serve with Nginx
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
