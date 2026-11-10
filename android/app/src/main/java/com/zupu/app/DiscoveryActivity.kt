package com.zupu.app

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.ArrayAdapter
import androidx.appcompat.app.AppCompatActivity
import com.zupu.app.databinding.ActivityDiscoveryBinding

/**
 * 局域网发现：只用标准 mDNS（_zupu._tcp），由宿主机 mDNS 旁车广播。
 * 不再做 UDP 广播/端口扫描兜底——测试期这样可以一眼看出 mDNS 是否真的通了。
 */
class DiscoveryActivity : AppCompatActivity() {

    private lateinit var b: ActivityDiscoveryBinding
    private var nsd: NsdDiscovery? = null
    private val items = ArrayList<String>()
    private lateinit var adapter: ArrayAdapter<String>
    private val found = ArrayList<NsdDiscovery.Found>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        b = ActivityDiscoveryBinding.inflate(layoutInflater)
        setContentView(b.root)

        adapter = ArrayAdapter(this, android.R.layout.simple_list_item_2, android.R.id.text1, items)
        b.list.adapter = adapter

        b.btnBack.setOnClickListener { finish() }
        b.list.setOnItemClickListener { _, _, position, _ ->
            val f = found.getOrNull(position) ?: return@setOnItemClickListener
            setResult(Activity.RESULT_OK, Intent().putExtra("address", f.endpoint))
            finish()
        }

        b.tvHint.text = "正在通过 mDNS 查找族谱服务…"
        nsd = NsdDiscovery(this).also {
            it.start { list ->
                runOnUiThread {
                    found.clear(); found.addAll(list)
                    items.clear()
                    list.forEach { f -> items.add("${f.label}\n${f.endpoint}") }
                    adapter.notifyDataSetChanged()
                    b.tvHint.text = if (list.isEmpty())
                        "正在通过 mDNS 查找族谱服务…"
                    else
                        "找到 ${list.size} 个族谱服务，点一个填入登录页"
                }
            }
        }

        // 6 秒后如果还没有结果，给出明确提示（不做端口扫描兜底）
        b.list.postDelayed({
            if (found.isEmpty()) {
                b.pb.visibility = View.GONE
                b.tvHint.text = "没找到族谱服务。\n请确认：① 手机与服务器同一 Wi-Fi；② 服务器上 mDNS 旁车容器在运行。"
            }
        }, 6000)
    }

    override fun onDestroy() {
        nsd?.stop()
        super.onDestroy()
    }
}
