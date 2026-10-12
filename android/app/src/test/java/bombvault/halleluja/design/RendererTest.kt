package bombvault.halleluja.design

import org.junit.Assert.assertEquals
import org.junit.Test

class RendererTest {
    @Test
    fun aCrashedPageGoesBackToTheListWithANotice() {
        assertEquals(AfterRenderer.Notice, afterRendererGone(crashed = true, onServer = true))
        assertEquals(AfterRenderer.Notice, afterRendererGone(crashed = true, onServer = false))
    }

    @Test
    fun aRendererEndedForMemoryReopensTheServerThatWasOpen() {
        assertEquals(AfterRenderer.Reopen, afterRendererGone(crashed = false, onServer = true))
    }

    @Test
    fun aRendererEndedOnTheListBringsTheListBack() {
        assertEquals(AfterRenderer.Launcher, afterRendererGone(crashed = false, onServer = false))
    }
}
