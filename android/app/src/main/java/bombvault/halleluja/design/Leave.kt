package bombvault.halleluja.design

import org.json.JSONObject

/**
 * The word the phone gives when it leaves the group, the way
 * internal/api/group.go reads it. It is signed with the phone's own key, so
 * another holder of the words cannot take the phone off a list.
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
}
