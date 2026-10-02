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
import org.json.JSONException
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

    // A group being looked at before it is taken over: its relay, and what
    // joining it would store.
    private var preview: Relay? = null
    private var previewSecret: ByteArray? = null
    private var previewUrl = PROJECT_RELAY

    val paired: Boolean get() = prefs.contains(SECRET)
    val connected: Boolean get() = relay?.connected == true

    /** The instances of the group being looked at, or null when none is. */
    val joining: List<Sibling>? get() = preview?.siblings?.filter { it.kind.isEmpty() }

    /**
     * Looks at the group of [code], twelve words and an own relay's address on
     * a second line, without joining it yet: its instances turn up in
     * [joining] for the person to take over.
     */
    fun join(code: String): PhraseError? {
        val lines = code.trim().lines().map { it.trim() }.filter { it.isNotEmpty() }
        val secret = Phrase(context).decode(lines.firstOrNull() ?: "").getOrElse { return (it as Phrase.Refused).reason }
        cancelJoin()
        previewSecret = secret
        previewUrl = lines.getOrNull(1) ?: PROJECT_RELAY
        preview = open(secret, previewUrl)
        return null
    }

    /** Joins the group being looked at and takes its instances over. */
    fun adopt() {
        val secret = previewSecret ?: return
        prefs.edit().putString(SECRET, wrap(secret)).putString(RELAY, previewUrl).apply()
        stop()
        relay = preview
        preview = null
        previewSecret = null
        adoptMembers()
        onChange()
    }

    fun cancelJoin() {
        preview?.stop()
        preview = null
        previewSecret = null
    }

    fun leave() {
        stop()
        prefs.edit().remove(SECRET).remove(RELAY).apply()
        servers.forgetMembers()
        onChange()
    }

    fun start() {
        if (relay != null || !paired) return
        relay = open(unwrap(prefs.getString(SECRET, null) ?: return), prefs.getString(RELAY, PROJECT_RELAY)!!)
    }

    fun stop() {
        relay?.stop()
        relay = null
        asked.clear()
    }

    private fun open(secret: ByteArray, url: String): Relay {
        val id = prefs.getString(ID, null) ?: randomId().also { prefs.edit().putString(ID, it).apply() }
        lateinit var r: Relay
        r = Relay(url, GroupKeys(secret), id, identity(), ::serve) {
            if (r === relay) adoptMembers()
            onChange()
        }
        r.start()
        return r
    }

    /** The name the phone goes by in the group, empty while it uses its own. */
    val deviceName: String get() = prefs.getString(NAME, "") ?: ""

    /** The name the person gave the phone, used while none is chosen in the app. */
    val phoneName: String get() = Settings.Global.getString(context.contentResolver, "device_name") ?: Build.MODEL

    /** Renames the phone and announces it again, so every instance shows the new name. */
    fun rename(name: String) {
        prefs.edit().putString(NAME, name).apply()
        if (relay == null) return
        stop()
        start()
    }

    /**
     * GETs [path] from [server] through the group, or returns null when the
     * group cannot carry the question.
     */
    suspend fun ask(server: Server, path: String): Answer? {
        val r = relay ?: return null
        val member = server.member ?: return null
        if (r.siblings.none { it.id == member }) return null
        return try {
            val (status, body) = r.call(member, "GET", path)
            // An instance older than the route has nothing to say here, but
            // its own address may.
            if (status == 404) null else Answer(status, String(body))
        } catch (_: IOException) {
            null
        }
    }

    /**
     * The session cookie [server] gives this phone, as a Set-Cookie value, so
     * its page opens signed in. Null when the group cannot ask, the server
     * has no password, or it does not list this phone on its Instances page.
     */
    suspend fun session(server: Server): String? {
        val r = relay ?: return null
        val member = server.member ?: return null
        if (r.siblings.none { it.id == member }) return null
        val ask = JSONObject().put("instanceId", prefs.getString(ID, "")).toString().toByteArray()
        return try {
            val (status, body) = r.call(member, "POST", "/api/group/peer/session", ask)
            if (status != 200) return null
            val s = JSONObject(String(body))
            if (!s.optBoolean("needed")) return null
            buildString {
                append("${s.getString("name")}=${s.getString("value")}; Path=/; Max-Age=${s.optInt("maxAge")}; HttpOnly; SameSite=Lax")
                if (s.optBoolean("secure")) append("; Secure")
            }
        } catch (_: IOException) {
            null
        } catch (_: JSONException) {
            null
        }
    }

    /** Asks each instance that turned up where it takes calls and puts it in the list. */
    private fun adoptMembers() {
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
        // What the Instances card shows.
        val name = deviceName.ifEmpty { phoneName }
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
        private const val NAME = "name"
    }
}
