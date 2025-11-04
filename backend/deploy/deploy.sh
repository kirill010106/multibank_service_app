#!/bin/bash
set -euo pipefail

APP_DIR="${APP_DIR:-/var/www/vtb-multibank}"
APP_NAME="server"
BACKUP_DIR="${APP_DIR}/backups"
LOG_DIR="${APP_DIR}/logs"
PID_FILE="${APP_DIR}/${APP_NAME}.pid"

echo "🚀 Starting deployment..."

# Create necessary directories
mkdir -p "$BACKUP_DIR" "$LOG_DIR"

# Backup current binary if exists
if [ -f "${APP_DIR}/${APP_NAME}" ]; then
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    BACKUP_PATH="${BACKUP_DIR}/${APP_NAME}_${TIMESTAMP}"
    
    echo "📦 Backing up current version to ${BACKUP_PATH}"
    cp "${APP_DIR}/${APP_NAME}" "${BACKUP_PATH}"
    
    # Keep only last 5 backups
    ls -t "${BACKUP_DIR}/${APP_NAME}_"* 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
fi

# Stop existing process
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "🛑 Stopping process (PID: $OLD_PID)..."
        kill "$OLD_PID"
        
        # Wait for process to stop
        for i in {1..10}; do
            if ! kill -0 "$OLD_PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        
        # Force kill if still running
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo "⚠️  Force killing process..."
            kill -9 "$OLD_PID" || true
        fi
    fi
    rm -f "$PID_FILE"
fi

# Make new binary executable
chmod +x "${APP_DIR}/${APP_NAME}"

# Load environment variables
if [ -f "${APP_DIR}/.env" ]; then
    set -a
    source "${APP_DIR}/.env"
    set +a
else
    echo "⚠️  Warning: .env file not found"
fi

# Start new process with nohup
echo "▶️  Starting new process..."
cd "$APP_DIR"

nohup ./${APP_NAME} > "${LOG_DIR}/app.log" 2>&1 &
NEW_PID=$!
echo $NEW_PID > "$PID_FILE"

echo "✅ Process started (PID: $NEW_PID)"

# Health check
echo "🏥 Running health check..."
sleep 3

MAX_RETRIES=10
for i in $(seq 1 $MAX_RETRIES); do
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        echo "✅ Health check passed!"
        echo "🎉 Deployment completed successfully!"
        exit 0
    fi
    
    echo "⏳ Waiting for server to start (attempt $i/$MAX_RETRIES)..."
    sleep 2
done

# Health check failed - rollback
echo "❌ Health check failed!"

if [ -f "$PID_FILE" ]; then
    FAILED_PID=$(cat "$PID_FILE")
    kill "$FAILED_PID" 2>/dev/null || true
    rm -f "$PID_FILE"
fi

if [ -f "${BACKUP_PATH}" ]; then
    echo "🔄 Rolling back to previous version..."
    cp "${BACKUP_PATH}" "${APP_DIR}/${APP_NAME}"
    chmod +x "${APP_DIR}/${APP_NAME}"
    
    nohup ./${APP_NAME} > "${LOG_DIR}/app.log" 2>&1 &
    ROLLBACK_PID=$!
    echo $ROLLBACK_PID > "$PID_FILE"
    
    sleep 3
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        echo "✅ Rollback successful"
        exit 1
    fi
fi

echo "❌ Rollback failed"
exit 1