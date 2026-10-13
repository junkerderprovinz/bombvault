package bombvault.halleluja.design

import kotlinx.coroutines.Deferred
import kotlinx.coroutines.selects.select
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.ConcurrentHashMap

/** The way the app takes to a server's page. */
enum class Way { Direct, Relay, None }

/**
 * What the app last learned about reaching each server at its own address,
 * from the launcher's questions and from opening it. Away from home the own
 * address only times out, and knowing that spares every open the wait.
 */
class Routes(private val now: () -> Long = System::currentTimeMillis) {
    private val heard = ConcurrentHashMap<String, Pair<Boolean, Long>>()

    fun heard(id: String, answered: Boolean) {
        heard[id] = answered to now()
    }

    /** Whether [id] answered at its own address lately, null when the app has not asked lately. */
    fun direct(id: String): Boolean? = heard[id]?.takeIf { now() - it.second < FRESH_MS }?.first

    /** Drops what was learned, once the phone may have changed networks. */
    fun forget() = heard.clear()

    companion object {
        // The launcher asks every ten seconds while it is on screen.
        const val FRESH_MS = 30_000L
    }
}

/**
 * The way in from the own address and the relay, tried at once. The own
 * address gets [headStart] before a relay that is ready wins: at home it is
 * faster and reaches the sign-in and the downloads.
 */
suspend fun pickWay(direct: Deferred<Boolean>?, relay: Deferred<Boolean>?, headStart: Long): Way {
    if (direct != null && withTimeoutOrNull(headStart) { direct.await() } == true) return Way.Direct
    val left = listOfNotNull(direct?.let { it to Way.Direct }, relay?.let { it to Way.Relay }).toMutableList()
    while (left.isNotEmpty()) {
        val (way, ok) = select { for ((d, way) in left) d.onAwait { way to it } }
        if (ok) return way
        left.removeAll { it.second == way }
    }
    return Way.None
}

/**
 * The first of [asks] to answer with 200. When none does, the first in the
 * order given that answered at all, so a slow group still beats an own
 * address that only says the session is missing.
 */
suspend fun firstAnswer(asks: List<Deferred<Answer?>>): Answer? {
    val left = asks.toMutableList()
    while (left.isNotEmpty()) {
        val (d, a) = select { for (d in left) d.onAwait { d to it } }
        if (a?.status == 200) return a
        left.remove(d)
    }
    return asks.firstNotNullOfOrNull { it.await() }
}

/**
 * How long [host] gets to take a connection. A machine on the local network
 * answers within moments or is not there; away from home its address is
 * dropped without a word, and only the timeout ends the wait.
 */
fun connectTimeout(host: String?): Int = if (host != null && isLocal(host)) LAN_CONNECT_MS else CONNECT_MS

private fun isLocal(host: String): Boolean {
    val parts = host.split('.').map { it.toIntOrNull() ?: return false }
    if (parts.size != 4 || parts.any { it !in 0..255 }) return false
    val (a, b) = parts
    return a == 10 || (a == 172 && b in 16..31) || (a == 192 && b == 168) || (a == 169 && b == 254)
}

private const val LAN_CONNECT_MS = 1500
private const val CONNECT_MS = 5000
