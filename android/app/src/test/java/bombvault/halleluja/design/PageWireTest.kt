package bombvault.halleluja.design

import org.json.JSONObject
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayOutputStream
import java.util.Base64
import java.util.zip.GZIPOutputStream

// The field names are the ones internal/api/peer_page.go reads and writes.
class PageWireTest {
    @Test
    fun onlyARelayHostNamesAMember() {
        assertEquals("0123abcd", relayMember("0123abcd.relay.invalid"))
        assertEquals("0123abcd", relayMember("0123ABCD.Relay.Invalid"))
        assertNull(relayMember("relay.invalid"))
        assertNull(relayMember("a.b.relay.invalid"))
        assertNull(relayMember("abc.relay.invalid.example.com"))
        assertNull(relayMember("appassets.androidplatform.net"))
        assertNull(relayMember(null))
        assertEquals("https://0123abcd.relay.invalid", relayOrigin("0123abcd"))
    }

    @Test
    fun aRequestCarriesItsBodyAndOnlyTheHeadersTheServerTakes() {
        val ask = JSONObject(
            String(
                PageWire.ask(
                    "POST",
                    "/api/runs/ack?x=1",
                    mapOf("content-type" to "application/json", "Cookie" to "session=stolen", "X-BombVault-Body" to "t1"),
                    "{\"id\":1}".toByteArray(),
                ),
            ),
        )
        assertEquals("POST", ask.getString("method"))
        assertEquals("/api/runs/ack?x=1", ask.getString("path"))
        val header = ask.getJSONObject("header")
        assertEquals("application/json", header.getString("content-type"))
        assertEquals(1, header.length())
        assertEquals("{\"id\":1}", String(Base64.getDecoder().decode(ask.getString("body"))))
        assertFalse(JSONObject(String(PageWire.ask("GET", "/", emptyMap(), null))).has("body"))
    }

    @Test
    fun anAnswerInPartsComesBackWholeAndUnpacked() {
        val bundle = ByteArray(700) { (it * 7).toByte() }
        val packed = ByteArrayOutputStream().also { out -> GZIPOutputStream(out).use { it.write(bundle) } }.toByteArray()
        val step = 100
        val first = PageWire.piece(
            JSONObject()
                .put("ok", true)
                .put("status", 200)
                .put("header", JSONObject().put("Content-Type", "text/javascript").put("Content-Encoding", "gzip"))
                .put("body", Base64.getEncoder().encodeToString(packed.copyOf(step)))
                .put("size", packed.size)
                .put("part", "token")
                .toString().toByteArray(),
        )
        val offsets = PageWire.offsets(first)
        assertEquals((step until packed.size step step).toList(), offsets)
        val rest = offsets.map { packed.copyOfRange(it, minOf(it + step, packed.size)) }
        val whole = PageWire.whole(first, rest)
        assertEquals(200, whole.status)
        assertArrayEquals(bundle, whole.body)
        assertNull(whole.header["Content-Encoding"])
        assertEquals("text/javascript", whole.header["Content-Type"])

        val part = JSONObject(String(PageWire.part("token", 300)))
        assertEquals("token", part.getString("part"))
        assertEquals(300, part.getInt("offset"))
    }

    @Test
    fun anAnswerInOneCallHasNoMoreParts() {
        val one = PageWire.piece("""{"ok":true,"status":404,"body":"bm8=","size":2}""".toByteArray())
        assertTrue(PageWire.offsets(one).isEmpty())
        assertEquals("no", String(PageWire.whole(one, emptyList()).body))
    }

    @Test
    fun aRunThatLeftTheSnapshotGetsItsLastEvent() {
        val poll = ProgressPoll()
        val running = "data: {\"key\":\"container:nginx\",\"phase\":\"backup\",\"percent\":40,\"active\":true}\n\n"
        val first = poll.next(running)
        assertTrue(first.startsWith("retry: ${ProgressPoll.RETRY_MS}\n\n"))
        assertTrue(first.contains("\"container:nginx\""))

        val ended = poll.next("")
        val event = JSONObject(ended.lines().first { it.startsWith("data: ") }.removePrefix("data: "))
        assertEquals("container:nginx", event.getString("key"))
        assertFalse(event.getBoolean("active"))
        assertEquals(40, event.getInt("percent"))

        assertFalse(poll.next("").contains("data: "))
        poll.next(running)
        poll.reset()
        assertFalse(poll.next("").contains("data: "))
    }
}
