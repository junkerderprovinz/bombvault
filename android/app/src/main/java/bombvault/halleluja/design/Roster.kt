package bombvault.halleluja.design

import org.json.JSONObject

/**
 * The group as one of its instances sees it, the lists the Instances page
 * shows: the instances in reach there and the ones away. The phone reaches
 * only who is on the relay with it, so an instance that was shut down when
 * the phone joined, or that it never met, comes to its list this way.
 */
object Roster {
    const val PATH = "/api/group/peer/members"

    /** An instance of the group. [seen] is the Unix second it was last in reach, 0 while it is. */
    data class Entry(val id: String, val name: String, val url: String, val seen: Long)

    /** The instances [body] lists, without phones and without anything that does not read. */
    fun parse(body: String): List<Entry> {
        val o = runCatching { JSONObject(body) }.getOrNull() ?: return emptyList()
        if (!o.optBoolean("ok")) return emptyList()
        fun read(key: String, away: Boolean): List<Entry> {
            val list = o.optJSONArray(key) ?: return emptyList()
            return (0 until list.length()).mapNotNull { i ->
                val m = list.optJSONObject(i) ?: return@mapNotNull null
                val id = m.optString("id")
                if (!MEMBER.matches(id) || m.optString("kind").isNotEmpty()) return@mapNotNull null
                val address = m.optString("address")
                Entry(
                    id,
                    m.optString("name").ifEmpty { id },
                    if (address.isEmpty()) "" else address.trimEnd('/') + "/",
                    if (away) m.optLong("lastSeen") else 0,
                )
            }
        }
        return read("members", away = false) + read("away", away = true)
    }
}
