package com.zupu.app

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.ArrayAdapter
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.zupu.app.databinding.ActivityDiscoveryBinding
import kotlinx.coroutines.launch

/**
 * 局域网发现：先 mDNS（_zupu._tcp，秒回），再扫 8080/8888/8090 兜底。
 * mDNS 需要服务端也广播（族谱 1.4.0+ 自带；容器安装需 host 网络）。
 */
class DiscoveryActivity : AppCompatActivity() {

    private lateinit var b: ActivityDiscoveryBinding
    private var nsd: NsdDiscovery? = null
    private val items = ArrayList<String>()
    private val endpoints = ArrayList<String>()
    private lateinit var adapter: ArrayAdapter<String>
    private val mdns = LinkedHashMap<String, String>()   // endpoint -> label
    private var remotes: List<RemoteDiscovery.Device> = emptyList()   // 被控电脑

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        b = ActivityDiscoveryBinding.inflate(layoutInflater)
        setContentView(b.root)

        adapter = ArrayAdapter(this, android.R.layout.simple_list_item_1, items)
        b.list.adapter = adapter
        b.btnBack.setOnClickListener { finish() }
        b.list.setOnItemClickListener { _, _, position, _ ->
            // 前半部分是被控电脑，后面才是族谱服务
            if (position < remotes.size) {
                askPin(remotes[position])
                return@setOnItemClickListener
            }
            val addr = endpoints.getOrNull(position - remotes.size) ?: return@setOnItemClickListener
            setResult(Activity.RESULT_OK, Intent().putExtra("address", addr))
            finish()
        }

        b.tvHint.text = "正在通过 mDNS 查找族谱服务…"
        nsd = NsdDiscovery(this).also {
            it.start { list ->
                runOnUiThread {
                    mdns.clear()
                    list.forEach { f -> mdns[f.endpoint] = f.label }
                    render(hint = "mDNS 找到 ${list.size} 个（继续扫描端口…）")
                }
            }
        }

        // 持续监听被控电脑的广播（每 2 秒一条），扫到一台就立刻显示，不等扫描窗口结束
        b.tvHint.text = "正在扫描局域网里的被控电脑…（电脑开启被控后会自动出现）"
        lifecycleScope.launch {
            RemoteDiscovery.listen { dev ->
                runOnUiThread {
                    if (remotes.none { it.key == dev.key }) {
                        remotes = remotes + dev
                        render(hint = "找到 " + remotes.size + " 台电脑，点一台输入被控码即可控制")
                    }
                }
            }
        }

        lifecycleScope.launch {
            val scanned = Net.discoverLan { snap ->
                scanResult = snap
                render(hint = "已找到 ${allEndpoints().size} 个族谱服务")
            }
            scanResult = scanned
            b.pb.visibility = View.GONE
            render(hint = if (allEndpoints().isEmpty())
                "没找到族谱服务。\n请确认：① 同一 Wi-Fi；② 服务端已开启 mDNS（族谱 1.4.0+，容器需 host 网络）；③ 或手动填 IP:端口"
            else "共找到 ${allEndpoints().size} 个族谱服务，点一个填入登录页")
        }
    }

    private var scanResult: List<String> = emptyList()

    private fun allEndpoints(): List<String> =
        (mdns.keys + scanResult).distinct().sorted()

    private fun render(hint: String) {
        val all = allEndpoints()
        items.clear(); endpoints.clear()
        // 被控电脑优先
        remotes.forEach { d ->
            items.add("💻 " + d.name + "（被控端）\n" + d.host + ":" + d.port + " · 点击输入被控码")
        }
        all.forEach { ep ->
            val label = mdns[ep]
            items.add(if (label.isNullOrBlank()) ep else "$label\n$ep")
            endpoints.add(ep)
        }
        adapter.notifyDataSetChanged()
        b.tvHint.text = hint
    }

    /** 输入被控码 → 记住并直接以被控模式进入 */
    private fun askPin(dev: RemoteDiscovery.Device) {
        val input = android.widget.EditText(this).apply {
            hint = "6 位被控码"
            inputType = android.text.InputType.TYPE_CLASS_NUMBER
            setPadding(48, 32, 48, 32)
        }
        com.google.android.material.dialog.MaterialAlertDialogBuilder(this)
            .setTitle("连接 " + dev.name)
            .setMessage(dev.host + ":" + dev.port + "\n被控码显示在电脑端「开启被控」里")
            .setView(input)
            .setPositiveButton("连接") { _, _ ->
                val pin = input.text?.toString()?.trim().orEmpty()
                if (pin.isEmpty()) {
                    android.widget.Toast.makeText(this, "请填写被控码", android.widget.Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }
                Prefs.setRemote(this, dev.host, pin, true)
                startActivity(Intent(this, MainActivity::class.java))
                finish()
            }
            .setNegativeButton("取消", null)
            .show()
    }

    override fun onDestroy() {
        nsd?.stop()
        super.onDestroy()
    }
}
