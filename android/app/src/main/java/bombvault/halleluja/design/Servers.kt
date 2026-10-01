package bombvault.halleluja.design

import android.content.Context
import android.net.Uri
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/**
 * A BombVault the app opens. [pin] is the SHA-256 of the certificate the
 * person trusted, for a server whose certificate Android cannot verify.
 */
data class Server(val id: String, val name: String, val url: String, val pin: String?) {
    val origin: String get() = originOf(url)

    /** The fields the launcher sees; the pin stays in the app. */
    fun toJson(): JSONObject = JSONObject().put("id", id).put("name", name).put("url", url)
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

    fun all(): List<Server> {
        val list = JSONArray(prefs.getString(KEY, "[]"))
        return (0 until list.length()).map {
            val o = list.getJSONObject(it)
            Server(o.getString("id"), o.getString("name"), o.getString("url"), o.optString("pin").ifEmpty { null })
        }
    }

    fun get(id: String): Server? = all().firstOrNull { it.id == id }

    fun forOrigin(origin: String): Server? = all().firstOrNull { it.origin == origin }

    /** Saves [name] and [url] under [id], or as a new server, and returns it. */
    fun save(id: String?, name: String, url: String): Server {
        val list = all()
        val old = list.firstOrNull { it.id == id }
        // Another address is another certificate, so the trust does not move with it.
        val pin = old?.pin?.takeIf { old.origin == originOf(url) }
        val server = Server(old?.id ?: UUID.randomUUID().toString(), name, url, pin)
        write(if (old == null) list + server else list.map { if (it.id == server.id) server else it })
        return server
    }

    fun remove(id: String) = write(all().filter { it.id != id })

    fun trust(id: String, pin: String) = write(all().map { if (it.id == id) it.copy(pin = pin) else it })

    private fun write(list: List<Server>) {
        val out = JSONArray()
        for (s in list) out.put(s.toJson().put("pin", s.pin ?: ""))
        prefs.edit().putString(KEY, out.toString()).apply()
    }

    companion object {
        private const val KEY = "list"
    }
}
