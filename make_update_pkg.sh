#!/bin/bash
# 从已构建的 Docker 镜像中提取 familytree 二进制 + web 前端，打包成更新包
# 用法：./make_update_pkg.sh <版本号> [镜像名]
# 例：./make_update_pkg.sh 0.3.0826.1
# 会在当前目录生成 族谱系统v0.3.0826.1.update.tar.gz

set -e

VERSION="${1:?用法: ./make_update_pkg.sh <版本号如0.3.0826.1> [镜像名]}"
IMAGE="${2:-zupu-familytree}"

# 找到镜像
echo "==> 检查镜像: $IMAGE"
docker image inspect "$IMAGE" >/dev/null 2>&1 || {
  echo "错误: 镜像 $IMAGE 不存在，请先 docker compose build"
  echo "  也可指定镜像名: ./make_update_pkg.sh $VERSION <镜像名>"
  exit 1
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "==> 从镜像提取 /app/familytree 二进制"
CID=$(docker create "$IMAGE")
docker cp "$CID:/app/familytree" "$TMP/familytree"
docker cp "$CID:/app/web" "$TMP/web"
docker rm "$CID" >/dev/null

chmod +x "$TMP/familytree"
chmod -R a+rX "$TMP/web"

# 校验
[ -x "$TMP/familytree" ] || { echo "错误: 未提取到二进制"; exit 1; }
[ -f "$TMP/web/index.html" ] || { echo "错误: 未提取到前端 index.html"; exit 1; }

PKG="族谱系统v${VERSION}.update.tar.gz"
echo "==> 打包 $PKG"
tar -czf "$PKG" -C "$TMP" familytree web

echo "==> 完成: $(pwd)/$PKG"
ls -lh "$PKG"
echo ""
echo "更新包结构:"
tar -tzf "$PKG" | head -20
echo "..."
echo "下一步: 登录目标系统(超管) → 版本更新页 → 上传此文件 → 立即更新"
