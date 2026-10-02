package bombvault.halleluja.design

import android.content.Context
import android.net.Uri
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/**
 * A BombVault the app opens. [pin] is the SHA-256 of the certificate the
 * person trusted, for a server whose certificate Android cannot verify.
 * [member] is its instance id when the server came from the paired group.
 */
data class Server(val id: String, val name: String, val url: String, val pin: String?, val member: String? = null) {
    val origin: String get() = originOf(url)

    /** The fields the launcher sees; the pin stays in the app. */
    fun toJson(): JSONObject = JSONObject().put("id", id).put("name", name).put("url", url).put("member", member != null)
}

/** The scheme, host and port of [url], the way the WebView reports an origin. */
fun originOf(url: String): String {
    val u = Uri.parse(url)
    val port = if (u.port == -1) "" else ":${u.port}"
    return "${u.scheme}://${u.host}$port"
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
            )
        }
    }

    fun get(id: String): Server? = all().firstOrNull { it.id == id }

    fun forOrigin(origin: String): Server? = all().firstOrNull { it.url.isNotEmpty() && it.origin == origin }

    /** Saves [name] and [url] under [id], or as a new server, and returns it. */
    @Synchronized
    fun save(id: String?, name: String, url: String): Server {
        val list = all()
        val old = list.firstOrNull { it.id == id }
        // Another address is another certificate, so the trust does not move with it.
        val pin = old?.pin?.takeIf { old.origin == originOf(url) }
        val server = Server(old?.id ?: UUID.randomUUID().toString(), name, url, pin, old?.member)
        write(if (old == null) list + server else list.map { if (it.id == server.id) server else it })
        return server
    }

    /**
     * Puts group member [member] in the list. A server already there under
     * that member or that address keeps its name and address, which the
     * person may have changed; one with no address yet takes [url].
     */
    @Synchronized
    fun adopt(member: String, name: String, url: String) {
        val list = all()
        val known = list.firstOrNull { it.member == member }
            ?: list.firstOrNull { url.isNotEmpty() && it.url.isNotEmpty() && it.origin == originOf(url) }
        val next = when {
            known == null -> list + Server(UUID.randomUUID().toString(), name, url, null, member)
            else -> list.map { if (it.id == known.id) it.copy(member = member, url = it.url.ifEmpty { url }) else it }
        }
        write(next)
    }

    /** Leaving the group keeps the servers as ones added by hand. */
    @Synchronized
    fun forgetMembers() = write(all().filter { it.url.isNotEmpty() }.map { it.copy(member = null) })

    @Synchronized
    fun remove(id: String) = write(all().filter { it.id != id })

    @Synchronized
    fun removeAll() = write(emptyList())

    @Synchronized
    fun trust(id: String, pin: String) = write(all().map { if (it.id == id) it.copy(pin = pin) else it })

    private fun write(list: List<Server>) {
        val out = JSONArray()
        for (s in list) out.put(s.toJson().put("pin", s.pin ?: "").put("member", s.member ?: ""))
        prefs.edit().putString(KEY, out.toString()).apply()
    }

    companion object {
        private const val KEY = "list"
    }
}
