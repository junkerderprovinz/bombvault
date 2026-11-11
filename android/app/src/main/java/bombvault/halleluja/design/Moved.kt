package bombvault.halleluja.design

import org.json.JSONObject
import java.util.Base64

/**
 * Moved is how the phone follows its group into a fortress. An instance whose
 * group moved hands each paired phone the keys of the pairing there, over the
 * group on the old phrase, sealed and signed like every answer in it. The
 * phone keeps them in place of its words and never learns the fortress
 * phrase, which opens every app of the fortress. The answer carries the
 * pairing's key for direct calls as well, which the phone does not make.
 */
object Moved {
    const val PATH = "/kit/fortress/v1/moved"

    private val GROUP = Regex("^[0-9a-f]{64}$")

    /** The keys [body] hands over, or null for anything that is no pair of keys. */
    fun parse(body: String): GroupKeys? {
        val o = runCatching { JSONObject(body) }.getOrNull() ?: return null
        return keys(o.optString("group"), o.optString("frame"))
    }

    /** [keys] as the phone keeps them. */
    fun store(keys: GroupKeys): String =
        JSONObject().put("group", keys.relayKey).put("frame", Base64.getEncoder().encodeToString(keys.frameKey)).toString()

    /** The keys [kept] holds, or null. */
    fun load(kept: String): GroupKeys? {
        val o = runCatching { JSONObject(kept) }.getOrNull() ?: return null
        return keys(o.optString("group"), o.optString("frame"))
    }

    private fun keys(group: String, frame: String): GroupKeys? {
        if (!GROUP.matches(group)) return null
        val f = runCatching { Base64.getDecoder().decode(frame) }.getOrNull() ?: return null
        return if (f.size == 32) GroupKeys(group, f) else null
    }
}
