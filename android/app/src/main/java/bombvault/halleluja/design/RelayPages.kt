package bombvault.halleluja.design

import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import kotlinx.coroutines.runBlocking
import java.io.ByteArrayInputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.File
import java.io.IOException
import java.security.MessageDigest
import java.util.Base64
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit

/**
 * Serves the interface of a group member through the relay, for a server the
 * phone cannot reach at its own address. Every request the WebView makes to
 * the member's relay origin becomes one sealed call, and the server answers
 * it as it would a paired phone at home.
 */
class RelayPages(
    private val pairing: Pairing,
    private val servers: Servers,
    cacheDir: File,
    private val onLost: (Server) -> Unit,
    private val onKeyChanged: (Server, ByteArray) -> Unit,
) {
    private val cache = Bundles(File(cacheDir, "relay-pages"))

    /** The member whose page the app opened through the relay, null while none is. */
    @Volatile var shown: String? = null

    private val bodies = ConcurrentHashMap<String, CompletableFuture<ByteArray>>()
    private val polls = ConcurrentHashMap<String, ProgressPoll>()

    /**
     * Takes the body of a request the page is about to send. A WebView hands
     * an intercepted request over without its body, so page.js passes it here
     * first under a ticket the request then names.
     */
    fun body(ticket: String, base64: String) {
        val data = runCatching { Base64.getDecoder().decode(base64) }.getOrNull() ?: return
        if (data.size > MAX_BODY) return
        // Bodies whose request never came are not kept for long.
        if (bodies.size > MAX_WAITING) bodies.clear()
        bodies.computeIfAbsent(ticket) { CompletableFuture() }.complete(data)
    }

    /** Answers one request the WebView makes to [member]'s relay origin. */
    fun serve(member: String, request: WebResourceRequest): WebResourceResponse {
        val headers = request.requestHeaders.mapKeys { it.key.lowercase() }
        if (!relayAllows(member, shown, headers["origin"], headers["referer"])) return plain(403)
        val server = servers.forMember(member) ?: return plain(404)
        val poll = polls.getOrPut(member) { ProgressPoll() }
        if (request.isForMainFrame) poll.reset()
        val method = request.method.uppercase()
        val url = request.url
        var path = (url.encodedPath ?: "/") + (url.encodedQuery?.let { "?$it" } ?: "")
        val header = request.requestHeaders.filterKeys { k -> PageWire.HEADERS.any { it.equals(k, ignoreCase = true) } }
        val body = request.requestHeaders.entries.firstOrNull { it.key.equals(BODY_HEADER, ignoreCase = true) }?.let { awaitBody(it.value) }
        val progress = method == "GET" && url.encodedPath == "/api/progress"

        val pinned = servers.memberKey(member)
        if (method == "GET" && pinned != null) cache.get(member, pinned, path)?.let { return respond(it) }
        var done: PageResult.Done? = null
        // A redirect from an intercepted request is not followed by the
        // WebView, so the app follows the few the interface makes.
        for (hop in 0..MAX_REDIRECTS) {
            val result = runBlocking { pairing.page(server, method.takeIf { hop == 0 } ?: "GET", path, header, body.takeIf { hop == 0 }) }
            if (result is PageResult.KeyChanged) {
                if (request.isForMainFrame) onKeyChanged(server, result.key)
                return plain(502)
            }
            done = result as? PageResult.Done
            val next = done?.answer?.header?.get("Location")
            if (done == null || done.answer.status !in 300..399 || next == null || !next.startsWith("/") || next.startsWith("//")) break
            path = next
        }
        if (progress) {
            val snapshot = done?.answer?.takeIf { it.status == 200 }
            val stream = if (snapshot != null) poll.next(String(snapshot.body)) else ProgressPoll.QUIET
            return WebResourceResponse("text/event-stream", "utf-8", 200, "OK", mapOf("Cache-Control" to "no-cache"), ByteArrayInputStream(stream.toByteArray()))
        }
        if (done == null) {
            if (request.isForMainFrame) onLost(server)
            return plain(502)
        }
        val answer = done.answer
        if (method == "GET" && answer.status == 200 && answer.header["Cache-Control"]?.contains("immutable") == true) cache.put(member, done.key, path, answer)
        return respond(answer)
    }

    /** Drops what [member]'s pages left on the phone, once another key is trusted for it. */
    fun forget(member: String) = cache.forget(member)

    /** Drops what every member's pages left on the phone, once the phone left the group. */
    fun forgetAll() = cache.forgetAll()

    private fun awaitBody(ticket: String): ByteArray? = try {
        bodies.computeIfAbsent(ticket) { CompletableFuture() }.get(BODY_WAIT_S, TimeUnit.SECONDS)
    } catch (_: Exception) {
        null
    } finally {
        bodies.remove(ticket)
    }

    private fun respond(a: PageAnswer): WebResourceResponse {
        val type = a.header["Content-Type"] ?: "application/octet-stream"
        val mime = type.substringBefore(';').trim()
        val charset = Regex("charset=\"?([^;\"]+)", RegexOption.IGNORE_CASE).find(type)?.groupValues?.get(1)?.trim()
        // A WebView takes no redirect from an intercepted request, and one it
        // could not follow is a failed request either way.
        val status = if (a.status in 100..299 || a.status in 400..599) a.status else 502
        val headers = a.header.filterKeys { it != "Location" && it != "Content-Type" }
        return WebResourceResponse(mime, charset, status, if (status < 400) "OK" else "Error", headers, ByteArrayInputStream(a.body))
    }

    private fun plain(status: Int) = WebResourceResponse("text/plain", "utf-8", status, "Error", emptyMap(), ByteArrayInputStream(ByteArray(0)))

    companion object {
        /** The header page.js names a request's body ticket in. */
        const val BODY_HEADER = "X-BombVault-Body"
        private const val BODY_WAIT_S = 5L
        private const val MAX_BODY = 1 shl 20
        private const val MAX_WAITING = 64
        private const val MAX_REDIRECTS = 3
    }
}

/**
 * Keeps the interface's bundles on the phone. Their names change with their
 * content, so one fetched once never has to come through the relay again.
 * Each is kept under the key its member signed it with, so a bundle from a
 * key the person trusted once does not outlast that trust.
 */
internal class Bundles(private val dir: File) {
    fun get(member: String, key: ByteArray, path: String): PageAnswer? {
        val f = file(member, key, path)
        if (!f.isFile) return null
        return try {
            DataInputStream(f.inputStream().buffered()).use {
                val type = it.readUTF()
                PageAnswer(200, mapOf("Content-Type" to type, "Cache-Control" to "public, max-age=31536000, immutable"), it.readBytes())
            }
        } catch (_: IOException) {
            null
        }
    }

    fun put(member: String, key: ByteArray, path: String, a: PageAnswer) {
        try {
            if (dir.walk().filter { it.isFile }.sumOf { it.length() } > MAX_CACHE) dir.deleteRecursively()
            val f = file(member, key, path)
            f.parentFile!!.mkdirs()
            val tmp = File.createTempFile("part", null, f.parentFile)
            DataOutputStream(tmp.outputStream().buffered()).use {
                it.writeUTF(a.header["Content-Type"] ?: "application/octet-stream")
                it.write(a.body)
            }
            tmp.renameTo(f)
        } catch (_: IOException) {
        }
    }

    fun forget(member: String) {
        memberDir(member).deleteRecursively()
    }

    fun forgetAll() {
        dir.deleteRecursively()
    }

    private fun memberDir(member: String) = File(dir, sha(member.toByteArray()))

    private fun file(member: String, key: ByteArray, path: String) = File(memberDir(member), sha(key + byteArrayOf(0) + path.toByteArray()))

    private fun sha(b: ByteArray) = MessageDigest.getInstance("SHA-256").digest(b).joinToString("") { "%02x".format(it) }

    companion object {
        private const val MAX_CACHE = 64L shl 20
    }
}
