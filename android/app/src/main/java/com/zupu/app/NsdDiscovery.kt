package com.zupu.app

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo

/**
 * 用 Android 标准 mDNS（NsdManager）发现族谱服务：服务类型 _zupu._tcp。
 * 由宿主机上的 mDNS 旁车（zupu-mdns）广播，address/port 直接可连。
 */
class NsdDiscovery(private val ctx: Context) {

    data class Found(val address: String, val port: Int, val label: String) {
        val endpoint: String get() = "$address:$port"
    }

    private val nsd: NsdManager = ctx.getSystemService(Context.NSD_SERVICE) as NsdManager
    private var listener: NsdManager.DiscoveryListener? = null
    private val resolved = LinkedHashMap<String, Found>()

    fun start(onChange: (List<Found>) -> Unit) {
        val l = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(serviceType: String) {}

            override fun onServiceFound(info: NsdServiceInfo) {
                if (info.serviceType != "_zupu._tcp." && info.serviceType != "_zupu._tcp") return
                nsd.resolveService(info, object : NsdManager.ResolveListener {
                    override fun onResolveFailed(serviceInfo: NsdServiceInfo, errorCode: Int) {}

                    override fun onServiceResolved(serviceInfo: NsdServiceInfo) {
                        val host = serviceInfo.host?.hostAddress ?: return
                        val port = serviceInfo.port
                        if (port <= 0) return
                        val label = try {
                            serviceInfo.attributes?.get("name")?.toString(Charsets.UTF_8)
                        } catch (e: Throwable) {
                            null
                        } ?: serviceInfo.serviceName
                        resolved["$host:$port"] = Found(host, port, label)
                        onChange(resolved.values.sortedBy { it.endpoint })
                    }
                })
            }

            override fun onServiceLost(info: NsdServiceInfo) {
                resolved.remove(info.host?.hostAddress + ":" + info.port)
                onChange(resolved.values.toList())
            }

            override fun onDiscoveryStopped(serviceType: String) {}
            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
                onChange(emptyList())
            }
            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {}
        }
        listener = l
        runCatching { nsd.discoverServices("_zupu._tcp", NsdManager.PROTOCOL_DNS_SD, l) }
    }

    fun stop() {
        listener?.let { runCatching { nsd.stopServiceDiscovery(it) } }
        listener = null
    }
}
