package bombvault.halleluja.design

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Base64

// The expected values come from internal/keydeposit, which signs and checks
// the same claims on the server.
class LeaveTest {
    private val phoneSeed = ByteArray(32) { 1 }
    private val serverKey = Base64.getDecoder().decode("gTl3Dqh9F19Wo1Rmw0x+zMuNipG07jeiXfYPW4/Js5Q=")
    private val at = 1_790_000_000_000L

    private fun serverWord(counter: Long = at) = JSONObject()
        .put("key", PageWire.b64(serverKey))
        .put("counter", counter)
        .put("proof", "kNdra+ioImPpWdYk/yAeG2DbGQ4qSdKDx85REkH2kSyQwzqEAZDyOvlUfUVrItVeqjIUZxzM6dJqUPLESys7Cg==")
        .toString()
        .toByteArray()

    @Test
    fun thePhonesLeaveIsSignedTheWayTheServerChecksIt() {
        val o = JSONObject(String(Leave.notice(phoneSeed, "app1", "tower", at)))
        assertEquals("iojj3XQJ8ZX9UtstPLpdcspnCb8dlBIb83SIAbQPb1w=", o.getString("key"))
        assertEquals(at, o.getLong("counter"))
        assertEquals("52zXvGlGnGZUhHLAPh02+sVdlRj5qsf6igsH8Dw87Kgax4OjEnTeuqS80T/km2NuBSt3vi0mJ/f7HudD6/CXDQ==", o.getString("proof"))
    }

    @Test
    fun aServersWordThatItLeftCountsUnderTheKeyPinnedForIt() {
        assertTrue(Leave.heard(serverKey, "tower", "app1", serverWord(), at + 1_000))
    }

    @Test
    fun aWordThatAServerLeftCountsOnlyFromThatServerToThisPhone() {
        val other = Proof.publicKey(ByteArray(32) { 3 })
        assertFalse(Leave.heard(null, "tower", "app1", serverWord(), at))
        assertFalse(Leave.heard(other, "tower", "app1", serverWord(), at))
        assertFalse(Leave.heard(serverKey, "garage", "app1", serverWord(), at))
        assertFalse(Leave.heard(serverKey, "tower", "app2", serverWord(), at))
        assertFalse(Leave.heard(serverKey, "tower", "app1", serverWord(at + 1), at))
        assertFalse(Leave.heard(serverKey, "tower", "app1", "not json".toByteArray(), at))
    }

    @Test
    fun anOldWordThatAServerLeftDoesNotCount() {
        assertFalse(Leave.heard(serverKey, "tower", "app1", serverWord(), at + 10 * 60_000))
    }
}
