#!/bin/bash
# 发布 Android 在线更新
#
#   1) 先用 rebuild.sh 编出 APK（族谱v<版本>-release.apk）
#   2) 再跑本脚本：上传 APK 到网关 /app，并写更新清单
#
#   bash publish-update.sh            # 发到测试通道 app-beta.json（版本号带测试段时用这个）
#   bash publish-update.sh --stable   # 同时更新正式通道 app-latest.json
#
# 清单格式（AppUpdate.kt 读取）：
#   versionCode / versionName / url / size / sha256 / notes / minSdk / force / stable
set -euo pipefail

SRC="/vol1/1000/zupu/android"
OUT="/vol1/1000/Docker/gateway/packages/zupu/app"
BASE_URL="https://zupu-update.lijiacore.com:4440/app"

STABLE=0
[ "${1:-}" = "--stable" ] && STABLE=1

# ── 找最新的 APK ──
APK=$(ls -t "$SRC"/族谱v*-release.apk 2>/dev/null | head -1 || true)
if [ -z "$APK" ] || [ ! -f "$APK" ]; then
  echo "❌ 没找到 族谱v*-release.apk，先跑：bash $SRC/rebuild.sh" >&2
  exit 1
fi

VN=$(basename "$APK" | sed -E 's/^族谱v(.*)-release\.apk$/\1/')
VC=$(awk -F'= *' '/versionCode = /{print $2; exit}' "$SRC/app/build.gradle.kts" | tr -d ' ')
SIZE=$(stat -c%s "$APK")
SHA=$(sha256sum "$APK" | awk '{print $1}')

if [ -z "$VC" ]; then
  echo "❌ 读不到 versionCode（$SRC/app/build.gradle.kts）" >&2
  exit 1
fi

echo "APK    : $APK"
echo "版本   : $VN (versionCode $VC)"
echo "大小   : $SIZE 字节"
echo "sha256 : $SHA"

# ── 复制到网关目录 ──
mkdir -p "$OUT"
DEST="$OUT/zupu-app-$VN.apk"
cp -f "$APK" "$DEST"
chmod 644 "$DEST" 2>/dev/null || true
echo "已上传: $DEST"

NOTES="被控端同步：手机横屏连电脑，操作与数据实时同步；连上后使用手机端身份；被控端只读跟随刷新"

write_manifest() {
  local file="$1" stable="$2"
  cat > "$OUT/$file" <<JSON
{
  "versionCode": $VC,
  "versionName": "$VN",
  "url": "$BASE_URL/zupu-app-$VN.apk",
  "size": $SIZE,
  "sha256": "$SHA",
  "notes": "$NOTES",
  "minSdk": 24,
  "force": false,
  "stable": $stable
}
JSON
  chmod 644 "$OUT/$file" 2>/dev/null || true
  echo "已写清单: /app/$file"
}

# 测试通道总是更新（版本号带 .MMDD.N 时就应该放这里）
write_manifest "app-beta.json" "false"

if [ "$STABLE" = "1" ]; then
  write_manifest "app-latest.json" "true"
fi

# ── 校验 ──
echo
echo "校验："
for f in app-beta.json app-latest.json; do
  [ -f "$OUT/$f" ] || continue
  code=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 20 "$BASE_URL/$f" || echo "000")
  echo "  /app/$f -> HTTP $code"
done
head_code=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 20 "$BASE_URL/zupu-app-$VN.apk" || echo "000")
echo "  /app/zupu-app-$VN.apk -> HTTP $head_code"

echo
echo "完成。App 里长按家族名 = 检查测试通道；设置页「检查 App 更新」= 正式通道。"
