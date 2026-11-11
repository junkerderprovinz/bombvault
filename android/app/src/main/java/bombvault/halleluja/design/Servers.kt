package bombvault.halleluja.design

import android.content.Context
import android.net.Uri
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/**
 * A BombVault the app opens. [pin] is the SHA-256 of the certificate the
 * person trusted, for a server whose certificate Android cannot verify.
 * [member] is its instance id when the server came from the paired group,
 * and [seen] the Unix second the group last had it in reach, 0 when unknown.
 */
data class Server(
    val id: String,
    val name: String,
    val url: String,
    val pin: String?,
    val member: String? = null,
    val seen: Long = 0,
) {
    val origin: String get() = originOf(url)

    /** Whether the app has a way in: the own address, or the relay for a member of the group. */
    val opens: Boolean get() = url.isNotEmpty() || member != null

    /** The fields the launcher sees; the pin stays in the app. */
    fun toJson(): JSONObject =
        JSONObject().put("id", id).put("name", name).put("url", url).put("member", member != null).put("seen", seen)
}

/** The scheme, host and port of [url], the way the WebView reports an origin. */
fun originOf(url: String): String {
    val u = Uri.parse(url)
    val port = if (u.port == -1) "" else ":${u.port}"
    return "${u.scheme}://${u.host}$port"
}

/**
 * Splits [list] into the servers the app keeps and the names of those saved
 * by address alone, which only a pairing brings back.
 */
fun withoutAddressOnly(list: List<Server>): Pair<List<Server>, List<String>> {
    val (kept, gone) = list.partition { it.member != null }
    return kept to gone.map { it.name }
}

/** The server list, kept in the app's own preferences. */
class Servers(context: Context) {
    private val prefs = context.getSharedPreferences("servers", Context.MODE_PRIVATE)

    @Synchronized
    fun all(): List<Server> {
        val list = JSONArray(prefs.getString(KEY, "[]"))
        return (0 until list.length()).map {
            val o = list.getJSONObject(it)
            Server(
                o.getString("id"),
                o.getString("name"),
                o.getString("url"),
                o.optString("pin").ifEmpty { null },
                o.optString("member").ifEmpty { null },
                o.optLong("seen"),
            )
        }
    }

    fun get(id: String): Server? = all().firstOrNull { it.id == id }

    /** The server a page at [origin] belongs to, at its own address or over the relay. */
    fun forOrigin(origin: String): Server? = all().firstOrNull {
        (it.url.isNotEmpty() && it.origin == origin) || (it.member != null && relayOrigin(it.member) == origin)
    }

    fun forMember(member: String): Server? = all().firstOrNull { it.member == member }

    /**
     * Puts group member [member] in the list, last in reach at [seen], under
     * the name and address the group knows it by. An empty [url] keeps the
     * address already there.
     */
    @Synchronized
    fun adopt(member: String, name: String, url: String, seen: Long) {
        val list = all()
        val known = list.firstOrNull { it.member == member }
        if (known == null) {
            write(list + Server(UUID.randomUUID().toString(), name, url, null, member, seen))
            return
        }
        val next = url.ifEmpty { known.url }
        // Another address is another certificate, so the trust does not move with it.
        val pin = known.pin?.takeIf { known.origin == originOf(next) }
        val updated = known.copy(name = name.ifEmpty { known.name }, url = next, pin = pin, seen = maxOf(known.seen, seen))
        write(list.map { if (it.id == known.id) updated else it })
    }

    /** Drops the servers saved by address alone and keeps their names for [addressGone]. */
    @Synchronized
    fun letGoOfAddresses() {
        val (kept, gone) = withoutAddressOnly(all())
        if (gone.isEmpty()) return
        write(kept)
        prefs.edit().putString(GONE, JSONArray(addressGone() + gone).toString()).apply()
    }

    /** The names of the servers [letGoOfAddresses] dropped, until [dismissAddressGone]. */
    fun addressGone(): List<String> {
        val names = JSONArray(prefs.getString(GONE, "[]"))
        return (0 until names.length()).map { names.getString(it) }
    }

    fun dismissAddressGone() = prefs.edit().remove(GONE).apply()

    @Synchronized
    fun removeAll() = write(emptyList())

    /** The key group member [member] signs with, as the phone pinned it. */
    fun memberKey(member: String): ByteArray? =
        JSONObject(prefs.getString(KEYS, "{}")!!).optString(member).takeIf { it.isNotEmpty() }?.let { PageWire.unb64(it) }

    /** Pins [key] for [member], unless one is pinned already and [replace] is not set. */
    @Synchronized
    fun pinMemberKey(member: String, key: ByteArray, replace: Boolean = false) {
        val keys = JSONObject(prefs.getString(KEYS, "{}")!!)
        if (keys.has(member) && !replace) return
        prefs.edit().putString(KEYS, keys.put(member, PageWire.b64(key)).toString()).apply()
    }

    @Synchronized
    fun forgetMemberKeys() = prefs.edit().remove(KEYS).apply()

    @Synchronized
    fun trust(id: String, pin: String) = write(all().map { if (it.id == id) it.copy(pin = pin) else it })

    private fun write(list: List<Server>) {
        val out = JSONArray()
        for (s in list) out.put(s.toJson().put("pin", s.pin ?: "").put("member", s.member ?: ""))
        prefs.edit().putString(KEY, out.toString()).apply()
    }

    companion object {
        private const val KEY = "list"
        private const val KEYS = "memberKeys"
        private const val GONE = "addressGone"
    }
}
