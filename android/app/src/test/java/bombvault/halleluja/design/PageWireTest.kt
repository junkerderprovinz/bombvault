package bombvault.halleluja.design

import org.json.JSONObject
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayOutputStream
import java.io.IOException
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
        val wire = PageWire.assemble(first, rest)
        assertArrayEquals(packed, wire)
        val whole = PageWire.unpack(first.status, first.header, wire)
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
        assertEquals("no", String(PageWire.unpack(one.status, one.header, PageWire.assemble(one, emptyList())).body))
    }

    @Test
    fun anAnswerLargerThanItSaidOrThanTheCapIsRefused() {
        val small = PageWire.piece("""{"ok":true,"status":200,"body":"bm8=","size":2,"part":"t"}""".toByteArray())
        assertThrows(IOException::class.java) { PageWire.assemble(small, listOf(ByteArray(1))) }
        val huge = PageWire.piece("""{"ok":true,"status":200,"body":"bm8=","size":${PageWire.MAX_ANSWER + 1},"part":"t"}""".toByteArray())
        assertThrows(IOException::class.java) { PageWire.assemble(huge, emptyList()) }

        val bomb = ByteArrayOutputStream().also { out ->
            GZIPOutputStream(out).use { z -> repeat(PageWire.MAX_UNPACKED / (1 shl 20) + 1) { z.write(ByteArray(1 shl 20)) } }
        }.toByteArray()
        assertThrows(IOException::class.java) { PageWire.unpack(200, mapOf("Content-Encoding" to "gzip"), bomb) }
    }

    @Test
    fun aPhoneSignsAsTheServerChecks() {
        // The same seed and claim in internal/api's TestAPhoneSignatureMatchesTheApp.
        val seed = ByteArray(32) { 7 }
        val message = Proof.message("page", "phone1", "attic1", 1790000000000, """{"method":"GET","path":"/"}""".toByteArray())
        assertEquals("ea4a6c63e29c520abef5507b132ec5f9954776aebebe7b92421eea691446d22c", hex(Proof.publicKey(seed)))
        val proof = Proof.sign(seed, message)
        assertEquals(
            "f5eae457fb1c03dda4c99e15e842e34596c13e560a61420645fb7127698661e056d6dca0d11f1b8a1c0e383c2110d4a811ad69d02603ac0c1a56c7bc1fcf1b05",
            hex(proof),
        )
        assertTrue(Proof.verify(Proof.publicKey(seed), message, proof))
        assertFalse(Proof.verify(Proof.publicKey(ByteArray(32) { 8 }), message, proof))
        assertFalse(Proof.verify(Proof.publicKey(seed), Proof.message("page", "phone1", "attic1", 1790000000001), proof))
    }

    @Test
    fun theAnswerDigestIsTheServers() {
        // The same answer in internal/api's TestTheAnswerDigestMatchesTheApp.
        val digest = PageWire.digest(
            """{"method":"GET","path":"/"}""".toByteArray(),
            200,
            mapOf("Content-Type" to "text/javascript", "Cache-Control" to "immutable"),
            "bundle".toByteArray(),
        )
        assertEquals("34199ce6a09cbfe79396ca3e5bb294b36084ad7ba5fc127e94509ecb63ec1733", hex(digest))
    }

    @Test
    fun onlyThePageOnScreenReachesItsMember() {
        val own = relayOrigin("b1")
        assertTrue(relayAllows("b1", "b1", null, null))
        assertTrue(relayAllows("b1", "b1", own, "$own/instances"))
        assertFalse(relayAllows("b1", null, null, null))
        assertFalse(relayAllows("b1", "c2", null, null))
        assertFalse(relayAllows("b1", "b1", relayOrigin("c2"), null))
        assertFalse(relayAllows("b1", "b1", "https://appassets.androidplatform.net", null))
        assertFalse(relayAllows("b1", "b1", null, "https://evil.example/"))
        assertFalse(relayAllows("b1", "b1", null, "$own.evil.example/"))
    }

    private fun hex(b: ByteArray) = b.joinToString("") { "%02x".format(it) }

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
