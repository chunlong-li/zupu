package com.zupu.app

import android.content.DialogInterface
import android.os.Bundle
import android.text.InputType
import android.widget.ArrayAdapter
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.zupu.app.databinding.ActivitySettingsBinding

/** APP 设置：更新 + 电脑同步 + 操作习惯（返回键行为） */
class AppSettingsActivity : AppCompatActivity() {

    private lateinit var b: ActivitySettingsBinding

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        b = ActivitySettingsBinding.inflate(layoutInflater)
        setContentView(b.root)

        b.btnBack.setOnClickListener { finish() }

        runCatching {
            b.tvVersion.text = packageManager.getPackageInfo(packageName, 0).versionName
        }

        b.rowUpdate.setOnClickListener {
            AppUpdate.check(stableOnly = true) { info, err ->
                runOnUiThread {
                    if (info != null) AppUpdate.prompt(this, info)
                    else Toast.makeText(this, "检查更新失败：" + (err ?: "清单为空"), Toast.LENGTH_LONG).show()
                }
            }
        }

        // ── 电脑同步（被控端）──
        updateRemoteState()
        // 改为「发现局域网设备」流程：扫描到被控电脑后点它输入被控码
        // 「被控/连接电脑」功能暂时关闭：整行隐藏
        // 「被控/连接电脑」功能暂时关闭：整行隐藏
        b.rowRemote.visibility = android.view.View.GONE

        b.rowRemote.setOnClickListener {
            if (Prefs.remoteOn(this)) {
                // 已连上：弹原来的对话框，里面有「开启同步」勾选，取消勾选+保存即可断开
                showRemoteDialog()
            } else {
                startActivity(android.content.Intent(this, DiscoveryActivity::class.java))
            }
        }
        // 长按这一行：直接断开控制模式
        b.rowRemote.setOnLongClickListener {
            if (Prefs.remoteOn(this)) {
                Prefs.setRemote(this, Prefs.remoteHost(this), Prefs.remotePin(this), false)
                updateRemoteState()
                Toast.makeText(this, "已断开控制模式", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, "当前没有连接电脑", Toast.LENGTH_SHORT).show()
            }
            true
        }

        // 返回键行为：按操作习惯偏好回显
        val options = arrayOf("后退优先", "直接退出")
        val modes = arrayOf("web", "exit")
        b.spBackMode.adapter = ArrayAdapter(this, android.R.layout.simple_spinner_dropdown_item, options)
        b.spBackMode.setSelection(if (Prefs.backMode(this) == "exit") 1 else 0)
        b.spBackMode.setOnItemSelectedListener(object : android.widget.AdapterView.OnItemSelectedListener {
            override fun onItemSelected(parent: android.widget.AdapterView<*>?, view: android.view.View?, position: Int, id: Long) {
                Prefs.setBackMode(this@AppSettingsActivity, modes[position])
            }
            override fun onNothingSelected(parent: android.widget.AdapterView<*>?) {}
        })
    }

    override fun onResume() {
        super.onResume()
        updateRemoteState()
    }

    private fun updateRemoteState() {
        val on = Prefs.remoteOn(this)
        val host = Prefs.remoteHost(this)
        b.tvRemoteState.text = when {
            !on -> "未开启"
            host.isBlank() -> "未配置"
            else -> "已开启 · " + host
        }
    }

    /** 填写电脑 IP:端口 + 配对码 */
    private fun showRemoteDialog() {
        val pad = (16 * resources.displayMetrics.density).toInt()
        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad, pad / 2, pad, 0)
        }

        val tip = TextView(this).apply {
            text = "在电脑客户端「连接设置 → 允许被控」里能看到 IP、端口和配对码"
            textSize = 12f
            setTextColor(0xFF6B7280.toInt())
            setPadding(0, 0, 0, pad / 2)
        }

        val etHost = EditText(this).apply {
            hint = "电脑地址，如 192.168.1.13:47810"
            setText(Prefs.remoteHost(this@AppSettingsActivity))
            setSingleLine(true)
            inputType = InputType.TYPE_CLASS_TEXT
        }

        val etPin = EditText(this).apply {
            hint = "配对码（6 位数字）"
            setText(Prefs.remotePin(this@AppSettingsActivity))
            setSingleLine(true)
            inputType = InputType.TYPE_CLASS_NUMBER
        }

        val cbOn = CheckBox(this).apply {
            text = "开启同步（横屏，操作同步到电脑）"
            isChecked = Prefs.remoteOn(this@AppSettingsActivity)
        }

        box.addView(tip)
        box.addView(etHost)
        box.addView(etPin)
        box.addView(cbOn)

        val dialog = AlertDialog.Builder(this)
            .setTitle("连接电脑")
            .setView(box)
            .setPositiveButton("保存", null)
            .setNegativeButton("取消", null)
            .create()

        // 自己接管点击，校验不通过时不关闭弹窗
        dialog.setOnShowListener {
            dialog.getButton(DialogInterface.BUTTON_POSITIVE).setOnClickListener {
                val host = etHost.text.toString().trim()
                val pin = etPin.text.toString().trim()
                if (cbOn.isChecked) {
                    if (host.isBlank() || pin.isBlank()) {
                        Toast.makeText(this, "开启同步需要填地址和配对码", Toast.LENGTH_SHORT).show()
                        return@setOnClickListener
                    }
                    if (!host.contains(":")) {
                        Toast.makeText(this, "地址要带端口，例如 192.168.1.13:47810", Toast.LENGTH_LONG).show()
                        return@setOnClickListener
                    }
                }
                Prefs.setRemote(this, host, pin, cbOn.isChecked)
                updateRemoteState()
                Toast.makeText(
                    this,
                    if (cbOn.isChecked) "已开启，返回主界面后生效" else "已关闭同步",
                    Toast.LENGTH_SHORT
                ).show()
                dialog.dismiss()
            }
        }
        dialog.show()
    }
}
