# 族谱 Android 客户端

一个「登录 + 网页套壳」的安卓 App：登录页仿飞牛 App（地址 / 账号 / 密码 / 记住密码 / 局域网发现），
登录成功后用 WebView 打开族谱系统网页（自动注入 token，免二次登录）。

## 界面

| 元素 | 说明 |
|---|---|
| 第 1 格 | 服务器地址，可只填 `192.168.1.190:8080`（自动补 `http://`） |
| 第 2 格 | 账号 |
| 第 3 格 | 密码（右侧眼睛可切换明文） |
| 记住密码 | 勾选后本地保存密码；取消只保存地址和账号 |
| 登录 | 调 `POST /api/auth/login`，成功存 token 并进 WebView |
| 发现局域网内设备 | 扫当前 /24 网段的 8080 端口，命中 `/health` 的即是族谱服务；多个会弹列表选择 |

## 原生顶栏与退出（v1.0.1）

WebView 上方加了一条**原生顶栏**（不依赖网页），右上角菜单（⋮）提供：

| 菜单项 | 作用 |
|---|---|
| 返回首页 | 回到 `http://地址` |
| 刷新 | 重载当前页 |
| 用浏览器打开 | 交给系统浏览器 |
| **退出登录** | 二次确认 → 调 `/api/auth/logout` → 清本地 token → **回到原生登录页** |

退出/登录态同步做了三层兜底：

1. 网页里点「退出」（清掉 localStorage.token）→ App 每 1.5 秒检测一次，发现 token 空了就自动回登录页；
2. 网页跳到 `/login` 路由 → `shouldOverrideUrlLoading` 直接拦下回登录页；
3. App 自己的「退出登录」菜单 → 主动清本地 + 通知服务端。

登录态注入也修了：网页判断身份靠 `localStorage.token` + `localStorage.user`（JSON），
现在两个都写入，不会再出现「用户ID 0 / 变成族员」。

## 登录后的行为

- WebView 加载 `http://地址`，页面加载完成后把 `token`/`username` 写进 `localStorage` 再刷新一次，
  前端即认为已登录（族谱前端就读 `localStorage.token`）；
- 返回键优先在网页内后退；站外链接用系统浏览器打开；
- token 失效时回登录页（目前由服务端 401 触发，前端跳登录，再走一次登录即可）。

## 构建

需要 JDK 17 + Android SDK（compileSdk 34）：

```bash
# 方式一：Android Studio 直接打开本目录，Run
# 方式二：命令行
export ANDROID_HOME=/path/to/android-sdk
./gradlew assembleDebug          # 产物 app/build/outputs/apk/debug/app-debug.apk
```

> 本机（飞牛 NAS）没有 JDK / Android SDK，**这份代码未在本机构建过**，需要在装了
> Android Studio 的电脑上构建。Gradle wrapper 需要首次联网下载（`gradle wrapper` 或直接用 AS 打开）。

## 目录

```
app/src/main/java/com/zupu/app/
  LoginActivity.kt   登录页（含局域网发现）
  MainActivity.kt    WebView 容器 + token 注入
  Net.kt             登录、局域网扫描（OkHttp + 协程）
  Prefs.kt           记住密码 / token 存储（SharedPreferences）
app/src/main/res/layout/activity_login.xml
app/src/main/res/layout/activity_main.xml
app/src/main/res/xml/network_security_config.xml   # 允许 http 明文（内网 8080）
```

## 已知限制

- 未做「发现局域网」以外的设备配对（族谱服务没有 mDNS/SSDP 广播，只能扫端口）；
- 扫描范围固定 8080 端口、当前 /24 网段（家用网段够用；跨网段/改端口需在 `Net.discoverLan(port)` 里调整）；
- 未做 App 内更新（族谱是网页，更新网页即更新 App 内容）。
