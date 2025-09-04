#!/bin/sh

set -e

# Generate Nginx configuration
envsubst '${BACKEND_HOST} ${BACKEND_PORT}' < /etc/nginx/templates/nginx.conf.template > /etc/nginx/conf.d/default.conf

# Generate JavaScript configuration for the frontend
envsubst '${VITE_SPOTIFY_CLIENT_ID} ${VITE_SPOTIFY_REDIRECT_URI} ${VITE_YOUTUBE_CLIENT_ID} ${VITE_YOUTUBE_REDIRECT_URI}' \
         < /usr/share/nginx/html/config.js.template > /usr/share/nginx/html/config.js

echo "--- Generated NGINX config ---"
cat /etc/nginx/conf.d/default.conf
echo "------------------------------"

echo "--- Generated Frontend config.js ---"
cat /usr/share/nginx/html/config.js
echo "------------------------------------"

exec nginx -g 'daemon off;'
