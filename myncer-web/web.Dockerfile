# Build stage
FROM node:20 AS builder

WORKDIR /app
COPY . .

# Avoid interactive prompts
ENV CI=true

RUN corepack enable && pnpm install && pnpm build

# Serve with Nginx
FROM nginx:stable-alpine AS runner

# Install gettext for envsubst utility
RUN apk add --no-cache gettext

COPY --from=builder /app/dist /usr/share/nginx/html

# Copy config template
COPY ./config.js.template /usr/share/nginx/html/config.js.template

# Copy nginx template and entrypoint script
COPY ./nginx.conf.template /etc/nginx/templates/nginx.conf.template
COPY ./entrypoint.sh /entrypoint.sh

# Make entrypoint script executable
RUN chmod +x /entrypoint.sh

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]
