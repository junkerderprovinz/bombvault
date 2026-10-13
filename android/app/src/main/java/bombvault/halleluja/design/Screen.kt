package bombvault.halleluja.design

/**
 * Screen keeps which server the app shows and which one it is on its way to.
 * The page on screen stays there while the app finds a way to that one, and
 * a step through its own history in that time does not call the switch off.
 */
class Screen {
    /** The server whose page is on screen, null while the launcher is. */
    private var shown: Server? = null

    /** The server being opened, until its page is on screen. */
    private var opening: Server? = null

    /** The server the app is busy with: the one it opens, else the one it shows. */
    val current: Server? get() = opening ?: shown

    fun open(server: Server) {
        opening = server
    }

    /** Whether [server] is still the one to load once the app knows its way in. */
    fun opens(server: Server) = opening?.id == server.id

    /** Gives up the server on its way, and says whether there was one. */
    fun cancel(): Boolean = (opening != null).also { opening = null }

    fun launcher() {
        shown = null
        opening = null
    }

    /**
     * Takes the page of [server], or of none of the list for null, as the one
     * on screen. While another server is on its way the page is left out and
     * the answer is false.
     */
    fun shows(server: Server?): Boolean {
        val going = opening
        if (going != null && server?.id != going.id) return false
        opening = null
        if (server != null) shown = server
        return true
    }
}
