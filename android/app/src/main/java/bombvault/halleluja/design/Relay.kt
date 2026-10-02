package bombvault.halleluja.design

import android.util.Base64
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.io.IOException
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import javax.crypto.AEADBadTagException
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec
import kotlin.math.abs

/** The project relay every BombVault instance dials unless its owner picks another. */
const val PROJECT_RELAY = "wss://parleyport.halleluja.design/relay/connect"

/** The keys a group secret gives, derived the way internal/relay derives them. */
class GroupKeys(secret: ByteArray) {
    val relayKey: String = hash("bombvault/relay/group-key/v1", secret).joinToString("") { "%02x".format(it) }
    val frameKey: ByteArray = hash("bombvault/relay/frame-key/v1", secret)

    private fun hash(domain: String, secret: ByteArray): ByteArray =
        MessageDigest.getInstance("SHA-256").run {
            update(domain.toByteArray())
            update(secret)
            digest()
        }
}

/** Another member of the group, as its sealed announce introduced it. */
data class Sibling(val id: String, val name: String, val version: String, val kind: String)

/**
 * Relay keeps the app's connection to the group's relay: it announces the app,
 * tracks the other members and carries sealed calls both ways. Every frame and
 * seal matches internal/relay, which is the other end.
 */
class Relay(
    url: String,
    private val keys: GroupKeys,
    private val id: String,
    private val identity: JSONObject,
    private val serve: (method: String, path: String) -> Pair<Int, ByteArray>,
    private val onChange: () -> Unit,
) {
    private val url = connectUrl(url)
    private val http = OkHttpClient.Builder().pingInterval(30, TimeUnit.SECONDS).readTimeout(0, TimeUnit.MILLISECONDS).build()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val pending = ConcurrentHashMap<String, CompletableDeferred<JSONObject>>()
    private val seen = ConcurrentHashMap<String, Long>()
    private val members = ConcurrentHashMap<String, Sibling>()

    @Volatile private var socket: WebSocket? = null
    @Volatile private var stopped = false
    private var backoff = MIN_BACKOFF_MS

    val connected: Boolean get() = socket != null
    val siblings: List<Sibling> get() = members.values.toList()

    fun start() = dial()

    fun stop() {
        stopped = true
        socket?.close(1000, null)
        scope.cancel()
    }

    /** Asks member [target] and returns its status and body. */
    suspend fun call(target: String, method: String, path: String): Pair<Int, ByteArray> {
        val ws = socket ?: throw IOException("relay not connected")
        val rid = randomHex(16)
        val call = JSONObject().put("method", method).put("path", path).put("id", rid).put("sent", System.currentTimeMillis() / 1000)
        val sealed = seal(keys.frameKey, "proxy-request\u0000$rid\u0000$target", call.toString().toByteArray())
        val answer = CompletableDeferred<JSONObject>()
        pending[rid] = answer
        try {
            ws.send(frame("proxy-request", JSONObject().put("requestId", rid).put("target", target).put("sealed", b64(sealed))))
            val resp = withTimeout(CALL_TIMEOUT_MS) { answer.await() }
            // The relay can write the error field itself, so it never counts as an answer.
            if (resp.has("error")) throw IOException("$target did not answer")
            val result = open(keys.frameKey, "proxy-response\u0000$rid", unb64(resp.optString("sealed")))
                ?: throw IOException("$target answered unreadably")
            val r = JSONObject(String(result))
            return r.getInt("status") to unb64(r.optString("body"))
        } finally {
            pending.remove(rid)
        }
    }

    private fun dial() {
        if (stopped) return
        http.newWebSocket(Request.Builder().url(url).build(), Listener())
    }

    private inner class Listener : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            val sealed = seal(keys.frameKey, "announce\u0000$id", identity.toString().toByteArray())
            val announce = JSONObject().put("instanceId", id).put("sealed", b64(sealed))
            webSocket.send(frame("hello", JSONObject().put("key", keys.relayKey).put("announce", announce)))
            socket = webSocket
            backoff = MIN_BACKOFF_MS
            onChange()
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            val env = runCatching { JSONObject(text) }.getOrNull() ?: return
            val data = env.optJSONObject("data") ?: return
            when (env.optString("type")) {
                "announce" -> announced(data)
                "presence" -> if (!data.optBoolean("online")) {
                    members.remove(data.optString("instanceId"))
                    onChange()
                }
                "proxy-response" -> pending.remove(data.optString("requestId"))?.complete(data)
                "proxy-request" -> scope.launch { answer(webSocket, data) }
            }
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = lost(webSocket)

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) = lost(webSocket)
    }

    private fun announced(data: JSONObject) {
        val sid = data.optString("instanceId")
        // An announce that does not open was not sealed by a group member.
        val plain = open(keys.frameKey, "announce\u0000$sid", unb64(data.optString("sealed"))) ?: return
        val who = runCatching { JSONObject(String(plain)) }.getOrNull() ?: return
        members[sid] = Sibling(sid, who.optString("name"), who.optString("version"), who.optString("kind"))
        onChange()
    }

    /** Runs one call a member made. A call that does not open, is old or ran before gets no reply. */
    private fun answer(ws: WebSocket, data: JSONObject) {
        val rid = data.optString("requestId")
        val plain = open(keys.frameKey, "proxy-request\u0000$rid\u0000$id", unb64(data.optString("sealed"))) ?: return
        val call = runCatching { JSONObject(String(plain)) }.getOrNull() ?: return
        if (call.optString("id") != rid || !admit(rid, call.optLong("sent"))) return
        val (status, body) = serve(call.optString("method"), call.optString("path"))
        val result = JSONObject().put("status", status).put("body", b64(body))
        val sealed = seal(keys.frameKey, "proxy-response\u0000$rid", result.toString().toByteArray())
        ws.send(frame("proxy-response", JSONObject().put("requestId", rid).put("sealed", b64(sealed))))
    }

    private fun admit(rid: String, sent: Long): Boolean {
        val now = System.currentTimeMillis() / 1000
        if (abs(now - sent) > CLOCK_SKEW_S) return false
        seen.entries.removeIf { now - it.value > 2 * CLOCK_SKEW_S }
        return seen.putIfAbsent(rid, now) == null
    }

    private fun lost(webSocket: WebSocket) {
        if (socket !== webSocket && socket != null) return
        socket = null
        members.clear()
        for (p in pending.values) p.completeExceptionally(IOException("relay connection dropped"))
        pending.clear()
        onChange()
        if (stopped) return
        val wait = backoff
        backoff = (backoff * 2).coerceAtMost(MAX_BACKOFF_MS)
        scope.launch {
            delay(wait)
            dial()
        }
    }

    companion object {
        private const val CALL_TIMEOUT_MS = 15_000L
        private const val CLOCK_SKEW_S = 120L
        private const val MIN_BACKOFF_MS = 1_000L
        private const val MAX_BACKOFF_MS = 60_000L
        private val random = SecureRandom()

        /** The WebSocket address for a relay given as http(s) or ws(s), with or without its path. */
        fun connectUrl(raw: String): String {
            var u = raw.trim().replaceFirst(Regex("^http://"), "ws://").replaceFirst(Regex("^https://"), "wss://")
            u = u.trimEnd('/')
            return if (u.endsWith("/connect")) u else "$u/relay/connect"
        }

        private fun frame(type: String, data: JSONObject) = JSONObject().put("type", type).put("data", data).toString()

        private fun randomHex(bytes: Int) = ByteArray(bytes).also(random::nextBytes).joinToString("") { "%02x".format(it) }

        // Go encodes a []byte field as padded standard base64.
        private fun b64(b: ByteArray) = Base64.encodeToString(b, Base64.NO_WRAP)

        private fun unb64(s: String): ByteArray = if (s.isEmpty()) ByteArray(0) else Base64.decode(s, Base64.DEFAULT)

        fun seal(key: ByteArray, aad: String, plain: ByteArray): ByteArray {
            val nonce = ByteArray(12).also(random::nextBytes)
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.ENCRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(128, nonce))
            c.updateAAD(aad.toByteArray())
            return nonce + c.doFinal(plain)
        }

        fun open(key: ByteArray, aad: String, sealed: ByteArray): ByteArray? {
            if (sealed.size < 12) return null
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(128, sealed.copyOf(12)))
            c.updateAAD(aad.toByteArray())
            return try {
                c.doFinal(sealed, 12, sealed.size - 12)
            } catch (_: AEADBadTagException) {
                null
            }
        }
    }
}
