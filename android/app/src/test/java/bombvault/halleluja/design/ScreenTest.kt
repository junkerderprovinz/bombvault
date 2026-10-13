package bombvault.halleluja.design

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ScreenTest {
    private val here = Server("a", "BV-RC", "", null, "m1")
    private val home = Server("b", "BV-RC-B", "https://bv-rc-b.lan", null, "m2")
    private val away = Server("c", "BV-RC-C", "", null, "m3")

    private fun onRelayPage(): Screen = Screen().apply {
        open(here)
        shows(here)
    }

    @Test
    fun theShownPageSteppingBackInItsHistoryLeavesTheSwitchOn() {
        val screen = onRelayPage()
        screen.open(away)

        assertFalse(screen.shows(here))

        assertTrue(screen.opens(away))
        assertEquals(away, screen.current)
    }

    @Test
    fun theOpenedServersPageEndsTheWay() {
        val screen = onRelayPage()
        screen.open(home)

        assertTrue(screen.shows(home))
        assertFalse(screen.opens(home))
        assertEquals(home, screen.current)
        assertTrue(screen.shows(home))
    }

    @Test
    fun aNewerOpenTakesOverTheWay() {
        val screen = onRelayPage()
        screen.open(home)
        screen.open(away)

        assertFalse(screen.opens(home))
        assertTrue(screen.opens(away))
    }

    @Test
    fun backWhileOnTheWayStaysWithThePageOnScreen() {
        val screen = onRelayPage()
        screen.open(home)

        assertTrue(screen.cancel())

        assertFalse(screen.opens(home))
        assertEquals(here, screen.current)
        assertFalse(screen.cancel())
    }

    @Test
    fun goingHomeFromAServersPageLeavesNoServerOnScreen() {
        val screen = onRelayPage()

        screen.launcher()

        assertNull(screen.current)
        assertTrue(screen.shows(null))
        assertNull(screen.current)
    }

    @Test
    fun theLauncherCallsTheWayOff() {
        val screen = onRelayPage()
        screen.open(home)

        screen.launcher()

        assertFalse(screen.opens(home))
        assertNull(screen.current)
    }
}
