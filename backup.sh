#!/bin/bash
# 族谱系统 - 更新前备份脚本
# 用法: ./backup.sh
# 自动读取 settings 表版本号，备份到 系统备份/ 目录，跳过数据目录

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKUP_DIR="$SCRIPT_DIR/系统备份"
mkdir -p "$BACKUP_DIR"

# 尝试从数据库读取版本号
VERSION="unknown"
DB_PATH="$SCRIPT_DIR/data/data/familytree.db"
if [ -f "$DB_PATH" ]; then
  VER=$(sqlite3 "$DB_PATH" "SELECT value FROM settings WHERE key='version'" 2>/dev/null || echo "")
  if [ -n "$VER" ]; then
    VERSION="$VER"
  fi
fi

BACKUP_NAME="族谱系统v${VERSION}"
ZIP_FILE="$BACKUP_DIR/${BACKUP_NAME}.zip"

echo "📦 正在备份: $BACKUP_NAME"
echo "   跳过: data/ 系统备份/ node_modules/ dist/ .git/"

cd "$SCRIPT_DIR"
zip -r "$ZIP_FILE" . \
  -x "data/*" "系统备份/*" \
  "frontend/node_modules/*" "frontend/dist/*" \
  ".git/*" \
  "*.zip" "*.rar"

echo "✅ 备份完成: $ZIP_FILE"
ls -lh "$ZIP_FILE"