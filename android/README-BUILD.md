# 族谱 Android 客户端（源码）

登录 + WebView 套壳：原生顶栏、可折叠侧边菜单、mDNS 局域网发现、App 内在线更新。

## 构建

需要 JDK 17 + Android SDK 34（本机工具链在 `/vol1/@appshare/dsh/toolchain`）：

```bash
cd android
bash rebuild.sh        # 产出 族谱v<版本>-release.apk
```

> ⚠️ 必须在**纯 ASCII 路径**下编译（脚本会自动复制到 `/tmp/zupu-app-build`），
> 中文路径会让 aapt 报 `values.xml 不存在`。

## 签名

`keystore/` 内有固定 release 证书（**不随仓库分发**，见 `.gitignore`）。
换证书会导致已安装用户「签名不一致」必须卸载重装，切勿更换。

## 发版

版本号规则与族谱系统一致：正式版 `1.0.1`，测试版 `1.0.1.1110.2`。

```bash
cd android
sed -i 's/versionCode = 12/versionCode = 13/; s/versionName = "1.0.1"/versionName = "1.0.2"/' app/build.gradle.kts
bash rebuild.sh
bash /vol1/@appshare/dsh/dsh_home/skills/zupu-app-release/scripts/publish-app.sh 1.0.2 13 "更新说明"
```

## 目录

```
app/src/main/java/com/zupu/app/
  LoginActivity.kt     登录页（地址历史下拉 / 记住密码 / mDNS 发现入口）
  DiscoveryActivity.kt 局域网发现（NsdManager，_zupu._tcp）
  MainActivity.kt      原生顶栏 + 可折叠抽屉 + WebView + 退出
  AppSettingsActivity.kt  APP 设置（检查更新 / 返回键行为）
  AppUpdate.kt         在线更新（自签证书兼容 + 内置下载 + 清理）
  NsdDiscovery.kt      mDNS 封装
  Net.kt / Prefs.kt    接口与本地存储
```
