package bombvault.halleluja.design

import org.json.JSONObject
import kotlin.math.abs

/**
 * The word a member gives when it leaves the group, the way
 * internal/api/group.go writes and reads it. It is signed with the member's
 * own key, so another holder of the words cannot take a member off a list.
 */
object Leave {
    const val PATH = "/api/group/peer/leave"

    /** The body that tells [member] the phone [me] leaves, signed with [seed] at [at]. */
    fun notice(seed: ByteArray, me: String, member: String, at: Long): ByteArray =
        JSONObject()
            .put("key", PageWire.b64(Proof.publicKey(seed)))
            .put("counter", at)
            .put("proof", PageWire.b64(Proof.sign(seed, Proof.message("leave", me, member, at))))
            .toString()
            .toByteArray()

    /**
     * Whether [body] is the word of [sender] to [me] that it left the group,
     * signed with [pinned], the key the phone pinned for it, and recent at [now].
     */
    fun heard(pinned: ByteArray?, sender: String, me: String, body: ByteArray, now: Long): Boolean {
        if (pinned == null) return false
        val o = runCatching { JSONObject(String(body)) }.getOrNull() ?: return false
        val at = o.optLong("counter")
        if (abs(now - at) > MAX_AGE_MS) return false
        val key = runCatching { PageWire.unb64(o.optString("key")) }.getOrNull() ?: return false
        val proof = runCatching { PageWire.unb64(o.optString("proof")) }.getOrNull() ?: return false
        return key.contentEquals(pinned) && Proof.verify(pinned, Proof.message("leave", sender, me, at), proof)
    }

    // The relay turns away a call sent longer ago than this as well.
    private const val MAX_AGE_MS = 120_000L
}
