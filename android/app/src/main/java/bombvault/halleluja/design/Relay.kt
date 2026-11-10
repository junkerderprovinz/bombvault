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
import kotlinx.coroutines.withTimeoutOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.net.URI
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import javax.crypto.AEADBadTagException
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/** The project relay every BombVault instance dials unless its owner picks another. */
const val PROJECT_RELAY = "wss://parleyport.halleluja.design/relay/connect"

/** The keys a group secret gives, derived the way the parleyport kit derives them. */
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

/**
 * The phone's member key in the group, the way the parleyport kit signs with
 * it: every call carries a signature, and the hello proves the id the key
 * makes.
 */
class MemberKey(private val seed: ByteArray) {
    val public: ByteArray = Proof.publicKey(seed)

    /** The id the phone goes by on the relay. */
    val id: String = "phone-" + hex(sha256(public))

    /** The id the phone goes by in BombVault when it has none from before. */
    val phoneId: String = hex(sha256(public).copyOf(16))

    fun sign(message: ByteArray): ByteArray = Proof.sign(seed, message)

    companion object {
        const val KIND = "mobile"
        private const val HELLO = "knightloader/relay/member-hello/v3"
        const val CALL = "bombvault/relay/member-call/v1"
        const val RESULT = "bombvault/relay/member-result/v1"

        /** The layout every member signature covers: the label, a zero byte, each field after its length, then [n]. */
        fun message(label: String, n: Long, vararg fields: ByteArray): ByteArray {
            val out = ByteArrayOutputStream()
            out.write(label.toByteArray())
            out.write(0)
            for (f in fields) {
                out.write(Proof.u64(f.size.toLong()))
                out.write(f)
            }
            out.write(Proof.u64(n))
            return out.toByteArray()
        }

        fun hello(relayKey: String, relayHost: String, id: String, challenge: ByteArray): ByteArray =
            message(HELLO, 0, relayKey.toByteArray(), relayHost.toByteArray(), id.toByteArray(), KIND.toByteArray(), challenge)

        /** The id an instance with [key] goes by on the relay. */
        fun instanceId(key: ByteArray): String = hex(sha256(key).copyOf(20))

        fun sha256(b: ByteArray): ByteArray = MessageDigest.getInstance("SHA-256").digest(b)

        fun hex(b: ByteArray) = b.joinToString("") { "%02x".format(it) }

        /** The host a hello names its relay by: lower case, no port, no brackets, no trailing dot. */
        fun relayHost(url: String): String {
            val host = runCatching { URI(url).host }.getOrNull() ?: return ""
            return host.trim('[', ']').trimEnd('.').lowercase()
        }
    }
}

/** Another member of the group, as its sealed announce introduced it. */
data class Sibling(val id: String, val name: String, val deployment: String, val client: Boolean)

/** What an instance answered: its status and body. [refused] is the code of a refusal, such as memberPending. */
data class Reply(val status: Int, val body: ByteArray) {
    val refused: String get() = if (status == 200) "" else runCatching { JSONObject(String(body)).optString("code") }.getOrDefault("")
}

/**
 * Relay keeps the phone's connection to the group's relay: it announces the
 * phone, tracks the instances and carries its calls. Every frame, seal and
 * signature matches the parleyport kit, which is the other end. The phone
 * calls and is never called.
 */
class Relay(
    url: String,
    private val keys: GroupKeys,
    private val member: MemberKey,
    private val name: String,
    private val onChange: () -> Unit,
) {
    private val url = connectUrl(url)
    private val host = MemberKey.relayHost(this.url)
    private val http = OkHttpClient.Builder().pingInterval(30, TimeUnit.SECONDS).readTimeout(0, TimeUnit.MILLISECONDS).build()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val pending = ConcurrentHashMap<String, CompletableDeferred<JSONObject>>()
    private val members = ConcurrentHashMap<String, Sibling>()

    @Volatile private var socket: WebSocket? = null
    @Volatile private var stopped = false
    private var backoff = MIN_BACKOFF_MS

    val connected: Boolean get() = socket != null

    /** The instances on the relay now. */
    val siblings: List<Sibling> get() = members.values.filter { !it.client }

    fun start() = dial()

    /**
     * Whether instance [target] is on the relay, waiting a moment for it:
     * right after the app starts, the instances are announced only once the
     * connection is up.
     */
    suspend fun reaches(target: String): Boolean =
        withTimeoutOrNull(ANNOUNCE_WAIT_MS) {
            while (members[target]?.client != false) delay(100)
            true
        } ?: false

    fun stop() {
        stopped = true
        socket?.close(1000, null)
        scope.cancel()
    }

    /** Asks instance [target] and returns its answer, once the key [target] is made from signed it. */
    suspend fun call(target: String, method: String, path: String, body: ByteArray? = null): Reply {
        val ws = socket ?: throw IOException("relay not connected")
        val rid = randomHex(16)
        val sealed = sealCall(rid, target, method, path, body ?: ByteArray(0), System.currentTimeMillis() / 1000)
        val answer = CompletableDeferred<JSONObject>()
        pending[rid] = answer
        try {
            ws.send(frame("proxy-request", JSONObject().put("requestId", rid).put("target", target).put("sealed", b64(sealed))))
            val resp = withTimeout(CALL_TIMEOUT_MS) { answer.await() }
            // The relay can write the error field itself, so it never counts as an answer.
            if (resp.has("error")) throw IOException("$target did not answer")
            val plain = open(keys.frameKey, "proxy-response\u0000$rid", unb64(resp.optString("sealed")))
                ?: throw IOException("$target answered unreadably")
            val r = JSONObject(String(plain))
            val reply = Reply(r.getInt("status"), unb64(r.optString("body")))
            if (!answeredBy(target, rid, sealed, reply, unb64(r.optString("key")), unb64(r.optString("sig")))) {
                throw IOException("$target answered without its key")
            }
            return reply
        } finally {
            pending.remove(rid)
        }
    }

    /** The call as the kit seals it, signed with the phone's key. */
    fun sealCall(rid: String, target: String, method: String, path: String, body: ByteArray, sent: Long): ByteArray {
        val sig = member.sign(
            MemberKey.message(
                MemberKey.CALL, sent, rid.toByteArray(), target.toByteArray(), method.toByteArray(), path.toByteArray(),
                body, member.id.toByteArray(), MemberKey.KIND.toByteArray(),
            ),
        )
        val call = JSONObject().put("method", method).put("path", path).put("id", rid).put("sent", sent)
            .put("from", member.id).put("kind", MemberKey.KIND).put("key", b64(member.public)).put("sig", b64(sig))
        if (body.isNotEmpty()) call.put("body", b64(body))
        return seal(keys.frameKey, "proxy-request\u0000$rid\u0000$target", call.toString().toByteArray())
    }

    private fun dial() {
        if (stopped) return
        http.newWebSocket(Request.Builder().url(url).build(), Listener())
    }

    private inner class Listener : WebSocketListener() {
        override fun onMessage(webSocket: WebSocket, text: String) {
            val env = runCatching { JSONObject(text) }.getOrNull() ?: return
            val data = env.optJSONObject("data") ?: return
            when (env.optString("type")) {
                "challenge" -> hello(webSocket, unb64(data.optString("nonce")))
                "announce" -> announced(data)
                "presence" -> if (!data.optBoolean("online")) {
                    members.remove(data.optString("instanceId"))
                    onChange()
                }
                "proxy-response" -> pending.remove(data.optString("requestId"))?.complete(data)
            }
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = lost(webSocket)

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) = lost(webSocket)
    }

    private fun hello(webSocket: WebSocket, challenge: ByteArray) {
        val identity = JSONObject().put("name", name).put("deployment", MemberKey.KIND).put("client", true).put("member", member.id)
        val sealed = seal(keys.frameKey, "announce\u0000${member.id}", identity.toString().toByteArray())
        val proof = JSONObject().put("kind", MemberKey.KIND).put("relay", host).put("key", b64(member.public))
            .put("sig", b64(member.sign(MemberKey.hello(keys.relayKey, host, member.id, challenge))))
        val announce = JSONObject().put("instanceId", member.id).put("sealed", b64(sealed))
        webSocket.send(frame("hello", JSONObject().put("key", keys.relayKey).put("announce", announce).put("proof", proof)))
        socket = webSocket
        backoff = MIN_BACKOFF_MS
        onChange()
    }

    private fun announced(data: JSONObject) {
        val sid = data.optString("instanceId")
        if (sid == member.id) return
        // An announce that does not open was not sealed by a group member.
        val plain = open(keys.frameKey, "announce\u0000$sid", unb64(data.optString("sealed"))) ?: return
        val who = runCatching { JSONObject(String(plain)) }.getOrNull() ?: return
        members[sid] = Sibling(sid, who.optString("name"), who.optString("deployment"), who.optBoolean("client"))
        onChange()
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
        private const val ANNOUNCE_WAIT_MS = 4_000L
        private const val MIN_BACKOFF_MS = 1_000L
        private const val MAX_BACKOFF_MS = 60_000L
        private val random = SecureRandom()

        /** Whether [reply] is [target]'s own answer to the call it got sealed as [sealedCall] under [rid]. */
        fun answeredBy(target: String, rid: String, sealedCall: ByteArray, reply: Reply, key: ByteArray, sig: ByteArray): Boolean {
            if (key.size != 32 || MemberKey.instanceId(key) != target) return false
            val signed = MemberKey.message(
                MemberKey.RESULT, reply.status.toLong(), rid.toByteArray(), target.toByteArray(),
                MemberKey.sha256(sealedCall), reply.body,
            )
            return Proof.verify(key, signed, sig)
        }

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
