package bombvault.halleluja.design

import android.webkit.CookieManager
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.SocketTimeoutException
import java.net.URL
import java.security.KeyStore
import java.security.MessageDigest
import java.security.cert.Certificate
import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLException
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

/** The SHA-256 of [cert], as colon-separated hex the way browsers print it. */
fun fingerprint(cert: Certificate): String =
    MessageDigest.getInstance("SHA-256").digest(cert.encoded).joinToString(":") { "%02X".format(it) }

/**
 * Opens [url] on [server] the way the WebView would: with its session cookie,
 * and accepting the certificate the person trusted for it as well as every one
 * Android trusts.
 */
fun open(server: Server, url: String): HttpURLConnection {
    val conn = URL(url).openConnection() as HttpURLConnection
    conn.connectTimeout = TIMEOUT_MS
    conn.readTimeout = TIMEOUT_MS
    CookieManager.getInstance().getCookie(url)?.let { conn.setRequestProperty("Cookie", it) }
    val pin = server.pin
    if (conn is HttpsURLConnection && pin != null) {
        val ctx = SSLContext.getInstance("TLS")
        ctx.init(null, arrayOf(PinnedTrust(pin)), null)
        conn.sslSocketFactory = ctx.socketFactory
        val usual = conn.hostnameVerifier
        // A self-signed certificate rarely names the address it is reached by.
        conn.hostnameVerifier = HostnameVerifier { host, session ->
            fingerprint(session.peerCertificates[0]) == pin || usual.verify(host, session)
        }
    }
    return conn
}

/**
 * A server's answer to a GET. Status 0 means it was not reached, and -1 that
 * it presented a certificate the app does not trust for it.
 */
class Answer(val status: Int, val body: String)

/** GETs [path] from [server]. */
fun get(server: Server, path: String): Answer = try {
    val conn = open(server, server.url.trimEnd('/') + path)
    conn.instanceFollowRedirects = false
    val status = conn.responseCode
    Answer(status, (if (status >= 400) conn.errorStream else conn.inputStream)?.let(::readUntilQuiet) ?: "")
} catch (e: SSLException) {
    Answer(-1, e.message ?: e.javaClass.simpleName)
} catch (e: IOException) {
    Answer(0, e.message ?: e.javaClass.simpleName)
}

/**
 * Reads [stream] to its end, or to the first pause longer than the read
 * timeout. An older server keeps the progress stream open after its snapshot,
 * and what came before the pause is the answer.
 */
private fun readUntilQuiet(stream: InputStream): String {
    val out = StringBuilder()
    stream.bufferedReader().use { reader ->
        val buf = CharArray(8192)
        try {
            while (true) {
                val n = reader.read(buf)
                if (n < 0) break
                out.append(buf, 0, n)
            }
        } catch (_: SocketTimeoutException) {
        }
    }
    return out.toString()
}

private class PinnedTrust(private val pin: String) : X509TrustManager {
    private val system = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        .apply { init(null as KeyStore?) }
        .trustManagers.filterIsInstance<X509TrustManager>().first()

    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) =
        system.checkClientTrusted(chain, authType)

    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
        if (fingerprint(chain[0]) != pin) system.checkServerTrusted(chain, authType)
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = system.acceptedIssuers
}

private const val TIMEOUT_MS = 5000

/**
 * What runs on [server], read over HTTP with the session, in the shape the
 * group's activity route answers in: runs, the progress snapshot as events,
 * and the next scheduled runs.
 */
fun activityOverHttp(server: Server): Answer {
    val runs = get(server, "/api/runs")
    if (runs.status != 200) return runs
    val live = get(server, "/api/progress?snapshot=1")
    val next = get(server, "/api/schedule/next")
    return try {
        val progress = JSONArray()
        if (live.status == 200) {
            for (line in live.body.lines()) if (line.startsWith("data: ")) progress.put(JSONObject(line.removePrefix("data: ")))
        }
        val upcoming = if (next.status == 200) JSONObject(next.body).optJSONArray("runs") ?: JSONArray() else JSONArray()
        val out = JSONObject()
            .put("ok", true)
            .put("runs", JSONObject(runs.body).optJSONArray("runs") ?: JSONArray())
            .put("progress", progress)
            .put("next", upcoming)
        Answer(200, out.toString())
    } catch (e: JSONException) {
        // Something answered that is not BombVault, such as a proxy's error page.
        Answer(0, e.message ?: "not BombVault")
    }
}
