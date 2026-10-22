package bombvault.halleluja.design

import org.junit.Assert.assertEquals
import org.junit.Test

// The body is what internal/api/group.go answers on GET /api/group/peer/members.
class RosterTest {
    private val body = """
        {"ok":true,"settling":false,
         "members":[
           {"id":"a1b2","name":"Attic","version":"v10.2.1","direct":false,"relay":true,"address":"https://192.168.1.21:3443"},
           {"id":"c3d4","name":"Pixel 8","version":"","direct":false,"relay":true,"address":"","kind":"android"}
         ],
         "away":[
           {"id":"e5f6","name":"Cellar","address":"https://192.168.1.22:3443/","lastSeen":1790000000},
           {"id":"a7b8","name":"","address":"","lastSeen":1790000500},
           {"id":"c9d0","name":"Old phone","kind":"android","address":"","lastSeen":1780000000}
         ]}
    """.trimIndent()

    @Test
    fun theInstancesInReachAndAwayAreListedWithoutPhones() {
        assertEquals(
            listOf(
                Roster.Entry("a1b2", "Attic", "https://192.168.1.21:3443/", 0),
                Roster.Entry("e5f6", "Cellar", "https://192.168.1.22:3443/", 1790000000),
                Roster.Entry("a7b8", "a7b8", "", 1790000500),
            ),
            Roster.parse(body),
        )
    }

    @Test
    fun anInstanceAwayKeepsWhenItWasLastSeenAndOneInReachHasNone() {
        val seen = Roster.parse(body).associate { it.id to it.seen }
        assertEquals(0L, seen["a1b2"])
        assertEquals(1790000000L, seen["e5f6"])
    }

    @Test
    fun aRefusalOrSomethingUnreadableListsNobody() {
        assertEquals(emptyList<Roster.Entry>(), Roster.parse("""{"ok":false,"error":"no"}"""))
        assertEquals(emptyList<Roster.Entry>(), Roster.parse("not json"))
        assertEquals(emptyList<Roster.Entry>(), Roster.parse("""{"ok":true}"""))
    }

    @Test
    fun anIdThatCannotNameARelayHostIsLeftOut() {
        val odd = """{"ok":true,"members":[{"id":"Not/A Host","name":"x","address":""}],"away":[]}"""
        assertEquals(emptyList<Roster.Entry>(), Roster.parse(odd))
    }
}
