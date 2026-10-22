package bombvault.halleluja.design

import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters
import org.bouncycastle.crypto.signers.Ed25519Signer
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.nio.ByteBuffer
import java.security.MessageDigest
import java.util.Base64
import java.util.zip.GZIPInputStream

/**
 * The host a server's interface opens under when the app reaches it through
 * the group. The name is reserved never to resolve, so a request the app does
 * not catch goes nowhere.
 */
const val RELAY_HOST = "relay.invalid"

/** The origin the interface of group member [member] opens at over the relay. */
fun relayOrigin(member: String) = "https://$member.$RELAY_HOST"

/** The member a relay origin's host names, or null for any other host. */
fun relayMember(host: String?): String? {
    val h = host?.lowercase() ?: return null
    if (!h.endsWith(".$RELAY_HOST")) return null
    return h.removeSuffix(".$RELAY_HOST").takeIf { MEMBER.matches(it) }
}

/** What a member id looks like: one label of a host name. */
internal val MEMBER = Regex("[0-9a-z][0-9a-z-]{0,62}")

/**
 * Whether a request the WebView makes to [member]'s relay origin may go
 * through: only while that member's page is the one on screen, and never
 * from another page, which could otherwise start a backup on it.
 */
fun relayAllows(member: String, shown: String?, origin: String?, referer: String?): Boolean {
    if (member != shown) return false
    val own = relayOrigin(member)
    if (origin != null && origin != own) return false
    return referer == null || referer == own || referer.startsWith("$own/")
}

/** One answer of a page over the relay, whole and unpacked. */
class PageAnswer(val status: Int, val header: Map<String, String>, val body: ByteArray)

/**
 * The Ed25519 signatures members of the group sign what they say with, spelled
 * the way internal/keydeposit spells a claim.
 */
object Proof {
    fun publicKey(seed: ByteArray): ByteArray = Ed25519PrivateKeyParameters(seed, 0).generatePublicKey().encoded

    fun message(action: String, owner: String, keeper: String, counter: Long, deposit: ByteArray = ByteArray(0)): ByteArray {
        val out = ByteArrayOutputStream()
        out.write("bombvault key deposit proof 1\u0000".toByteArray())
        for (field in listOf(action.toByteArray(), owner.toByteArray(), keeper.toByteArray(), deposit, ByteArray(0))) {
            out.write(u64(field.size.toLong()))
            out.write(field)
        }
        out.write(u64(counter))
        return out.toByteArray()
    }

    fun sign(seed: ByteArray, message: ByteArray): ByteArray = Ed25519Signer().run {
        init(true, Ed25519PrivateKeyParameters(seed, 0))
        update(message, 0, message.size)
        generateSignature()
    }

    fun verify(key: ByteArray, message: ByteArray, signature: ByteArray): Boolean {
        if (key.size != 32 || signature.size != 64) return false
        return Ed25519Signer().run {
            init(false, Ed25519PublicKeyParameters(key, 0))
            update(message, 0, message.size)
            verifySignature(signature)
        }
    }

    /** The colon-separated SHA-256 of [key], the way the app shows a certificate's. */
    fun fingerprint(key: ByteArray): String = MessageDigest.getInstance("SHA-256").digest(key).joinToString(":") { "%02X".format(it) }

    fun u64(n: Long): ByteArray = ByteBuffer.allocate(8).putLong(n).array()
}

/**
 * The wire form of a page over the relay, the way internal/api/peer_page.go
 * reads and writes it.
 */
object PageWire {
    const val PATH = "/api/group/peer/page"

    /** The most of one answer the app takes, as the server caps it too. */
    const val MAX_ANSWER = 16 shl 20

    /** The most an answer may unpack to. */
    const val MAX_UNPACKED = 48 shl 20

    /** The length the server cuts every part of an answer but the last to, so an answer has 64 parts at most. */
    const val PART = 256 shl 10

    /** The request headers the server takes from a page. */
    val HEADERS = listOf("Content-Type", "Accept", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest")

    /** One part of an answer: the first carries the status, headers and signature, and [part] names the rest. */
    class Piece(val status: Int, val header: Map<String, String>, val body: ByteArray, val size: Int, val part: String, val proof: ByteArray)

    fun ask(method: String, path: String, header: Map<String, String>, body: ByteArray?): ByteArray {
        val h = JSONObject()
        for ((k, v) in header) if (HEADERS.any { it.equals(k, ignoreCase = true) }) h.put(k, v)
        val o = JSONObject().put("method", method).put("path", path).put("header", h)
        if (body != null && body.isNotEmpty()) o.put("body", b64(body))
        return o.toString().toByteArray()
    }

    fun part(token: String, offset: Int): ByteArray = JSONObject().put("part", token).put("offset", offset).toString().toByteArray()

    /** The call that carries [ask], signed by the phone with [seed] for [keeper]. */
    fun call(ask: ByteArray, session: String?, seed: ByteArray, phone: String, keeper: String, at: Long): ByteArray {
        val proof = Proof.sign(seed, Proof.message("page", phone, keeper, at, ask))
        val o = JSONObject().put("ask", b64(ask)).put("key", b64(Proof.publicKey(seed))).put("counter", at).put("proof", b64(proof))
        if (!session.isNullOrEmpty()) o.put("session", session)
        return o.toString().toByteArray()
    }

    fun piece(raw: ByteArray): Piece {
        val o = JSONObject(String(raw))
        val header = mutableMapOf<String, String>()
        o.optJSONObject("header")?.let { h -> for (k in h.keys()) header[k] = h.getString(k) }
        val body = unb64(o.optString("body"))
        return Piece(o.optInt("status"), header, body, o.optInt("size", body.size), o.optString("part"), unb64(o.optString("proof")))
    }

    /**
     * Where the parts after [first] start. The first part is not signed yet,
     * so its size and length are checked before a single part is asked for.
     */
    fun offsets(first: Piece): List<Int> {
        if (first.size !in 0..MAX_ANSWER) throw IOException("the answer is too large")
        if (first.part.isEmpty()) return emptyList()
        if (first.body.size != PART || first.size <= PART) throw IOException("the answer is cut where the server never cuts it")
        return (PART until first.size step PART).toList()
    }

    /** Puts [first] and the [rest] of its parts, in order, back into the body as it was sent. */
    fun assemble(first: Piece, rest: List<ByteArray>): ByteArray {
        if (first.size !in 0..MAX_ANSWER) throw IOException("the answer is too large")
        val out = ByteArrayOutputStream(first.size)
        out.write(first.body)
        for (b in rest) {
            if (out.size() + b.size > first.size) throw IOException("the answer is longer than it said")
            out.write(b)
        }
        if (out.size() != first.size) throw IOException("the answer is shorter than it said")
        return out.toByteArray()
    }

    /** What the server signed for an answer to [ask]; internal/api spells it the same way. */
    fun digest(ask: ByteArray, status: Int, header: Map<String, String>, body: ByteArray): ByteArray {
        val sha = MessageDigest.getInstance("SHA-256")
        val h = MessageDigest.getInstance("SHA-256")
        h.update("bombvault page answer 1\u0000".toByteArray())
        fun field(b: ByteArray) {
            h.update(Proof.u64(b.size.toLong()))
            h.update(b)
        }
        field(sha.digest(ask))
        field(status.toString().toByteArray())
        for (k in header.keys.sorted()) {
            field(k.toByteArray())
            field(header.getValue(k).toByteArray())
        }
        field(sha.digest(body))
        return h.digest()
    }

    /** The answer the page gets, unpacked when it came packed. */
    fun unpack(status: Int, header: Map<String, String>, body: ByteArray): PageAnswer {
        val h = header.toMutableMap()
        if (!h.remove("Content-Encoding").equals("gzip", ignoreCase = true)) return PageAnswer(status, h, body)
        val out = ByteArrayOutputStream()
        GZIPInputStream(ByteArrayInputStream(body)).use { zin ->
            val buf = ByteArray(64 shl 10)
            while (true) {
                val n = zin.read(buf)
                if (n < 0) break
                if (out.size() + n > MAX_UNPACKED) throw IOException("the answer unpacks too large")
                out.write(buf, 0, n)
            }
        }
        return PageAnswer(status, h, out.toByteArray())
    }

    /** The JSON array internal/api's callPayload makes of a few ids, which need no escaping. */
    fun payload(vararg parts: String): ByteArray = JSONArray(parts.toList()).toString().toByteArray()

    fun b64(b: ByteArray): String = Base64.getEncoder().encodeToString(b)

    fun unb64(s: String): ByteArray = if (s.isEmpty()) ByteArray(0) else Base64.getDecoder().decode(s)
}

/**
 * Turns the snapshots of /api/progress, which a page over the relay polls,
 * back into the stream the interface expects: a run that left the snapshot
 * since the last poll gets its final event, so its bar ends instead of
 * standing still.
 */
class ProgressPoll {
    private var last: Map<String, String> = emptyMap()

    /** Forgets the runs seen, for a page that starts again. */
    @Synchronized
    fun reset() {
        last = emptyMap()
    }

    /** The stream to hand the page for one [snapshot]. */
    @Synchronized
    fun next(snapshot: String): String {
        val now = LinkedHashMap<String, String>()
        for (line in snapshot.lines()) {
            if (!line.startsWith("data: ")) continue
            val event = runCatching { JSONObject(line.removePrefix("data: ")) }.getOrNull() ?: continue
            val key = event.optString("key")
            if (key.isNotEmpty()) now[key] = event.toString()
        }
        val out = StringBuilder("retry: $RETRY_MS\n\n")
        for (event in now.values) out.append("data: ").append(event).append("\n\n")
        for ((key, event) in last) {
            if (key !in now) out.append("data: ").append(JSONObject(event).put("active", false)).append("\n\n")
        }
        last = now
        return out.toString()
    }

    companion object {
        /** How long the page waits before it polls again. */
        const val RETRY_MS = 2000

        /** What the page gets while the relay does not answer: a later try. */
        const val QUIET = "retry: 5000\n\n"
    }
}
