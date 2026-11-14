package com.zupu.app

import android.content.Context

/** 记住的服务器地址 / 账号 / 密码 / 上次登录拿到的 token */
object Prefs {
    private const val F = "zupu"
    private fun sp(c: Context) = c.getSharedPreferences(F, Context.MODE_PRIVATE)

    fun address(c: Context) = sp(c).getString("address", "") ?: ""

    fun addresses(c: Context): List<String> =
        (sp(c).getString("addresses", "") ?: "").split("\n").filter { it.isNotBlank() }

    fun addAddress(c: Context, addr: String) {
        if (addr.isBlank()) return
        val list = addresses(c).toMutableList()
        list.remove(addr)
        list.add(0, addr)
        sp(c).edit().putString("addresses", list.take(8).joinToString("\n")).apply()
    }
    fun user(c: Context) = sp(c).getString("user", "") ?: ""
    fun pass(c: Context) = sp(c).getString("pass", "") ?: ""
    fun remember(c: Context) = sp(c).getBoolean("remember", true)
    fun token(c: Context) = sp(c).getString("token", "") ?: ""
    fun tokenUser(c: Context) = sp(c).getString("tokenUser", "") ?: ""
    fun role(c: Context) = sp(c).getString("role", "") ?: ""
    fun userJson(c: Context) = sp(c).getString("userJson", "") ?: ""

    /** 登录时后端返回的角色（超级管理员 / 理事长 / 族员 …） */
    fun roleName(c: Context) = sp(c).getString("roleName", "") ?: ""
    fun setRoleName(c: Context, r: String) { sp(c).edit().putString("roleName", r).apply() }
    fun uid(c: Context) = sp(c).getLong("uid", 0L)

    /** 返回键行为：web = 网页内后退（默认）；exit = 再按一次退出 App */
    fun backMode(c: Context) = sp(c).getString("backMode", "web") ?: "web"
    fun setBackMode(c: Context, v: String) { sp(c).edit().putString("backMode", v).apply() }

    fun saveLogin(c: Context, address: String, user: String, pass: String, remember: Boolean) {
        sp(c).edit().putString("address", address).putString("user", user)
            .putString("pass", if (remember) pass else "").putBoolean("remember", remember).apply()
    }

    fun saveToken(c: Context, token: String, user: String, role: String, userJson: String = "") {
        sp(c).edit().putString("token", token).putString("tokenUser", user)
            .putString("role", role).putString("userJson", userJson).apply()
    }

    fun saveUid(c: Context, uid: Long) { sp(c).edit().putLong("uid", uid).apply() }

    /** 该账号是否已经看过「赞助作者」弹窗（每个账号各一次） */
    fun sponsorSeen(c: Context, uid: Long) = sp(c).getBoolean("sponsorSeen:$uid", false)
    fun setSponsorSeen(c: Context, uid: Long) {
        sp(c).edit().putBoolean("sponsorSeen:$uid", true).apply()
    }

    fun clearToken(c: Context) {
        sp(c).edit().remove("token").remove("tokenUser").remove("role").apply()
    }
}
