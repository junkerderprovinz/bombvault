package bombvault.halleluja.design

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ServersTest {
    private val here = Server("a", "BV-RC", "https://bv-rc.lan", null, "m1")
    private val home = Server("b", "BV-RC-B", "https://bv-rc-b.lan", null, "m2")
    private val away = Server("c", "BV-RC-C", "", null, "m3")
    private val nowhere = Server("d", "Old", "", null)
    private val list = listOf(here, home, away, nowhere)

    @Test
    fun aPageHearsOfTheOtherServersButNotOfItself() {
        assertEquals(listOf(home, away), othersThan(list, here))
    }

    @Test
    fun aPageOpensAnotherServerOfTheListByItsId() {
        assertEquals(home, openable(list, here, "b"))
        assertEquals(away, openable(list, here, "c"))
    }

    @Test
    fun aPageCannotOpenAnAddressOrAServerTheAppDoesNotKnow() {
        assertNull(openable(list, here, "https://evil.example"))
        assertNull(openable(list, here, "https://bv-rc-b.lan"))
        assertNull(openable(list, here, "m2"))
        assertNull(openable(list, here, "z"))
        assertNull(openable(list, here, ""))
    }

    @Test
    fun aPageCannotOpenItselfOrAServerWithNoWayIn() {
        assertNull(openable(list, here, "a"))
        assertNull(openable(list, here, "d"))
    }
}
