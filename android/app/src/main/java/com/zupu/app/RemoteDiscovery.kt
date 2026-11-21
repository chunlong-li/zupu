package com.zupu.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.SocketTimeoutException

/**
 * 发现局域网里的「被控电脑」：电脑开启被控后每 2 秒向 47820 端口广播一次。
 * 这里持续监听（不是固定 3 秒窗口），扫到一台就立刻回调，页面销毁时协程取消会自动关掉 socket。
 */
object RemoteDiscovery {

    const val PORT = 47820

    data class Device(val name: String, val host: String, val port: Int) {
        val key: String get() = "$host:$port"
    }

    /**
     * 持续监听，每发现一台新电脑就回调一次（在主线程回调）。
     * 传入的 onDevice 需要自己去重。
     */
    suspend fun listen(onDevice: (Device) -> Unit) = withContext(Dispatchers.IO) {
        var sock: DatagramSocket? = null
        try {
            sock = DatagramSocket(null).apply {
                reuseAddress = true
                broadcast = true
                bind(java.net.InetSocketAddress(PORT))
                soTimeout = 700
            }
        } catch (e: Exception) {
            // 端口被占用（比如同时开了两次扫描）→ 直接返回
            return@withContext
        }
        try {
            val buf = ByteArray(2048)
            while (true) {
                try {
                    val pkt = DatagramPacket(buf, buf.size)
                    sock.receive(pkt)
                    val text = String(pkt.data, 0, pkt.length, Charsets.UTF_8)
                    val o = runCatching { JSONObject(text) }.getOrNull() ?: continue
                    if (o.optString("app") != "zupu") continue
                    if (o.optString("role") != "remote") continue
                    val host = o.optString("host").ifBlank { pkt.address?.hostAddress ?: "" }
                    val port = o.optInt("port", 47810)
                    if (host.isBlank() || port <= 0) continue
                    val name = o.optString("name").ifBlank { "电脑 " + host }
                    onDevice(Device(name, host, port))
                } catch (e: SocketTimeoutException) {
                    // 继续等
                } catch (e: Exception) {
                    break
                }
            }
        } finally {
            runCatching { sock?.close() }
        }
    }
}
