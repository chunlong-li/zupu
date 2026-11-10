package com.zupu.app

import android.os.Bundle
import android.widget.ArrayAdapter
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.zupu.app.databinding.ActivitySettingsBinding

/** APP 设置：更新 + 操作习惯（返回键行为） */
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
            AppUpdate.check { info, err ->
                runOnUiThread {
                    if (info != null) AppUpdate.prompt(this, info)
                    else Toast.makeText(this, "检查更新失败：" + (err ?: "清单为空"), Toast.LENGTH_LONG).show()
                }
            }
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
}
