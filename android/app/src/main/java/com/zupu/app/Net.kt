package com.zupu.app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.concurrent.TimeUnit

/** 登录结果：token + 用户信息 JSON（网页端就是靠 localStorage.user 判断身份的） */
data class LoginResult(val token: String, val userJson: String, val userName: String, val uid: Long)

object Net {
    val client: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .build()

    private val fastClient = OkHttpClient.Builder()
        .connectTimeout(300, TimeUnit.MILLISECONDS)
        .readTimeout(800, TimeUnit.MILLISECONDS)
        .build()

    fun normalize(input: String): String {
        var s = input.trim()
        if (s.isEmpty()) return ""
        if (!s.startsWith("http://") && !s.startsWith("https://")) s = "http://$s"
        return s.trimEnd('/')
    }

    fun login(baseUrl: String, user: String, pass: String): LoginResult {
        val body = JSONObject().put("username", user).put("password", pass)
            .toString().toRequestBody("application/json; charset=utf-8".toMediaType())
        val req = Request.Builder().url("$baseUrl/api/auth/login").post(body).build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string() ?: ""
            if (!resp.isSuccessful) {
                val msg = runCatching { JSONObject(text).optString("error") }.getOrNull()
                throw IllegalStateException(msg?.takeIf { it.isNotBlank() } ?: "登录失败（HTTP ${resp.code}）")
            }
            val json = JSONObject(text)
            val token = json.optString("token")
            if (token.isBlank()) throw IllegalStateException("登录失败：没有返回 token")
            val userObj = json.optJSONObject("user")
            return LoginResult(
                token = token,
                userJson = userObj?.toString() ?: "{}",
                userName = userObj?.optString("username").orEmpty().ifBlank { user },
                uid = userObj?.optLong("id", 0L) ?: 0L
            )
        }
    }

    private fun probe(host: String, port: Int): String? = try {
        val req = Request.Builder().url("http://$host:$port/health").get().build()
        fastClient.newCall(req).execute().use { r ->
            val body = r.body?.string() ?: ""
            if (r.isSuccessful && body.contains("ok", ignoreCase = true)) "$host:$port" else null
        }
    } catch (e: Exception) {
        null
    }

    /** 快速扫描只扫 8080（族谱默认端口） */
    private val FAST_PORTS = intArrayOf(8080, 8888, 8090)

    /** 广播发现端口：族谱服务端（1.3.16+）在这个 UDP 端口上应答 */
    // 同一台宿主机可以跑多个实例（测试+正式），每个只能占一个「宿主机 UDP 端口」，
    // 所以 App 对几个候选端口都广播一次，谁在听谁应答。
    val DISCOVER_PORTS = intArrayOf(45789, 45790, 45791)
    private const val DISCOVER_MAGIC = "ZUPU_DISCOVER"

    /**
     * 广播发现：向 255.255.255.255:45789 喊一嗓子，收集 2 秒内的应答。
     * 应答格式（JSON）：{"name":"族谱","version":"1.3.16","port":8080}
     * 返回 host:port 列表。
     */
    suspend fun discoverBroadcast(timeoutMs: Long = 2000): List<String> = withContext(Dispatchers.IO) {
        val found = java.util.Collections.synchronizedList(ArrayList<String>())
        var socket: java.net.DatagramSocket? = null
        try {
            socket = java.net.DatagramSocket()
            socket.broadcast = true
            val payload = DISCOVER_MAGIC.toByteArray()
            val targets = arrayListOf(java.net.InetAddress.getByName("255.255.255.255"))
            localBroadcastAddresses().forEach { targets.add(it) }
            for (t in targets) {
                for (dp in DISCOVER_PORTS) {
                    runCatching {
                        socket.send(java.net.DatagramPacket(payload, payload.size, t, dp))
                    }
                }
            }
            socket.soTimeout = 300
            val deadline = System.currentTimeMillis() + timeoutMs
            val buf = ByteArray(1024)
            while (System.currentTimeMillis() < deadline) {
                val pkt = java.net.DatagramPacket(buf, buf.size)
                try {
                    socket.receive(pkt)
                } catch (e: java.net.SocketTimeoutException) {
                    continue
                } catch (e: Exception) {
                    break
                }
                val ip = pkt.address.hostAddress ?: continue
                val text = String(pkt.data, 0, pkt.length)
                val port = runCatching { JSONObject(text).optInt("port", 8080) }.getOrDefault(8080)
                val hit = "$ip:$port"
                if (!found.contains(hit)) found.add(hit)
            }
        } catch (e: Exception) {
            // 广播被系统限制（部分安卓需 MulticastLock）→ 返回空，走端口扫描兜底
        } finally {
            runCatching { socket?.close() }
        }
        sortFound(found)
    }

    /** 本机所在网段的定向广播地址（部分路由器不转发 255.255.255.255） */
    private fun localBroadcastAddresses(): List<java.net.InetAddress> {
        val out = ArrayList<java.net.InetAddress>()
        return try {
            java.net.NetworkInterface.getNetworkInterfaces().toList()
                .filter { it.isUp && !it.isLoopback }
                .flatMap { it.interfaceAddresses }
                .mapNotNull { it.broadcast }
                .filterIsInstance<java.net.Inet4Address>()
                .also { out.addAll(it) }
            out
        } catch (e: Exception) {
            emptyList()
        }
    }

    /** 快速扫描：只扫当前 /24 网段的 8080 端口 */
    suspend fun discoverLan(onFound: (List<String>) -> Unit = {}): List<String> = withContext(Dispatchers.IO) {
        val prefix = localIpv4Prefix() ?: return@withContext emptyList()
        val found = java.util.Collections.synchronizedList(ArrayList<String>())
        coroutineScope {
            (1..254).map { i ->
                async {
                    val host = "$prefix.$i"
                    for (port in FAST_PORTS) {
                        val hit = probe(host, port) ?: continue
                        if (found.contains(hit)) continue
                        found.add(hit)
                        val snap = sortFound(found.toList())
                        withContext(Dispatchers.Main) { onFound(snap) }
                    }
                }
            }.awaitAll()
        }
        sortFound(found.toList())
    }

    /** 按 IP、再按端口排序 */
    fun sortFound(list: List<String>): List<String> = list.sortedWith(
        compareBy({ hostNum(it) }, { it.substringAfterLast(":").toIntOrNull() ?: 0 })
    )

    private fun hostNum(addr: String): Long {
        val ip = addr.substringBeforeLast(":")
        val p = ip.split(".")
        if (p.size != 4) return 0
        return p.fold(0L) { acc, x -> acc * 256 + (x.toIntOrNull() ?: 0) }
    }

    private fun localIpv4Prefix(): String? = try {
        java.net.NetworkInterface.getNetworkInterfaces().toList()
            .filter { it.isUp && !it.isLoopback }
            .flatMap { it.inetAddresses.toList() }
            .filterIsInstance<java.net.Inet4Address>()
            .map { it.hostAddress ?: "" }
            .firstOrNull { it.startsWith("192.168.") || it.startsWith("10.") || it.startsWith("172.") }
            ?.substringBeforeLast('.')
    } catch (e: Exception) {
        null
    }
}
