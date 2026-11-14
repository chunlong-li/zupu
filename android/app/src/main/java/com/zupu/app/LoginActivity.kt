package com.zupu.app

import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.zupu.app.databinding.ActivityLoginBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class LoginActivity : AppCompatActivity() {

    private lateinit var b: ActivityLoginBinding

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        b = ActivityLoginBinding.inflate(layoutInflater)
        setContentView(b.root)

        // 回显上次填的地址/账号/密码
        b.etAddress.setText(Prefs.address(this))
        b.etUser.setText(Prefs.user(this))
        b.etPass.setText(Prefs.pass(this))
        b.cbRemember.isChecked = Prefs.remember(this)
        if (b.etAddress.text.isNullOrBlank()) b.etAddress.setText("192.168.1.190:8080")
        setupAddressDropdown()

        b.btnLogin.setOnClickListener { doLogin() }

        // 已登录过：直接进（token 还在就不必重填）
        if (Prefs.token(this).isNotBlank() && Prefs.address(this).isNotBlank()) {
            openWeb()
        }

        b.tvDiscover.setOnClickListener { discover() }
    }

    /** 地址框：登录过的地址做下拉候选 */
    private fun setupAddressDropdown() {
        val hosts = Prefs.addresses(this).toMutableList()
        val current = b.etAddress.text?.toString()?.trim().orEmpty()
        if (current.isNotBlank() && hosts.none { it == current }) hosts.add(0, current)
        if (hosts.size <= 1) return
        b.etAddress.setAdapter(
            android.widget.ArrayAdapter(this, android.R.layout.simple_list_item_1, hosts)
        )
        b.etAddress.setOnClickListener { b.etAddress.showDropDown() }
        b.etAddress.setOnFocusChangeListener { _, has -> if (has) b.etAddress.showDropDown() }
    }

    private fun doLogin() {
        val address = Net.normalize(b.etAddress.text?.toString() ?: "")
        val user = b.etUser.text?.toString()?.trim() ?: ""
        val pass = b.etPass.text?.toString() ?: ""
        if (address.isEmpty()) return err("请填服务器地址")
        if (user.isEmpty()) return err("请填账号")
        if (pass.isEmpty()) return err("请填密码")

        setBusy(true)
        lifecycleScope.launch {
            try {
                val r = withContext(Dispatchers.IO) { Net.login(address, user, pass) }
                Prefs.saveLogin(this@LoginActivity, address, user, pass, b.cbRemember.isChecked)
                Prefs.addAddress(this@LoginActivity, address)
                Prefs.saveToken(this@LoginActivity, r.token, r.userName, "", r.userJson)
                Prefs.saveUid(this@LoginActivity, r.uid)
                runCatching {
                    val role = org.json.JSONObject(r.userJson).optString("role")
                    Prefs.setRoleName(this@LoginActivity, role)
                }
                openWeb()
            } catch (e: Exception) {
                err(e.message ?: "登录失败")
            } finally {
                setBusy(false)
            }
        }
    }

    private fun openWeb() {
        startActivity(Intent(this, MainActivity::class.java))
        finish()
    }

    private val discoveryLauncher = registerForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult()
    ) { result ->
        val addr = result.data?.getStringExtra("address").orEmpty()
        if (addr.isNotBlank()) {
            b.etAddress.setText(addr)
            Toast.makeText(this, "已填入：$addr", Toast.LENGTH_SHORT).show()
        }
        b.tvDiscover.isEnabled = true
        b.tvDiscover.text = getString(R.string.discover)
    }

    private fun discover() {
        b.tvDiscover.isEnabled = false
        b.tvDiscover.text = getString(R.string.discovering)
        discoveryLauncher.launch(Intent(this, DiscoveryActivity::class.java))
    }

    private fun setBusy(busy: Boolean) {
        b.progress.visibility = if (busy) View.VISIBLE else View.GONE
        b.btnLogin.isEnabled = !busy
        b.tvDiscover.isEnabled = !busy
        if (busy) b.tvError.visibility = View.GONE
    }

    private fun err(msg: String) {
        b.tvError.text = msg
        b.tvError.visibility = View.VISIBLE
    }
}
