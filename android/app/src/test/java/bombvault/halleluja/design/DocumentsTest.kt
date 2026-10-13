package bombvault.halleluja.design

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class DocumentsTest {
    @get:Rule val tmp = TemporaryFolder()

    private val trusted = ByteArray(32) { 1 }
    private val other = ByteArray(32) { 2 }

    private val index = """
        <!DOCTYPE html><html><head>
        <link rel="icon" type="image/png" href="/logo.png" />
        <script type="module" crossorigin src="/assets/index-BUUuaLvF.js"></script>
        <link rel="stylesheet" crossorigin href="/assets/index-Dp42edRJ.css">
        </head><body><div id="root"></div></body></html>
    """.trimIndent()

    private fun document(text: String = index) = PageAnswer(
        200,
        mapOf(
            "Content-Type" to "text/html; charset=utf-8",
            "Cache-Control" to "no-store, must-revalidate",
            "Content-Security-Policy" to "default-src 'self'; script-src 'self'",
        ),
        text.toByteArray(),
    )

    @Test
    fun aKeptDocumentOpensWithTheHeadersItCameWith() {
        val docs = Documents(tmp.newFolder())
        docs.put("b1", trusted, "/", document())
        val kept = docs.get("b1", trusted, "/")!!
        assertEquals(200, kept.status)
        assertEquals("default-src 'self'; script-src 'self'", kept.header["Content-Security-Policy"])
        assertEquals("text/html; charset=utf-8", kept.header["Content-Type"])
        assertArrayEquals(index.toByteArray(), kept.body)
    }

    @Test
    fun aDocumentIsKeptOnlyUnderTheKeyAndPathItCameWith() {
        val docs = Documents(tmp.newFolder())
        docs.put("b1", trusted, "/", document())
        assertNull(docs.get("b1", other, "/"))
        assertNull(docs.get("b1", trusted, "/dashboard"))
        assertNull(docs.get("c2", trusted, "/"))
    }

    @Test
    fun aRefusedRefreshDropsTheKeptDocument() {
        val docs = Documents(tmp.newFolder())
        docs.put("b1", trusted, "/", document())
        docs.put("b1", trusted, "/dashboard", document())
        docs.drop("b1", trusted, "/")
        assertNull(docs.get("b1", trusted, "/"))
        assertNotNull(docs.get("b1", trusted, "/dashboard"))
    }

    @Test
    fun trustingAnotherKeyOrLeavingTheGroupDropsTheDocuments() {
        val docs = Documents(tmp.newFolder())
        docs.put("b1", trusted, "/", document())
        docs.put("c2", trusted, "/", document())
        docs.forget("b1")
        assertNull(docs.get("b1", trusted, "/"))
        assertNotNull(docs.get("c2", trusted, "/"))
        docs.forgetAll()
        assertNull(docs.get("c2", trusted, "/"))
    }

    @Test
    fun aDocumentNamesTheHashedBundlesItLoads() {
        assertEquals(listOf("/assets/index-BUUuaLvF.js", "/assets/index-Dp42edRJ.css"), bundlesOf(index))
    }

    @Test
    fun aDocumentIsReadyOnlyWhileEveryBundleItLoadsIsOnThePhone() {
        val bundles = Bundles(tmp.newFolder())
        val js = PageAnswer(200, mapOf("Content-Type" to "text/javascript"), "js".toByteArray())
        bundles.put("b1", trusted, "/assets/index-BUUuaLvF.js", js)
        assertFalse(bundlesOf(index).all { bundles.has("b1", trusted, it) })
        bundles.put("b1", trusted, "/assets/index-Dp42edRJ.css", js)
        assertTrue(bundlesOf(index).all { bundles.has("b1", trusted, it) })
        assertFalse(bundlesOf(index).all { bundles.has("b1", other, it) })
    }

    @Test
    fun onlyAWholeHtmlAnswerCountsAsADocument() {
        assertTrue(isDocument(document()))
        assertFalse(isDocument(PageAnswer(403, document().header, ByteArray(0))))
        assertFalse(isDocument(PageAnswer(200, mapOf("Content-Type" to "application/json"), "{}".toByteArray())))
    }
}
