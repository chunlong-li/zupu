#!/bin/bash
# 一键编译族谱 Android 客户端（release 签名；工具链在 /vol1/@appshare/dsh/toolchain）
set -e
TC=/vol1/@appshare/dsh/toolchain
export JAVA_HOME=$TC/jdk17
export ANDROID_HOME=$TC/android-sdk
export GRADLE_USER_HOME=$TC/gh
export GRADLE_OPTS="-Dorg.gradle.native=false"
export LANG=C.UTF-8 LC_ALL=C.UTF-8
export PATH=$TC/gradle-8.7/bin:$JAVA_HOME/bin:$PATH

SRC="$(cd "$(dirname "$0")" && pwd)"
chmod -R u+rwX "$SRC" 2>/dev/null || true

# 必须在纯 ASCII 路径下编译（中文路径会让 aapt/Gradle 报 values.xml 不存在）
BUILD=/tmp/zupu-app-build
chmod -R u+rwX "$BUILD" 2>/dev/null || true
rm -rf "$BUILD" 2>/dev/null || { chmod -R u+rwX "$BUILD"; rm -rf "$BUILD"; }
mkdir -p "$BUILD"
rsync -rlt --no-perms --no-owner --no-group \
  --exclude build --exclude .gradle --exclude local.properties \
  "$SRC"/ "$BUILD"/ 2>/dev/null || cp -r --no-preserve=mode,ownership "$SRC"/. "$BUILD"/
chmod -R u+rwX "$BUILD"
echo "sdk.dir=$ANDROID_HOME" > "$BUILD/local.properties"

cd "$BUILD"
gradle --no-daemon assembleRelease 2>&1 | tail -20

# 从 build.gradle.kts 读真实版本号，用于产物命名
VN=$(awk -F'"' '/versionName = /{print $2; exit}' "$BUILD/app/build.gradle.kts")
VC=$(awk -F'= *' '/versionCode = /{print $2; exit}' "$BUILD/app/build.gradle.kts" | tr -d ' ')

APK="$BUILD/app/build/outputs/apk/release/app-release.apk"
OUT="$SRC/族谱v${VN}-release.apk"
cp -f "$APK" "$OUT"

# 清掉同名旧包，避免目录里堆一串
find "$SRC" -maxdepth 1 -name '族谱v*-release.apk' ! -name "族谱v${VN}-release.apk" -delete 2>/dev/null || true

echo
echo "✅ 编译完成：$OUT"
echo "   版本：v${VN} (versionCode ${VC})  大小：$(stat -c%s "$OUT") 字节"
