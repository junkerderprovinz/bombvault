package bombvault.halleluja.design

import android.annotation.SuppressLint
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.res.Configuration
import android.graphics.Color
import android.net.Uri
import android.net.http.SslError
import android.os.Bundle
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.graphics.ColorUtils
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.lifecycle.lifecycleScope
import androidx.webkit.JavaScriptReplyProxy
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewCompat
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import javax.net.ssl.SSLException

/**
 * The app's one screen: the launcher, and the BombVault it opens, in one
 * WebView. The launcher comes from the app's assets; a server's own interface
 * is the one its browser shows.
 */
class MainActivity : ComponentActivity() {
    private lateinit var root: FrameLayout
    private lateinit var web: WebView
    private lateinit var servers: Servers
    private lateinit var discovery: Discovery
    private lateinit var loader: WebViewAssetLoader

    /** The server on screen, null while the launcher is. */
    private var current: Server? = null
    private var found: List<Found> = emptyList()

    /** Why the last server did not open, shown once by the launcher. */
    private var problem: JSONObject? = null

    private var launcher: JavaScriptReplyProxy? = null
    private var forgetHistory = false
    private var chooser: ValueCallback<Array<Uri>>? = null

    private val pickFiles = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        chooser?.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(result.resultCode, result.data))
        chooser = null
    }

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        servers = Servers(this)
        discovery = Discovery(this) {
            found = it
            pushState()
        }
        loader = WebViewAssetLoader.Builder()
            .addPathHandler("/", LauncherAssets(WebViewAssetLoader.AssetsPathHandler(this)))
            .build()

        web = WebView(this)
        root = FrameLayout(this)
        root.addView(web)
        setContentView(root)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        ViewCompat.setOnApplyWindowInsetsListener(root) { v, insets ->
            val bars = insets.getInsets(
                WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout() or WindowInsetsCompat.Type.ime(),
            )
            v.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            WindowInsetsCompat.CONSUMED
        }
        paintBars(if (night()) DARK else LIGHT)

        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.settings.allowFileAccess = false
        web.settings.allowContentAccess = false
        web.webViewClient = Client()
        web.webChromeClient = Chrome()
        web.setDownloadListener { url, _, disposition, mime, _ -> download(url, disposition, mime) }

        // Registered for every origin because the list of servers changes while
        // a page is open; onMessage decides who may ask for what.
        WebViewCompat.addWebMessageListener(web, "bombvaultApp", setOf("*")) { _, message, origin, isMainFrame, reply ->
            if (isMainFrame) onMessage(JSONObject(message.data ?: "{}"), origin.toString().trimEnd('/'), reply)
        }
        WebViewCompat.addDocumentStartJavaScript(web, assets.open("page.js").bufferedReader().readText(), setOf("*"))

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                when {
                    web.canGoBack() -> web.goBack()
                    current != null -> showLauncher()
                    else -> finish()
                }
            }
        })

        val restored = savedInstanceState?.let { web.restoreState(it) } != null
        if (restored) {
            current = web.url?.let { servers.forOrigin(originOf(it)) }
        } else {
            showLauncher()
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        web.saveState(outState)
    }

    override fun onResume() {
        super.onResume()
        if (current == null) discovery.start()
    }

    override fun onPause() {
        super.onPause()
        discovery.stop()
        CookieManager.getInstance().flush()
    }

    private fun showLauncher() {
        current = null
        forgetHistory = true
        web.loadUrl(LAUNCHER_URL)
        discovery.start()
    }

    private fun open(server: Server) {
        discovery.stop()
        current = server
        problem = null
        web.loadUrl(server.url)
    }

    private fun onMessage(msg: JSONObject, origin: String, reply: JavaScriptReplyProxy) {
        if (origin == LAUNCHER_ORIGIN) {
            launcher = reply
            fromLauncher(msg)
            return
        }
        val server = servers.forOrigin(origin) ?: return
        when (msg.optString("op")) {
            "file" -> saveFile(msg.getString("name"), msg.optString("type").ifEmpty { null }, msg.getString("data"))
            "bars" -> parseColor(msg.optString("color"))?.let { if (server.id == current?.id) paintBars(it) }
        }
    }

    private fun fromLauncher(msg: JSONObject) {
        when (msg.optString("op")) {
            "state" -> pushState()
            "save" -> {
                val s = msg.getJSONObject("server")
                val saved = servers.save(s.optString("id").ifEmpty { null }, s.getString("name"), s.getString("url"))
                if (msg.optBoolean("open")) open(saved) else pushState()
            }
            "remove" -> {
                servers.remove(msg.getString("id"))
                pushState()
            }
            "open" -> servers.get(msg.getString("id"))?.let(::open)
            "trust" -> {
                servers.trust(msg.getString("id"), msg.getString("fingerprint"))
                // The WebView remembers the refusal for the host otherwise.
                web.clearSslPreferences()
                servers.get(msg.getString("id"))?.let(::open)
            }
            "dismiss" -> {
                problem = null
                pushState()
            }
            "fetch" -> {
                val server = servers.get(msg.getString("id")) ?: return
                val ticket = msg.getInt("ticket")
                lifecycleScope.launch {
                    val answer = withContext(Dispatchers.IO) { get(server, msg.getString("path")) }
                    launcher?.postMessage(
                        JSONObject().put("op", "fetched").put("ticket", ticket).put("status", answer.status).put("body", answer.body).toString(),
                    )
                }
            }
        }
    }

    private fun pushState() {
        val state = JSONObject()
            .put("op", "state")
            .put("servers", JSONArray(servers.all().map { it.toJson() }))
            .put(
                "found",
                JSONArray(found.map { JSONObject().put("name", it.name).put("url", it.url).put("version", it.version) }),
            )
        problem?.let { state.put("problem", it) }
        launcher?.postMessage(state.toString())
    }

    /** Goes back to the launcher, which tells the person why [server] did not open. */
    private fun fail(server: Server, kind: String, fingerprint: String? = null, detail: String? = null) {
        problem = JSONObject().put("id", server.id).put("kind", kind).apply {
            fingerprint?.let { put("fingerprint", it) }
            detail?.let { put("detail", it) }
        }
        showLauncher()
    }

    private fun download(url: String, disposition: String?, mime: String?) {
        // blob: links never reach here alive; page.js saves those.
        val server = servers.forOrigin(originOf(url)) ?: return
        lifecycleScope.launch {
            try {
                val name = withContext(Dispatchers.IO) { Downloads.fromUrl(this@MainActivity, server, url, disposition, mime) }
                toast(getString(R.string.saved, name))
            } catch (_: SSLException) {
                toast(getString(R.string.save_untrusted))
            } catch (e: IOException) {
                toast(getString(R.string.save_failed, e.message ?: e.javaClass.simpleName))
            }
        }
    }

    private fun saveFile(name: String, mime: String?, base64: String) {
        lifecycleScope.launch {
            try {
                withContext(Dispatchers.IO) { Downloads.fromPage(this@MainActivity, name, mime, base64) }
                toast(getString(R.string.saved, name))
            } catch (e: IOException) {
                toast(getString(R.string.save_failed, e.message ?: e.javaClass.simpleName))
            }
        }
    }

    private fun toast(text: String) = Toast.makeText(this, text, Toast.LENGTH_LONG).show()

    private fun openElsewhere(uri: Uri) {
        try {
            startActivity(Intent(Intent.ACTION_VIEW, uri))
        } catch (_: ActivityNotFoundException) {
            toast(getString(R.string.no_app))
        }
    }

    /** The system bars sit on [color], with icons that read on it. */
    private fun paintBars(color: Int) {
        root.setBackgroundColor(color)
        val light = ColorUtils.calculateLuminance(color) > 0.5
        WindowCompat.getInsetsController(window, root).apply {
            isAppearanceLightStatusBars = light
            isAppearanceLightNavigationBars = light
        }
    }

    private fun night() =
        resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES

    private inner class Client : WebViewClient() {
        override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? =
            loader.shouldInterceptRequest(request.url)

        override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
            val uri = request.url
            if (uri.scheme != "http" && uri.scheme != "https") {
                openElsewhere(uri)
                return true
            }
            val origin = originOf(uri.toString())
            if (origin == LAUNCHER_ORIGIN) return false
            // A link to another saved server stays in the app, anything else
            // (GitHub, donations, documentation) opens in the browser.
            if (servers.forOrigin(origin) != null) return false
            openElsewhere(uri)
            return true
        }

        // Every navigation passes here, Back included, so this is where the app
        // learns which page is on screen.
        override fun doUpdateVisitedHistory(view: WebView, url: String, isReload: Boolean) {
            if (url.startsWith(LAUNCHER_ORIGIN)) {
                if (current != null) {
                    current = null
                    discovery.start()
                    paintBars(if (night()) DARK else LIGHT)
                }
            } else {
                current = servers.forOrigin(originOf(url)) ?: current
            }
        }

        override fun onPageFinished(view: WebView, url: String) {
            if (forgetHistory && url.startsWith(LAUNCHER_ORIGIN)) {
                forgetHistory = false
                // The launcher is the bottom of the stack: Back from it leaves.
                web.clearHistory()
                paintBars(if (night()) DARK else LIGHT)
            }
        }

        @SuppressLint("WebViewClientOnReceivedSslError")
        override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
            val server = servers.forOrigin(originOf(error.url))
            val cert = error.certificate.x509Certificate
            val print = cert?.let(::fingerprint)
            if (server != null && print != null && server.pin == print) {
                handler.proceed()
                return
            }
            handler.cancel()
            if (server != null && print != null && server.id == current?.id) {
                fail(server, if (server.pin == null) "untrusted" else "changed", fingerprint = print)
            }
        }

        override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
            val server = current ?: return
            if (request.isForMainFrame && originOf(request.url.toString()) == server.origin) {
                fail(server, "unreachable", detail = error.description.toString())
            }
        }
    }

    private inner class Chrome : WebChromeClient() {
        override fun onShowFileChooser(
            webView: WebView,
            callback: ValueCallback<Array<Uri>>,
            params: FileChooserParams,
        ): Boolean {
            chooser?.onReceiveValue(null)
            chooser = callback
            try {
                pickFiles.launch(params.createIntent())
            } catch (_: ActivityNotFoundException) {
                chooser = null
                return false
            }
            return true
        }
    }

    /** Serves the launcher bundle as the root of its origin. */
    private class LauncherAssets(private val assets: WebViewAssetLoader.AssetsPathHandler) : WebViewAssetLoader.PathHandler {
        override fun handle(path: String): WebResourceResponse? = assets.handle("launcher/$path")
    }

    companion object {
        private const val LAUNCHER_ORIGIN = "https://${WebViewAssetLoader.DEFAULT_DOMAIN}"
        private const val LAUNCHER_URL = "$LAUNCHER_ORIGIN/launcher.html"

        // The ground of BombVault's dark and light themes, the same values its
        // theme-color meta carries.
        private val DARK = Color.parseColor("#161616")
        private val LIGHT = Color.parseColor("#f4f4f4")

        private fun parseColor(s: String): Int? = runCatching { Color.parseColor(s) }.getOrNull()
    }
}
