#!/bin/bash
set -euo pipefail

# MinIO to RustFS Data Migration Script
# This script migrates all data from MinIO to RustFS using rclone
# Both are S3-compatible, so migration is a straightforward sync operation

# Configuration
MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://localhost:9010}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-minioadmin}"

RUSTFS_ENDPOINT="${RUSTFS_ENDPOINT:-http://localhost:9000}"
RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY:-rustfsadmin}"
RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY:-rustfsadmin}"

RCLONE_CONFIG_DIR="${RCLONE_CONFIG_DIR:-/tmp/rclone-migration}"
LOG_FILE="${LOG_FILE:-/tmp/migration-$(date +%Y%m%d-%H%M%S).log}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1" | tee -a "$LOG_FILE"
}

# Check if rclone is installed
check_rclone() {
    if ! command -v rclone &> /dev/null; then
        error "rclone is not installed. Please install it first:"
        echo "  curl https://rclone.org/install.sh | sudo bash"
        exit 1
    fi
    log "rclone version: $(rclone version | head -1)"
}

# Create rclone configuration
setup_rclone_config() {
    log "Setting up rclone configuration..."
    
    mkdir -p "$RCLONE_CONFIG_DIR"
    
    cat > "$RCLONE_CONFIG_DIR/rclone.conf" << EOF
[minio]
type = s3
provider = Minio
env_auth = false
access_key_id = ${MINIO_ACCESS_KEY}
secret_access_key = ${MINIO_SECRET_KEY}
endpoint = ${MINIO_ENDPOINT}
acl = private

[rustfs]
type = s3
provider = Minio
env_auth = false
access_key_id = ${RUSTFS_ACCESS_KEY}
secret_access_key = ${RUSTFS_SECRET_KEY}
endpoint = ${RUSTFS_ENDPOINT}
acl = private
EOF

    export RCLONE_CONFIG="$RCLONE_CONFIG_DIR/rclone.conf"
    log "rclone configuration created at $RCLONE_CONFIG"
}

# List all buckets in MinIO
list_minio_buckets() {
    log "Listing buckets in MinIO..."
    rclone lsd minio: 2>/dev/null | awk '{print $5}' || echo ""
}

# Create bucket in RustFS if it doesn't exist
create_rustfs_bucket() {
    local bucket=$1
    log "Creating bucket '$bucket' in RustFS..."
    rclone mkdir "rustfs:$bucket" 2>/dev/null || true
}

# Migrate a single bucket
migrate_bucket() {
    local bucket=$1
    log "Migrating bucket: $bucket"
    
    # Create bucket in RustFS
    create_rustfs_bucket "$bucket"
    
    # Get object count before migration
    local source_count=$(rclone size "minio:$bucket" 2>/dev/null | grep "Total objects:" | awk '{print $3}' || echo "0")
    log "  Source objects: $source_count"
    
    # Sync data from MinIO to RustFS
    log "  Starting sync..."
    rclone sync "minio:$bucket" "rustfs:$bucket" \
        --progress \
        --transfers 8 \
        --checkers 16 \
        --s3-chunk-size 64M \
        --s3-upload-concurrency 4 \
        --log-file "$LOG_FILE" \
        --log-level INFO \
        2>&1 | tee -a "$LOG_FILE"
    
    # Verify migration
    local dest_count=$(rclone size "rustfs:$bucket" 2>/dev/null | grep "Total objects:" | awk '{print $3}' || echo "0")
    log "  Destination objects: $dest_count"
    
    if [ "$source_count" == "$dest_count" ]; then
        log "  Migration verified: object counts match"
    else
        warn "  Object count mismatch! Source: $source_count, Destination: $dest_count"
    fi
}

# Verify data integrity using checksums
verify_bucket() {
    local bucket=$1
    log "Verifying bucket: $bucket"
    
    rclone check "minio:$bucket" "rustfs:$bucket" \
        --one-way \
        --log-file "$LOG_FILE" \
        --log-level INFO \
        2>&1 | tee -a "$LOG_FILE"
    
    if [ $? -eq 0 ]; then
        log "  Verification passed for bucket: $bucket"
        return 0
    else
        error "  Verification failed for bucket: $bucket"
        return 1
    fi
}

# Main migration function
migrate_all() {
    log "Starting MinIO to RustFS migration..."
    log "MinIO endpoint: $MINIO_ENDPOINT"
    log "RustFS endpoint: $RUSTFS_ENDPOINT"
    log "Log file: $LOG_FILE"
    
    # Get list of buckets
    local buckets=$(list_minio_buckets)
    
    if [ -z "$buckets" ]; then
        warn "No buckets found in MinIO"
        return 0
    fi
    
    log "Found buckets: $buckets"
    
    # Migrate each bucket
    local failed_buckets=""
    for bucket in $buckets; do
        migrate_bucket "$bucket"
        
        # Verify migration
        if ! verify_bucket "$bucket"; then
            failed_buckets="$failed_buckets $bucket"
        fi
    done
    
    # Summary
    log ""
    log "=========================================="
    log "Migration Summary"
    log "=========================================="
    
    if [ -z "$failed_buckets" ]; then
        log "All buckets migrated successfully!"
        return 0
    else
        error "Failed buckets:$failed_buckets"
        return 1
    fi
}

# Rollback function (copy data back to MinIO)
rollback() {
    log "Starting rollback: RustFS to MinIO..."
    
    local buckets=$(rclone lsd rustfs: 2>/dev/null | awk '{print $5}' || echo "")
    
    for bucket in $buckets; do
        log "Rolling back bucket: $bucket"
        rclone sync "rustfs:$bucket" "minio:$bucket" \
            --progress \
            --transfers 8 \
            --checkers 16 \
            2>&1 | tee -a "$LOG_FILE"
    done
    
    log "Rollback complete"
}

# Dry run function
dry_run() {
    log "Performing dry run..."
    
    local buckets=$(list_minio_buckets)
    
    for bucket in $buckets; do
        log "Would migrate bucket: $bucket"
        rclone sync "minio:$bucket" "rustfs:$bucket" \
            --dry-run \
            --progress \
            2>&1 | tee -a "$LOG_FILE"
    done
}

# Print usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  migrate   - Migrate all data from MinIO to RustFS"
    echo "  verify    - Verify data integrity after migration"
    echo "  rollback  - Rollback: copy data from RustFS back to MinIO"
    echo "  dry-run   - Perform a dry run (no actual data transfer)"
    echo "  help      - Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  MINIO_ENDPOINT     - MinIO endpoint (default: http://localhost:9010)"
    echo "  MINIO_ACCESS_KEY   - MinIO access key (default: minioadmin)"
    echo "  MINIO_SECRET_KEY   - MinIO secret key (default: minioadmin)"
    echo "  RUSTFS_ENDPOINT    - RustFS endpoint (default: http://localhost:9000)"
    echo "  RUSTFS_ACCESS_KEY  - RustFS access key (default: rustfsadmin)"
    echo "  RUSTFS_SECRET_KEY  - RustFS secret key (default: rustfsadmin)"
}

# Main entry point
main() {
    local command="${1:-migrate}"
    
    check_rclone
    setup_rclone_config
    
    case "$command" in
        migrate)
            migrate_all
            ;;
        verify)
            local buckets=$(list_minio_buckets)
            for bucket in $buckets; do
                verify_bucket "$bucket"
            done
            ;;
        rollback)
            rollback
            ;;
        dry-run)
            dry_run
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

main "$@"
