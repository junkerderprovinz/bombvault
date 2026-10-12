package bombvault.halleluja.design

import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
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

private val MEMBER = Regex("[0-9a-z][0-9a-z-]{0,62}")

/** One answer of a page over the relay, whole and unpacked. */
class PageAnswer(val status: Int, val header: Map<String, String>, val body: ByteArray)

/**
 * The wire form of a page over the relay, the way internal/api/peer_page.go
 * reads and writes it.
 */
object PageWire {
    const val PATH = "/api/group/peer/page"

    /** The request headers the server takes from a page. */
    val HEADERS = listOf("Content-Type", "Accept", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest")

    /** One part of an answer: the first carries the status and headers, and [part] names the rest. */
    class Piece(val status: Int, val header: Map<String, String>, val body: ByteArray, val size: Int, val part: String)

    fun ask(method: String, path: String, header: Map<String, String>, body: ByteArray?): ByteArray {
        val h = JSONObject()
        for ((k, v) in header) if (HEADERS.any { it.equals(k, ignoreCase = true) }) h.put(k, v)
        val o = JSONObject().put("method", method).put("path", path).put("header", h)
        if (body != null && body.isNotEmpty()) o.put("body", b64(body))
        return o.toString().toByteArray()
    }

    fun part(token: String, offset: Int): ByteArray = JSONObject().put("part", token).put("offset", offset).toString().toByteArray()

    fun piece(raw: ByteArray): Piece {
        val o = JSONObject(String(raw))
        val header = mutableMapOf<String, String>()
        o.optJSONObject("header")?.let { h -> for (k in h.keys()) header[k] = h.getString(k) }
        val body = o.optString("body").let { if (it.isEmpty()) ByteArray(0) else Base64.getDecoder().decode(it) }
        return Piece(o.optInt("status"), header, body, o.optInt("size", body.size), o.optString("part"))
    }

    /** Where the parts after [first] start; the server cuts every part but the last to one length. */
    fun offsets(first: Piece): List<Int> {
        val step = first.body.size
        if (step == 0 || first.part.isEmpty()) return emptyList()
        return (step until first.size step step).toList()
    }

    /** Puts [first] and the [rest] of its parts, in order, back into one answer. */
    fun whole(first: Piece, rest: List<ByteArray>): PageAnswer {
        val out = ByteArrayOutputStream(first.size)
        out.write(first.body)
        for (b in rest) out.write(b)
        var body = out.toByteArray()
        val header = first.header.toMutableMap()
        if (header.remove("Content-Encoding").equals("gzip", ignoreCase = true)) {
            body = GZIPInputStream(ByteArrayInputStream(body)).use { it.readBytes() }
        }
        return PageAnswer(first.status, header, body)
    }

    private fun b64(b: ByteArray) = Base64.getEncoder().encodeToString(b)
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
