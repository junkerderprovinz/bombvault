package bombvault.halleluja.design

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RoutesTest {
    private fun answered(b: Boolean) = CompletableDeferred(b)

    private fun never() = CompletableDeferred<Boolean>()

    @Test
    fun whatTheLauncherLearnedHoldsForAWhile() {
        var now = 0L
        val routes = Routes { now }
        routes.heard("a", false)
        now += Routes.FRESH_MS - 1
        assertEquals(false, routes.direct("a"))
        now += 1
        assertNull(routes.direct("a"))
    }

    @Test
    fun anotherNetworkForgetsTheWays() {
        val routes = Routes()
        routes.heard("a", true)
        routes.forget()
        assertNull(routes.direct("a"))
    }

    @Test
    fun theOwnAddressWinsWhenItAnswersWithinItsHeadStart() = runBlocking {
        val direct = async {
            delay(50)
            true
        }
        assertEquals(Way.Direct, pickWay(direct, answered(true), headStart = 1000))
    }

    @Test
    fun aReadyRelayWinsOnceTheOwnAddressIsSilentPastItsHeadStart() = runBlocking {
        val way = withTimeout(1000) { pickWay(never(), answered(true), headStart = 100) }
        assertEquals(Way.Relay, way)
    }

    @Test
    fun anOwnAddressThatFailedHandsOverWithoutWaitingOutTheHeadStart() = runBlocking {
        val way = withTimeout(1000) { pickWay(answered(false), answered(true), headStart = 60_000) }
        assertEquals(Way.Relay, way)
    }

    @Test
    fun theOwnAddressStillWinsWhenTheRelayIsNotThere() = runBlocking {
        val direct = async {
            delay(150)
            true
        }
        assertEquals(Way.Direct, pickWay(direct, answered(false), headStart = 50))
    }

    @Test
    fun noWayWhenNeitherAnswers() = runBlocking {
        assertEquals(Way.None, pickWay(answered(false), answered(false), headStart = 50))
        assertEquals(Way.None, pickWay(null, answered(false), headStart = 50))
    }

    @Test
    fun theFirstFullAnswerDoesNotWaitForTheOtherAsk() = runBlocking {
        val hanging = CompletableDeferred<Answer?>()
        val a = withTimeout(1000) { firstAnswer(listOf(hanging, CompletableDeferred(Answer(200, "runs")))) }
        assertEquals("runs", a?.body)
    }

    @Test
    fun aFullAnswerBeatsAQuickerRefusal() = runBlocking {
        val slow = async<Answer?> {
            delay(100)
            Answer(200, "runs")
        }
        assertEquals(200, firstAnswer(listOf(slow, CompletableDeferred(Answer(401, ""))))?.status)
    }

    @Test
    fun withoutAFullAnswerTheGroupsAnswerComesFirst() = runBlocking {
        val group = CompletableDeferred<Answer?>(Answer(500, "group"))
        val own = CompletableDeferred<Answer?>(Answer(401, "own"))
        assertEquals("group", firstAnswer(listOf(group, own))?.body)
        val silent = CompletableDeferred<Answer?>().apply { complete(null) }
        assertEquals("own", firstAnswer(listOf(silent, own))?.body)
    }

    @Test
    fun anAddressOnTheLocalNetworkGetsLessTimeToConnect() {
        for (host in listOf("192.168.20.85", "10.0.0.2", "172.16.4.1", "169.254.1.1")) {
            assertTrue(host, connectTimeout(host) < connectTimeout("bombvault.example.org"))
        }
        for (host in listOf("172.32.0.1", "8.8.8.8", "192.168.1", "bv.lan", null)) {
            assertEquals(host, connectTimeout("bombvault.example.org"), connectTimeout(host))
        }
    }
}
