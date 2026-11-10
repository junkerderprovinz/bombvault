package bombvault.halleluja.design

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.util.Base64

// The expected values come from the parleyport kit, so a change on either
// side shows up here as a phone that cannot join its group.
class GroupWireTest {
    private val phrase = Phrase(File("src/main/assets/bip39-english.txt").readLines().filter { it.isNotBlank() })
    private val words = "legal winner thank year wave sausage worth useful legal winner thank yellow"
    private val secret = ByteArray(16) { 0x7f }

    @Test
    fun theWordsDecodeToTheServersSecret() {
        assertArrayEquals(secret, phrase.decode(words).getOrThrow())
    }

    @Test
    fun aMistypedWordIsCaughtByTheChecksum() {
        val refused = phrase.decode(words.replace("yellow", "year")).exceptionOrNull() as Phrase.Refused
        assertEquals(PhraseError.Checksum, refused.reason)
    }

    @Test
    fun theKeysMatchTheServers() {
        val keys = GroupKeys(secret)
        assertEquals("a76ebfdb7fc9f2b8e17f55c13db67e6a280c04a504b9a4361f3d41d5dbd72022", keys.relayKey)
        assertEquals("9ccf7381ec22225691a9c85c06293cd7c651c415da4abaedf25481d23b248d71", keys.frameKey.joinToString("") { "%02x".format(it) })
    }

    @Test
    fun anAnnounceTheServerSealedOpens() {
        val sealed = Base64.getDecoder().decode("6d7RsOcAR4S0a0VLtUJHylXO+yGmnaQEwgjB341s9GCnjWwzmWexlfp54c2bk+EM+m30JirKL/gVNq/d+1km")
        val plain = Relay.open(GroupKeys(secret).frameKey, "announce\u0000abc123", sealed)
        assertEquals("""{"name":"Tower","version":"v9.8.0"}""", String(plain!!))
    }

    @Test
    fun aCallOpensOnlyForTheMemberItWasSealedTo() {
        val sealed = Base64.getDecoder().decode(
            "MONLcH4C9R+TI/yhh/DnlDwYQkfRWjjeFX1JwuPJVZMWVKzUz+RriC6TKwXW/DcsIDC1l+l+Kzp0c2qsRqMfrzpO3h7hhtHrHKHrO7YDs+r/f4y8dPcLTc1M+HwOZDmffn1BH++w7IhQ",
        )
        val key = GroupKeys(secret).frameKey
        val plain = Relay.open(key, "proxy-request\u0000rid1\u0000app1", sealed)
        assertEquals("""{"method":"GET","path":"/api/group/peer/hello","id":"rid1","sent":1790000000}""", String(plain!!))
        assertNull(Relay.open(key, "proxy-request\u0000rid1\u0000someone-else", sealed))
    }

    @Test
    fun whatTheAppSealsTheServerCanOpen() {
        val key = GroupKeys(secret).frameKey
        val sealed = Relay.seal(key, "proxy-response\u0000rid1", "hi".toByteArray())
        assertEquals("hi", String(Relay.open(key, "proxy-response\u0000rid1", sealed)!!))
    }

    private val phone = MemberKey(ByteArray(32) { 5 })
    private val instance = "dbc298251c51321b7266e78d1c151c2b62aff8cb"
    private val instanceKey = Base64.getDecoder().decode("/RckOFqgx1tk+3jNYC+h2ZH96/drE8WO1wLqyDXp9hg=")

    @Test
    fun thePhoneGoesByTheIdsItsKeyMakes() {
        assertEquals("phone-7599776c3085e3f9da0d13071eb0b4ab50fd2bf64c06dd92c2365af3a328eca3", phone.id)
        assertEquals("7599776c3085e3f9da0d13071eb0b4ab", phone.phoneId)
        assertEquals(instance, MemberKey.instanceId(instanceKey))
    }

    @Test
    fun theHelloProvesThePhonesIdTheWayTheRelayChecksIt() {
        val signed = MemberKey.hello(GroupKeys(secret).relayKey, "relay.example.org", phone.id, "challenge-bytes-0123456789abcdef".toByteArray())
        assertEquals(
            "aHccJdpm/rte3OHkWTeSh+k30U612KS1Gf8hpU87/df2lgJ3CdPFktH/khOoDqtyQw7gjuGlHhq4UYKL8M+mAg==",
            Base64.getEncoder().encodeToString(phone.sign(signed)),
        )
        assertEquals("relay.example.org", MemberKey.relayHost("wss://Relay.Example.org.:443/relay/connect"))
    }

    @Test
    fun aCallIsSignedTheWayAnInstanceChecksIt() {
        val signed = MemberKey.message(
            MemberKey.CALL, 1_790_000_000, "rid1".toByteArray(), instance.toByteArray(), "GET".toByteArray(),
            "/api/group/peer/hello".toByteArray(), ByteArray(0), phone.id.toByteArray(), "mobile".toByteArray(),
        )
        assertEquals(
            "8VWKGSlDB7gUnxaeiBnlAmWMtXQlzD+b3BiSmpukre3FYrN7fWgXqKmoMqHyHYzLbSRAya2W18aGmN0d/LJ0Dw==",
            Base64.getEncoder().encodeToString(phone.sign(signed)),
        )
    }

    @Test
    fun anAnswerCountsOnlyAsTheInstancesWhoseKeySignedIt() {
        val sealed = "the sealed call as it went out".toByteArray()
        val sig = Base64.getDecoder().decode("Aj9BVhX0bCM1zJSBHTeYendCXaNLSIz1bWOxDG8CyLAV35BgKaZuzjv0cZBNygStwZiZOmm2oLiq+y9L3ODoAQ==")
        val reply = Reply(200, """{"ok":true}""".toByteArray())
        assertTrue(Relay.answeredBy(instance, "rid1", sealed, reply, instanceKey, sig))
        assertFalse(Relay.answeredBy("0".repeat(40), "rid1", sealed, reply, instanceKey, sig))
        assertFalse(Relay.answeredBy(instance, "rid2", sealed, reply, instanceKey, sig))
        assertFalse(Relay.answeredBy(instance, "rid1", "another call".toByteArray(), reply, instanceKey, sig))
        assertFalse(Relay.answeredBy(instance, "rid1", sealed, Reply(200, """{"ok":false}""".toByteArray()), instanceKey, sig))
    }

    @Test
    fun aRefusalNamesItsCode() {
        assertEquals("memberPending", Reply(403, """{"error":"waits","code":"memberPending"}""".toByteArray()).refused)
        assertEquals("", Reply(200, "{}".toByteArray()).refused)
    }
}
