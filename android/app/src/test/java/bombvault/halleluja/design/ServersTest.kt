package bombvault.halleluja.design

import org.junit.Assert.assertEquals
import org.junit.Test

class ServersTest {
    @Test
    fun theServersSavedByAddressAloneGoAndAreNamed() {
        val member = Server("a", "Parents", "", null, "m1")
        val memberAtHome = Server("b", "Tower", "https://tower.lan:3443/", null, "m2")
        val office = Server("c", "Office", "https://192.168.1.10:3443/", null)
        val nas = Server("d", "NAS", "http://10.8.0.2:3080/", "AB:CD")

        val (kept, gone) = withoutAddressOnly(listOf(member, office, memberAtHome, nas))

        assertEquals(listOf(member, memberAtHome), kept)
        assertEquals(listOf("Office", "NAS"), gone)
    }

    @Test
    fun aGroupOnlyListStaysAsItIs() {
        val list = listOf(Server("a", "Parents", "", null, "m1"))

        assertEquals(list to emptyList<String>(), withoutAddressOnly(list))
    }
}
