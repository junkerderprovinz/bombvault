package bombvault.halleluja.design

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class BundlesTest {
    @get:Rule val tmp = TemporaryFolder()

    private val trusted = ByteArray(32) { 1 }
    private val other = ByteArray(32) { 2 }
    private val path = "/assets/index-3f2a.js"

    private fun bundle(text: String) =
        PageAnswer(200, mapOf("Content-Type" to "text/javascript", "Cache-Control" to "public, immutable"), text.toByteArray())

    @Test
    fun aBundleIsServedOnlyUnderTheKeyItWasSignedWith() {
        val cache = Bundles(tmp.newFolder())
        cache.put("b1", other, path, bundle("someone else's"))
        assertNull(cache.get("b1", trusted, path))

        cache.put("b1", trusted, path, bundle("the member's own"))
        val kept = cache.get("b1", trusted, path)!!
        assertArrayEquals("the member's own".toByteArray(), kept.body)
        assertEquals("text/javascript", kept.header["Content-Type"])
        assertArrayEquals("someone else's".toByteArray(), cache.get("b1", other, path)!!.body)
    }

    @Test
    fun trustingAnotherKeyDropsOnlyThatMembersBundles() {
        val cache = Bundles(tmp.newFolder())
        cache.put("b1", other, path, bundle("b1"))
        cache.put("c2", trusted, path, bundle("c2"))
        cache.forget("b1")
        assertNull(cache.get("b1", other, path))
        assertNotNull(cache.get("c2", trusted, path))
    }

    @Test
    fun leavingTheGroupDropsEveryBundle() {
        val cache = Bundles(tmp.newFolder())
        cache.put("b1", trusted, path, bundle("b1"))
        cache.put("c2", trusted, path, bundle("c2"))
        cache.forgetAll()
        assertNull(cache.get("b1", trusted, path))
        assertNull(cache.get("c2", trusted, path))

        cache.put("b1", trusted, path, bundle("again"))
        assertArrayEquals("again".toByteArray(), cache.get("b1", trusted, path)!!.body)
    }
}
