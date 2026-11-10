package com.zupu.app

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.net.Uri
import okhttp3.Request
import org.json.JSONObject

/** App 在线更新：读网关上的 app-latest.json，比对 versionCode；有新版就下载安装包。 */
object AppUpdate {

    // 与族谱更新网关同一个域名
    private const val MANIFEST = "https://zupu-update.lijiacore.com:4440/app/app-latest.json"

    data class Info(
        val versionCode: Int,
        val versionName: String,
        val url: String,
        val notes: String,
        val size: Long
    )

    // 网关用的是自签证书（族谱自己的更新地址也设了 INSECURE），安卓默认不信任 →
    // 这里单独给更新检查/下载用一个「宽松」客户端，其它请求不受影响。
    private val insecureClient: okhttp3.OkHttpClient by lazy {
        val tm = object : javax.net.ssl.X509TrustManager {
            override fun checkClientTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
            override fun checkServerTrusted(chain: Array<out java.security.cert.X509Certificate>?, authType: String?) {}
            override fun getAcceptedIssuers(): Array<java.security.cert.X509Certificate> = emptyArray()
        }
        val ctx = javax.net.ssl.SSLContext.getInstance("TLS")
        ctx.init(null, arrayOf<javax.net.ssl.TrustManager>(tm), java.security.SecureRandom())
        okhttp3.OkHttpClient.Builder()
            .sslSocketFactory(ctx.socketFactory, tm)
            .hostnameVerifier { _, _ -> true }
            .connectTimeout(8, java.util.concurrent.TimeUnit.SECONDS)
            .readTimeout(20, java.util.concurrent.TimeUnit.SECONDS)
            .build()
    }

    fun check(onResult: (Info?, String?) -> Unit) {
        Thread {
            var err: String? = null
            val info = runCatching {
                val req = Request.Builder().url(MANIFEST).build()
                insecureClient.newCall(req).execute().use { r ->
                    if (!r.isSuccessful) return@use null
                    val s = r.body?.string() ?: return@use null
                    val o = JSONObject(s)
                    Info(
                        o.optInt("versionCode", 0),
                        o.optString("versionName"),
                        o.optString("url"),
                        o.optString("notes"),
                        o.optLong("size", 0)
                    )
                }
            }.onFailure { err = it.message ?: it.javaClass.simpleName }.getOrNull()
            onResult(info, err)
        }.start()
    }

    /** 弹更新提示；用户确认后交给系统浏览器下载安装（无需文件权限） */
    fun prompt(activity: Activity, info: Info) {
        val cur = runCatching {
            activity.packageManager.getPackageInfo(activity.packageName, 0).versionCode
        }.getOrDefault(0)

        if (info.versionCode <= cur) {
            AlertDialog.Builder(activity)
                .setTitle("检查更新")
                .setMessage("已是最新版本（${info.versionName}）")
                .setPositiveButton("好", null)
                .show()
            return
        }

        val sizeMb = if (info.size > 0) String.format("%.1f MB", info.size / 1048576.0) else ""
        AlertDialog.Builder(activity)
            .setTitle("发现新版本 ${info.versionName}")
            .setMessage(buildString {
                if (sizeMb.isNotEmpty()) append("安装包大小：$sizeMb\n\n")
                if (info.notes.isNotBlank()) append(info.notes)
            })
            .setNegativeButton("以后再说", null)
            .setPositiveButton("下载更新") { _, _ -> downloadAndInstall(activity, info) }
            .show()
    }

    /** App 内下载 + 进度 + 直接安装（FileProvider 暴露 URI，无需文件权限） */
    fun downloadAndInstall(activity: Activity, info: Info) {
        val dir = java.io.File(activity.externalCacheDir ?: activity.cacheDir, "updates")
        if (!dir.exists()) dir.mkdirs()
        val apk = java.io.File(dir, "zupu-${info.versionName}.apk")

        val progress = android.app.ProgressDialog(activity).apply {
            setTitle("正在下载更新")
            setMessage("0%")
            setProgressStyle(android.app.ProgressDialog.STYLE_HORIZONTAL)
            max = 100
            setCancelable(false)
            show()
        }

        Thread {
            try {
                val req = Request.Builder().url(info.url).build()
                insecureClient.newCall(req).execute().use { resp ->
                    if (!resp.isSuccessful) throw IllegalStateException("HTTP ${resp.code}")
                    val body = resp.body ?: throw IllegalStateException("空响应")
                    val total = body.contentLength()
                    body.byteStream().use { input ->
                        java.io.FileOutputStream(apk).use { output ->
                            val buf = ByteArray(64 * 1024)
                            var read: Int
                            var done = 0L
                            var lastPct = -1
                            while (input.read(buf).also { read = it } > 0) {
                                output.write(buf, 0, read)
                                done += read
                                if (total > 0) {
                                    val pct = (done * 100 / total).toInt()
                                    if (pct != lastPct) {
                                        lastPct = pct
                                        activity.runOnUiThread { progress.progress = pct; progress.setMessage("$pct%") }
                                    }
                                }
                            }
                        }
                    }
                }
                activity.runOnUiThread {
                    progress.dismiss()
                    install(activity, apk)
                }
            } catch (e: Exception) {
                activity.runOnUiThread {
                    progress.dismiss()
                    AlertDialog.Builder(activity).setTitle("下载失败").setMessage(e.message ?: e.javaClass.simpleName)
                        .setPositiveButton("好", null).show()
                }
            }
        }.start()
    }

    /** 拉起系统安装器 */
    private fun install(activity: Activity, apk: java.io.File) {
        if (!activity.packageManager.canRequestPackageInstalls()) {
            AlertDialog.Builder(activity)
                .setTitle("需要安装权限")
                .setMessage("请允许「安装未知应用」，然后重新点检查更新。")
                .setPositiveButton("去设置") { _, _ ->
                    runCatching {
                        activity.startActivity(
                            Intent(android.provider.Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                                Uri.parse("package:" + activity.packageName))
                        )
                    }
                }
                .setNegativeButton("取消", null)
                .show()
            return
        }
        val uri: Uri = androidx.core.content.FileProvider.getUriForFile(
            activity, activity.packageName + ".fileprovider", apk
        )
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        activity.startActivity(intent)
    }

    /**
     * 清理下载目录：安装完成后（当前版本 == 该 APK 的版本）就删掉，
     * 其它残留超过 1 天的也删。避免每更新一次多占 6~7 MB。
     */
    fun cleanup(activity: Activity) {
        Thread {
            runCatching {
                val dir = java.io.File(activity.externalCacheDir ?: activity.cacheDir, "updates")
                if (!dir.exists()) return@runCatching
                val curVer = activity.packageManager.getPackageInfo(activity.packageName, 0).versionName
                val now = System.currentTimeMillis()
                dir.listFiles()?.forEach { f ->
                    val installed = f.name.contains(curVer ?: "")
                    val stale = now - f.lastModified() > 24L * 3600 * 1000
                    if (installed || stale) f.delete()
                }
            }
        }.start()
    }
}
