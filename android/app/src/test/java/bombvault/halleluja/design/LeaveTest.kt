package bombvault.halleluja.design

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test

// The expected values come from internal/keydeposit, which checks the same
// claims on the server.
class LeaveTest {
    private val phoneSeed = ByteArray(32) { 1 }
    private val at = 1_790_000_000_000L

    @Test
    fun thePhonesLeaveIsSignedTheWayTheServerChecksIt() {
        val o = JSONObject(String(Leave.notice(phoneSeed, "app1", "tower", at)))
        assertEquals("iojj3XQJ8ZX9UtstPLpdcspnCb8dlBIb83SIAbQPb1w=", o.getString("key"))
        assertEquals(at, o.getLong("counter"))
        assertEquals("52zXvGlGnGZUhHLAPh02+sVdlRj5qsf6igsH8Dw87Kgax4OjEnTeuqS80T/km2NuBSt3vi0mJ/f7HudD6/CXDQ==", o.getString("proof"))
    }
}
