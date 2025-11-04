#!/bin/bash
# VTB Multibank Backend Deployment Script for Timeweb Cloud
# Usage: ./deploy.sh

set -e  # Exit on error

# Configuration
APP_DIR="/var/www/vtb-multibank"
BACKUP_DIR="/var/www/vtb-multibank-backups"
SERVICE_NAME="vtb-backend"

echo "🚀 Starting deployment..."

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Backup current version
if [ -f "$APP_DIR/server" ]; then
    echo "📦 Backing up current version..."
    BACKUP_FILE="$BACKUP_DIR/server-$(date +%Y%m%d-%H%M%S)"
    cp "$APP_DIR/server" "$BACKUP_FILE"
    echo "✅ Backup saved: $BACKUP_FILE"
fi

# Stop service if using systemd
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "⏸️  Stopping service..."
    sudo systemctl stop $SERVICE_NAME
else
    # Fallback: kill process
    echo "⏸️  Stopping process..."
    pkill -f "$APP_DIR/server" || true
fi

# Wait for port to be free
echo "⏳ Waiting for port 8080 to be free..."
for i in {1..10}; do
    if ! lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null ; then
        break
    fi
    sleep 1
done

# Set permissions
echo "🔐 Setting permissions..."
chmod +x "$APP_DIR/server"
chown -R www-data:www-data "$APP_DIR"

# Create logs directory
mkdir -p "$APP_DIR/logs"

# Start service
if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo "▶️  Starting service with systemd..."
    sudo systemctl start $SERVICE_NAME
    sleep 3
    sudo systemctl status $SERVICE_NAME --no-pager
else
    echo "▶️  Starting process with nohup..."
    cd "$APP_DIR"
    nohup ./server > logs/app.log 2>&1 &
    echo $! > server.pid
fi

# Health check
echo "🏥 Running health check..."
sleep 5

for i in {1..10}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        echo "✅ Health check passed!"
        echo "🎉 Deployment successful!"
        exit 0
    fi
    echo "⏳ Waiting for server to start... ($i/10)"
    sleep 2
done

echo "❌ Health check failed!"
echo "📋 Last 20 lines of logs:"
tail -n 20 "$APP_DIR/logs/app.log"

# Rollback
if [ -n "$BACKUP_FILE" ]; then
    echo "🔄 Rolling back to previous version..."
    cp "$BACKUP_FILE" "$APP_DIR/server"
    chmod +x "$APP_DIR/server"
    
    if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
        sudo systemctl start $SERVICE_NAME
    else
        cd "$APP_DIR"
        nohup ./server > logs/app.log 2>&1 &
    fi
fi

exit 1
