package bombvault.halleluja.design

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.util.Base64

// The answer an instance gives once its group moved into a fortress: the keys
// of the pairing there, which the phone keeps in place of its words.
class MovedTest {
    private val fortress = ByteArray(16) { 0x2a }
    private val keys = GroupKeys(fortress)
    private val b64 = Base64.getEncoder()

    private fun answer(group: String = keys.relayKey, frame: ByteArray = keys.frameKey) =
        """{"group":"$group","frame":"${b64.encodeToString(frame)}","peerAuth":"${b64.encodeToString(ByteArray(32) { 1 })}"}"""

    @Test
    fun theKeysOfThePairingAreTakenAsTheyCome() {
        val moved = Moved.parse(answer())!!
        assertEquals(keys.relayKey, moved.relayKey)
        assertArrayEquals(keys.frameKey, moved.frameKey)
    }

    @Test
    fun thePhoneAsksWhereTheKitHandsTheKeysOut() {
        assertEquals("/kit/fortress/v1/moved", Moved.PATH)
    }

    @Test
    fun keysKeptAsTextComeBackTheSame() {
        val kept = Moved.parse(answer())!!.let { Moved.store(it) }
        val back = Moved.load(kept)!!
        assertEquals(keys.relayKey, back.relayKey)
        assertArrayEquals(keys.frameKey, back.frameKey)
    }

    @Test
    fun anAnswerThatIsNoPairOfKeysIsLeftAlone() {
        assertNull(Moved.parse(answer(group = "abc")))
        assertNull(Moved.parse(answer(group = keys.relayKey.uppercase().replace('A', 'G'))))
        assertNull(Moved.parse(answer(frame = ByteArray(8))))
        assertNull(Moved.parse("""{"error":"not_moved"}"""))
        assertNull(Moved.parse("not json"))
        assertNull(Moved.load("not kept"))
    }
}
