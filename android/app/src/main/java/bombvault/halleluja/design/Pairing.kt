package bombvault.halleluja.design

import android.content.Context
import android.os.Build
import android.provider.Settings
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.io.IOException
import java.security.KeyStore
import java.security.SecureRandom
import java.util.concurrent.ConcurrentHashMap
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Pairing makes the app a member of the BombVault group whose twelve words it
 * was given. It joins over the relay only, adds every instance of the group to
 * the server list and reads their activity through the group, which works
 * without a session and away from home.
 */
class Pairing(private val context: Context, private val servers: Servers, private val onChange: () -> Unit) {
    private val prefs = context.getSharedPreferences("group", Context.MODE_PRIVATE)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val asked = ConcurrentHashMap.newKeySet<String>()
    private var relay: Relay? = null

    val paired: Boolean get() = prefs.contains(SECRET)
    val connected: Boolean get() = relay?.connected == true

    /** Joins the group of [code]: twelve words, and an own relay's address on a second line. */
    fun join(code: String): PhraseError? {
        val lines = code.trim().lines().map { it.trim() }.filter { it.isNotEmpty() }
        val secret = Phrase(context).decode(lines.firstOrNull() ?: "").getOrElse { return (it as Phrase.Refused).reason }
        prefs.edit()
            .putString(SECRET, wrap(secret))
            .putString(RELAY, lines.getOrNull(1) ?: PROJECT_RELAY)
            .putString(ID, prefs.getString(ID, null) ?: randomId())
            .apply()
        stop()
        start()
        return null
    }

    fun leave() {
        stop()
        prefs.edit().remove(SECRET).remove(RELAY).apply()
        servers.forgetMembers()
        onChange()
    }

    fun start() {
        if (relay != null || !paired) return
        val secret = unwrap(prefs.getString(SECRET, null) ?: return)
        val r = Relay(prefs.getString(RELAY, PROJECT_RELAY)!!, GroupKeys(secret), prefs.getString(ID, null)!!, identity(), ::serve) {
            adopt()
            onChange()
        }
        relay = r
        r.start()
    }

    fun stop() {
        relay?.stop()
        relay = null
        asked.clear()
    }

    /** What runs on [server], asked through the group, or null when the group cannot carry the question. */
    suspend fun activity(server: Server): Answer? {
        val r = relay ?: return null
        val member = server.member ?: return null
        if (r.siblings.none { it.id == member }) return null
        return try {
            val (status, body) = r.call(member, "GET", "/api/group/peer/activity")
            // An instance older than the route has nothing to say here, but
            // its own address may.
            if (status == 404) null else Answer(status, String(body))
        } catch (_: IOException) {
            null
        }
    }

    /** Asks each instance that turned up where it takes calls and puts it in the list. */
    private fun adopt() {
        val r = relay ?: return
        for (s in r.siblings) {
            if (s.kind.isNotEmpty() || !asked.add(s.id)) continue
            scope.launch {
                val url = try {
                    val (status, body) = r.call(s.id, "GET", "/api/group/peer/hello")
                    if (status == 200) JSONObject(String(body)).optString("directUrl") else ""
                } catch (_: Exception) {
                    asked.remove(s.id)
                    return@launch
                }
                servers.adopt(s.id, s.name, if (url.isEmpty()) "" else url.trimEnd('/') + "/")
                onChange()
            }
        }
    }

    /** The app answers the hello a probe asks for, and nothing else. */
    private fun serve(method: String, path: String): Pair<Int, ByteArray> {
        if (method != "GET" || path != "/api/group/peer/hello") return 404 to ByteArray(0)
        val hello = identity().put("ok", true).put("instanceId", prefs.getString(ID, ""))
        return 200 to hello.toString().toByteArray()
    }

    private fun identity(): JSONObject {
        // The name the person gave the phone, which is what the Instances card shows.
        val name = Settings.Global.getString(context.contentResolver, "device_name") ?: Build.MODEL
        val version = context.packageManager.getPackageInfo(context.packageName, 0).versionName ?: ""
        return JSONObject().put("name", name).put("version", version).put("kind", "android")
    }

    // The secret opens every member's backups, so it is kept sealed by a key
    // that never leaves the phone's keystore.
    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(KEY_ALIAS, null) as SecretKey?)?.let { return it }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        gen.init(
            KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build(),
        )
        return gen.generateKey()
    }

    private fun wrap(secret: ByteArray): String {
        val c = Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key()) }
        return Base64.encodeToString(c.iv + c.doFinal(secret), Base64.NO_WRAP)
    }

    private fun unwrap(stored: String): ByteArray {
        val raw = Base64.decode(stored, Base64.DEFAULT)
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, raw.copyOf(12)))
        return c.doFinal(raw, 12, raw.size - 12)
    }

    private fun randomId() = ByteArray(16).also(SecureRandom()::nextBytes).joinToString("") { "%02x".format(it) }

    companion object {
        private const val SECRET = "secret"
        private const val RELAY = "relay"
        private const val ID = "id"
        private const val KEY_ALIAS = "group"
    }
}
