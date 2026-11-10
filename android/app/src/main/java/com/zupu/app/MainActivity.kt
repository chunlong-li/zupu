package com.zupu.app

import android.annotation.SuppressLint
import android.content.Intent
import android.graphics.Bitmap
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.KeyEvent
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.zupu.app.databinding.ActivityMainBinding

class MainActivity : AppCompatActivity() {

    private lateinit var b: ActivityMainBinding
    private var injected = false
    private val ui = Handler(Looper.getMainLooper())

    private val tokenWatch = object : Runnable {
        override fun run() {
            if (!isFinishing && this@MainActivity::b.isInitialized) {
                b.webView.evaluateJavascript(
                    "(function(){try{return localStorage.getItem('token')||''}catch(e){return ''}})()"
                ) { v ->
                    val t = (v ?: "").trim('"')
                    if (t.isBlank() && Prefs.token(this@MainActivity).isNotBlank()) backToLogin()
                }
                ui.postDelayed(this, 1500)
            }
        }
    }

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        b = ActivityMainBinding.inflate(layoutInflater)
        setContentView(b.root)

        val base = Prefs.address(this)
        if (base.isBlank()) { backToLogin(); return }

        // ☰ 打开原生抽屉（不再借网页菜单）
        b.btnMenu.setOnClickListener { b.drawer.openDrawer(androidx.core.view.GravityCompat.START) }
        setupDrawer()
        b.tvAvatar.setOnClickListener { clickWeb(".user-avatar", "网页资料弹窗暂不可用") }
        b.tvBranch.setOnClickListener { pickBranch() }
        b.btnLogout.setOnClickListener { confirmLogout() }
        // 长按家族名 = 检查 App 更新（隐藏入口，不占界面）
        b.tvTitle.setOnLongClickListener {
            AppUpdate.check { info, err ->
                runOnUiThread {
                    if (info != null) AppUpdate.prompt(this, info)
                    else toast("检查更新失败：" + (err ?: "清单为空"))
                }
            }
            true
        }

        setupWebView(base)
        b.webView.loadUrl(base)
        // 上次更新下载的安装包若已安装完成 → 顺手删掉，不留垃圾
        AppUpdate.cleanup(this)
    }


    /** 可折叠抽屉：分组默认收起 */
    private fun setupDrawer() {
        val D = DrawerAdapter::class.java.declaredClasses[0]
        val groups = listOf(
            DrawerAdapter.Group("常用", listOf(
                DrawerAdapter.Item("👥 成员管理", "/members"),
                DrawerAdapter.Item("🏡 房支管理", "/branches"),
                DrawerAdapter.Item("📈 数据看板", "/dashboard")
            )),
            DrawerAdapter.Group("📊 数据看板", listOf(
                DrawerAdapter.Item("🎂 生日", "/birthday"),
                DrawerAdapter.Item("🕯️ 祭日", "/deathday"),
                DrawerAdapter.Item("🎏 传统日程", "/traditions"),
                DrawerAdapter.Item("🗺️ 热力分布", "/heatmap"),
                DrawerAdapter.Item("🚶 迁徙记录", "/migrations"),
                DrawerAdapter.Item("📇 通讯录", "/contacts")
            )),
            DrawerAdapter.Group("🌳 世系可视化", listOf(
                DrawerAdapter.Item("🏯 宝塔式", "/tree"),
                DrawerAdapter.Item("📜 纯文字连线", "/texttree"),
                DrawerAdapter.Item("🎐 苏式垂吊图", "/sutree"),
                DrawerAdapter.Item("🕸️ 苏式垂丝图", "/susi"),
                DrawerAdapter.Item("📿 苏氏垂珠体", "/suzhu"),
                DrawerAdapter.Item("🪜 世代分层图", "/gentree"),
                DrawerAdapter.Item("📃 竖排牒记式", "/demo-dieji"),
                DrawerAdapter.Item("📄 横版牒记式", "/demo-dieji-land"),
                DrawerAdapter.Item("📕 谱书封面", "/cover-design"),
                DrawerAdapter.Item("🌲 传代树", "/test-zupu")
            )),
            DrawerAdapter.Group("🏛️ 谱头文化", listOf(
                DrawerAdapter.Item("🗓️ 家族大事记", "/culture/events"),
                DrawerAdapter.Item("✒️ 谱序", "/culture/articles"),
                DrawerAdapter.Item("👤 名人传记", "/culture/bios")
            )),
            DrawerAdapter.Group("🏮 祠墓", listOf(
                DrawerAdapter.Item("🛕 祠堂", "/hall"),
                DrawerAdapter.Item("🏯 祠宇志", "/shrines"),
                DrawerAdapter.Item("🪦 茔墓志", "/graves")
            )),
            DrawerAdapter.Group("更多", listOf(
                DrawerAdapter.Item("📜 字辈派语", "/generation"),
                DrawerAdapter.Item("💍 配偶资料", "/spouses-list"),
                DrawerAdapter.Item("📚 家族知识库", "/knowledge"),
                DrawerAdapter.Item("💰 收支公示", "/accounts"),
                DrawerAdapter.Item("🪦 功德碑", "/merit")
            )),
            DrawerAdapter.Group("⚙️ 系统设置", listOf(
                DrawerAdapter.Item("🏷️ 家族设置", "/settings"),
                DrawerAdapter.Item("🩺 数据体检", "/data-check"),
                DrawerAdapter.Item("💾 备份管理", "/backup"),
                DrawerAdapter.Item("⬆️ 版本更新", "/update"),
                DrawerAdapter.Item("📖 使用说明", "/usage-guide")
            )),
            // 顶层菜单项：没有子项，点标题直接打开 APP 设置
            DrawerAdapter.Group("📱 APP 设置", emptyList())
        )
        val adapter = DrawerAdapter(this, groups)
        b.nav.setAdapter(adapter)
        b.nav.setOnGroupClickListener { _, _, groupPos, _ ->
            if (adapter.getChildrenCount(groupPos) == 0) {
                b.drawer.closeDrawers()
                startActivity(android.content.Intent(this, AppSettingsActivity::class.java))
                true
            } else false
        }
        b.nav.setOnChildClickListener { _, _, g, c, _ ->
            b.drawer.closeDrawers()
            val item = adapter.getChild(g, c) as DrawerAdapter.Item
            if (item.tag == "appsettings") {
                startActivity(android.content.Intent(this, AppSettingsActivity::class.java))
            } else if (item.tag == "appupdate") {
                AppUpdate.check { info, err ->
                    runOnUiThread {
                        if (info != null) AppUpdate.prompt(this, info)
                        else toast("检查更新失败：" + (err ?: "清单为空"))
                    }
                }
            } else item.route?.let { navigate(it) }
            true
        }
    }

    /** 原生菜单 → 网页路由（与网页侧边栏一一对应） */
    private val routes = mapOf(
        R.id.nav_members to "/members",
        R.id.nav_branches to "/branches",
        R.id.nav_dashboard to "/dashboard",
        R.id.nav_birthday to "/birthday",
        R.id.nav_deathday to "/deathday",
        R.id.nav_traditions to "/traditions",
        R.id.nav_heatmap to "/heatmap",
        R.id.nav_migrations to "/migrations",
        R.id.nav_contacts to "/contacts",
        R.id.nav_tree to "/tree",
        R.id.nav_texttree to "/texttree",
        R.id.nav_sutree to "/sutree",
        R.id.nav_susi to "/susi",
        R.id.nav_suzhu to "/suzhu",
        R.id.nav_gentree to "/gentree",
        R.id.nav_dieji to "/demo-dieji",
        R.id.nav_dieji_land to "/demo-dieji-land",
        R.id.nav_cover to "/cover-design",
        R.id.nav_chuandai to "/test-zupu",
        R.id.nav_events to "/culture/events",
        R.id.nav_articles to "/culture/articles",
        R.id.nav_bios to "/culture/bios",
        R.id.nav_hall to "/hall",
        R.id.nav_shrines to "/shrines",
        R.id.nav_graves to "/graves",
        R.id.nav_generation to "/generation",
        R.id.nav_spouses to "/spouses-list",
        R.id.nav_knowledge to "/knowledge",
        R.id.nav_accounts to "/accounts",
        R.id.nav_merit to "/merit",
        R.id.nav_settings to "/settings",
        R.id.nav_datacheck to "/data-check",
        R.id.nav_backup to "/backup",
        R.id.nav_update to "/update",
        R.id.nav_guide to "/usage-guide"
    )

    /** 切到网页的某个路由：网页是 history 模式，必须走 /路径 而不是 #/路径 */
    private fun navigate(route: String) {
        val base = Prefs.address(this)
        // 首选：网页若暴露了 router（后续版本），直接 push，不重载
        val js = """
            (function(){try{
              if(window.__zupuRouter){window.__zupuRouter.push('ROUTE');return 'ok'}
              return 'no'
            }catch(e){return 'no'}})()
        """.trimIndent().replace("ROUTE", route)
        b.webView.evaluateJavascript(js) { r ->
            if (!(r ?: "").contains("ok")) {
                // 兜底：history 模式直接加载完整 URL
                b.webView.loadUrl(base + route)
            }
        }
    }

    private fun clickWeb(selector: String, failMsg: String) {
        b.webView.evaluateJavascript(
            "(function(){var e=document.querySelector('$selector');if(e){e.click();return 'ok'}return 'no'})()"
        ) { r -> if ((r ?: "").contains("no")) toast(failMsg) }
    }

    private fun setupWebView(base: String) {
        val token = Prefs.token(this)
        val userJson = Prefs.userJson(this).ifBlank { "{}" }
        val userName = Prefs.tokenUser(this)

        b.webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            loadWithOverviewMode = true
            useWideViewPort = true
        }
        b.webView.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                b.webProgress.progress = newProgress
                b.webProgress.visibility = if (newProgress >= 100) View.GONE else View.VISIBLE
            }
        }
        b.webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                if (!injected && token.isNotBlank()) {
                    injected = true
                    // 每个账号只弹一次「赞助作者」：没看过→清标记让它弹；看过→种标记压掉
                    val uid = Prefs.uid(this@MainActivity)
                    val seen = uid > 0L && Prefs.sponsorSeen(this@MainActivity, uid)
                    val sponsorJs = if (seen) {
                        "localStorage.setItem('zupu_sponsor_shown','1');"
                    } else {
                        "localStorage.removeItem('zupu_sponsor_shown');"
                    }
                    if (uid > 0L) Prefs.setSponsorSeen(this@MainActivity, uid)
                    val js = """
                        (function(){try{
                          localStorage.setItem('token', ${jsStr(token)});
                          localStorage.setItem('user', ${jsStr(userJson)});
                          localStorage.setItem('username', ${jsStr(userName)});
                          ${sponsorJs}
                        }catch(e){}})();
                    """.trimIndent()
                    view?.evaluateJavascript(js, null)
                    view?.postDelayed({ view.reload() }, 200)
                    return
                }
                // 隐藏网页自带顶栏，并把家族名/头像带回原生顶栏
                view?.evaluateJavascript(
                    """
                    (function(){
                      try{
                        if(!document.getElementById('zupu-app-css')){
                          var st=document.createElement('style');
                          st.id='zupu-app-css';
                          st.textContent='.topbar{display:none!important}.body-area{padding-top:0!important}';
                          document.head.appendChild(st);
                        }
                        var logo=document.querySelector('.topbar .logo');
                        var av=document.querySelector('.user-avatar');
                        return JSON.stringify({
                          title: logo?logo.innerText.trim():'',
                          avatar: av?av.innerText.trim():''
                        });
                      }catch(e){return '{}'}
                    })();
                    """.trimIndent()
                ) { raw ->
                    runCatching {
                        val s = (raw ?: "").trim('"').replace("\\\"", "\"")
                        val o = org.json.JSONObject(s)
                        o.optString("title").takeIf { it.isNotBlank() }?.let { b.tvTitle.text = it }
                        o.optString("avatar").takeIf { it.isNotBlank() }?.let { b.tvAvatar.text = it.take(1) }
                    }
                }
                syncBranchLabel()
            }

            override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
                val u = request?.url?.toString() ?: return false
                if (u.contains("/login")) { backToLogin(); return true }
                return if (!u.startsWith(base)) {
                    runCatching { startActivity(Intent(Intent.ACTION_VIEW, request!!.url)) }
                    true
                } else false
            }

            override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                super.onPageStarted(view, url, favicon)
                if (url != null && url.contains("/login")) backToLogin()
            }
        }
    }

    // ===== 房支 =====

    private fun pickBranch() {
        Thread {
            val list = runCatching { fetchBranches() }.getOrDefault(emptyList())
            runOnUiThread {
                if (list.isEmpty()) { toast("没有房支数据"); return@runOnUiThread }
                val names = ArrayList<String>()
                val ids = ArrayList<Long>()
                names.add(getString(R.string.all_books)); ids.add(0L)
                for ((id, meta) in list) {
                    val (label, level) = meta
                    names.add("　".repeat((level - 1).coerceAtLeast(0)) + label)
                    ids.add(id)
                }
                b.webView.evaluateJavascript(
                    "(function(){try{var u=localStorage.getItem('branchCtx:u" + Prefs.uid(this) + "');" +
                    "if(u){var o=JSON.parse(u);if(Number(o.v)===2)return String(o.current||0)}" +
                    "var g=JSON.parse(localStorage.getItem('branchCtx')||'{}');return String(g.current||0)}catch(e){return '0'}})()"
                ) { cur ->
                    val current = (cur ?: "0").trim('"').toLongOrNull() ?: 0L
                    val checked = ids.indexOf(current).coerceAtLeast(0)
                    AlertDialog.Builder(this)
                        .setTitle("选择房支")
                        .setSingleChoiceItems(names.toTypedArray(), checked) { d, which ->
                            d.dismiss()
                            applyBranch(ids[which], names[which].trim())
                        }
                        .setNegativeButton("取消", null)
                        .show()
                }
            }
        }.start()
    }

    private fun applyBranch(id: Long, label: String) {
        val title = if (id == 0L) getString(R.string.all_books) else label
        // 首选：调用网页暴露的 __zupuSetBranch（不刷新页面，数据自己变）
        b.webView.evaluateJavascript(
            "(function(){try{if(typeof window.__zupuSetBranch==='function'){window.__zupuSetBranch(BRID);return 'ok'}return 'no'}catch(e){return 'no'}})()"
                .replace("BRID", id.toString())
        ) { r ->
            if ((r ?: "").contains("ok")) {
                b.tvBranch.text = title
                return@evaluateJavascript
            }
            // 兜底：老版本网页没有这个函数 → 写 localStorage 再刷新
            fallbackApplyBranch(id, title)
        }
    }

    private fun fallbackApplyBranch(id: Long, title: String) {
        b.tvBranch.text = title
        val uid = Prefs.uid(this)
        val js = """
            (function(){try{
              var uid=UIDV;
              if(!uid){try{uid=Number((JSON.parse(localStorage.getItem('user')||'{}')).id)||0}catch(e){}}
              var inc=true;
              try{var old=JSON.parse(localStorage.getItem('branchCtx')||'{}');if(typeof old.includeChildren==='boolean')inc=old.includeChildren;}catch(e){}
              var mirror={current:BRV, includeChildren:inc};
              if(uid>0) mirror.uid=uid;
              localStorage.setItem('branchCtx', JSON.stringify(mirror));
              if(uid>0){localStorage.setItem('branchCtx:u'+uid, JSON.stringify({v:2, current:BRV, includeChildren:inc, picked:true}));}
              return 'uid='+uid+' cur=BRV';
            }catch(e){return 'err:'+e}})();
        """.trimIndent().replace("UIDV", uid.toString()).replace("BRV", id.toString())
        b.webView.evaluateJavascript(js) { r ->
            val real = Regex("uid=(\\d+)").find(r ?: "")?.groupValues?.get(1)?.toLongOrNull() ?: 0L
            if (real > 0L && Prefs.uid(this) != real) Prefs.saveUid(this, real)
        }
        b.webView.postDelayed({ b.webView.reload() }, 200)
    }

    private fun syncBranchLabel() {
        val uid = Prefs.uid(this)
        b.webView.evaluateJavascript(
            "(function(){try{var u=localStorage.getItem('branchCtx:u" + uid + "');" +
            "if(u){var o=JSON.parse(u);if(Number(o.v)===2)return String(o.current||0)}" +
            "var g=JSON.parse(localStorage.getItem('branchCtx')||'{}');return String(g.current||0)}catch(e){return '0'}})()"
        ) { cur ->
            val id = (cur ?: "0").trim('"').toLongOrNull() ?: 0L
            if (id == 0L) { b.tvBranch.text = getString(R.string.all_books); return@evaluateJavascript }
            Thread {
                val list = runCatching { fetchBranches() }.getOrDefault(emptyList())
                runOnUiThread {
                    val hit = list.firstOrNull { it.first == id }
                    b.tvBranch.text = hit?.second?.first ?: getString(R.string.all_books)
                }
            }.start()
        }
    }

    private fun fetchBranches(): List<Pair<Long, Pair<String, Int>>> {
        val req = okhttp3.Request.Builder()
            .url("${Prefs.address(this)}/api/branches")
            .header("Authorization", "Bearer ${Prefs.token(this)}")
            .build()
        Net.client.newCall(req).execute().use { resp ->
            val text = resp.body?.string() ?: return emptyList()
            val arr = org.json.JSONArray(text)
            val out = ArrayList<Pair<Long, Pair<String, Int>>>()
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                val name = o.optString("name")
                val sub = o.optString("subtitle")
                val label = if (sub.isBlank()) name else "$name（$sub）"
                out.add(o.optLong("id") to (label to o.optInt("level", 1)))
            }
            return out
        }
    }

    // ===== 退出 =====

    private fun confirmLogout() {
        AlertDialog.Builder(this)
            .setTitle(R.string.menu_logout)
            .setMessage(R.string.logout_confirm)
            .setNegativeButton(R.string.cancel, null)
            .setPositiveButton(R.string.menu_logout) { _, _ ->
                runCatching {
                    val req = okhttp3.Request.Builder()
                        .url("${Prefs.address(this)}/api/auth/logout")
                        .header("Authorization", "Bearer ${Prefs.token(this)}")
                        .post(okhttp3.RequestBody.create(null, ByteArray(0)))
                        .build()
                    Net.client.newCall(req).execute().close()
                }
                b.webView.evaluateJavascript(
                    "try{localStorage.removeItem('token');localStorage.removeItem('user')}catch(e){}", null
                )
                backToLogin()
            }
            .show()
    }

    override fun onResume() {
        super.onResume()
        ui.postDelayed(tokenWatch, 1500)
        if (this::b.isInitialized) b.webView.postDelayed({ syncBranchLabel() }, 800)
    }

    override fun onPause() { super.onPause(); ui.removeCallbacks(tokenWatch) }

    private fun toast(msg: String) = Toast.makeText(this, msg, Toast.LENGTH_SHORT).show()

    private fun jsStr(s: String): String =
        "'" + s.replace("\\", "\\\\").replace("'", "\\'").replace("\n", " ") + "'"

    private fun backToLogin() {
        ui.removeCallbacks(tokenWatch)
        Prefs.clearToken(this)
        startActivity(Intent(this, LoginActivity::class.java))
        finish()
    }

    private var exitArmed = false

    override fun onKeyDown(keyCode: Int, event: KeyEvent?): Boolean {
        if (keyCode != KeyEvent.KEYCODE_BACK) return super.onKeyDown(keyCode, event)
        if (this::b.isInitialized && b.drawer.isDrawerOpen(androidx.core.view.GravityCompat.START)) {
            b.drawer.closeDrawers(); return true
        }
        when (Prefs.backMode(this)) {
            "exit" -> {
                if (!exitArmed) {
                    exitArmed = true
                    toast("再按一次返回键退出应用")
                    ui.postDelayed({ exitArmed = false }, 2000)
                    return true
                }
                finishAffinity()
                return true
            }
            else -> if (this::b.isInitialized && b.webView.canGoBack()) {
                b.webView.goBack(); return true
            }
        }
        return super.onKeyDown(keyCode, event)
    }
}
