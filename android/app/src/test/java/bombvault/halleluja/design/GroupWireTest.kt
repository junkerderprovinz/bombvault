package bombvault.halleluja.design

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.io.File
import java.util.Base64

// The expected values come from internal/seedphrase and internal/relay, so a
// change on either side shows up here as a phone that cannot join its group.
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
}
