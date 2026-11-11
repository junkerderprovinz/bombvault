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
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withTimeoutOrNull
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
 * was given. It joins over the relay only, waits until an instance lets it
 * in, adds every instance of the group to the server list and reads their
 * activity through the group, which works without a session and away from
 * home.
 *
 * An instance goes by two ids: the one BombVault knows it by, which the
 * server list and every signed word use, and the one its key makes on the
 * relay, which calls are addressed to.
 */
class Pairing(
    private val context: Context,
    private val servers: Servers,
    private val onChange: () -> Unit,
) {
    private val prefs = context.getSharedPreferences("group", Context.MODE_PRIVATE)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val asked = ConcurrentHashMap.newKeySet<String>()

    // The instances asked on this connection whether the group moved into a fortress.
    private val askedMoved = ConcurrentHashMap.newKeySet<String>()
    private var relay: Relay? = null

    // A page asks for its bundles in parallel, each in parts; the relay takes
    // a few dozen calls of one member at a time.
    private val pageCalls = Semaphore(PAGE_CALLS)

    // The session each member gave the phone for its pages, empty for one
    // without a password.
    private val sessions = ConcurrentHashMap<String, String>()
    private val sessionLock = Mutex()

    @Volatile private var seed: ByteArray? = null

    // The relay id of each instance, by the id BombVault knows it by.
    private val relayIds = ConcurrentHashMap<String, String>()

    // The instances that answered that this phone waits for their approval.
    private val waitingOn = ConcurrentHashMap.newKeySet<String>()

    // Whether the relay connection read the group's roster from an instance.
    @Volatile private var rosterRead = false

    // A group being looked at before it is taken over: its relay, and what
    // joining it would store.
    private var preview: Relay? = null
    private var previewSecret: ByteArray? = null
    private var previewUrl = PROJECT_RELAY

    val paired: Boolean get() = prefs.contains(SECRET) || prefs.contains(KEYS)
    val connected: Boolean get() = relay?.connected == true

    /** The instances of the group being looked at, or null when none is. */
    val joining: List<Sibling>? get() = preview?.siblings

    /** Whether an instance said that this phone waits to be let in there. */
    val waiting: Boolean get() = waitingOn.isNotEmpty()

    /** Whether the relay of the group being looked at answers. */
    val joiningConnected: Boolean get() = preview?.connected == true

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
        preview = open(GroupKeys(secret), previewUrl)
        return null
    }

    /** Joins the group being looked at and takes its instances over. */
    fun adopt() {
        val secret = previewSecret ?: return
        prefs.edit().putString(SECRET, wrap(secret)).remove(KEYS).putString(RELAY, previewUrl).apply()
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

    /**
     * Leaves the group, and its servers with it. The relay stays open a moment
     * longer, so the instances in reach hear it and take the phone off their
     * Instances page.
     */
    fun leave() {
        val r = relay
        relay = null
        stop()
        prefs.edit().remove(SECRET).remove(KEYS).remove(RELAY).apply()
        servers.removeAll()
        servers.forgetMemberKeys()
        onChange()
        if (r != null) {
            scope.launch {
                tellLeaving(r)
                r.stop()
            }
        }
    }

    // An instance that does not hear keeps the phone listed until a person
    // removes it there.
    private suspend fun tellLeaving(r: Relay) {
        val me = myId()
        val seed = signSeed()
        val known = relayIds.entries.associate { (id, relayId) -> relayId to id }
        withTimeoutOrNull(LEAVE_WAIT_MS) {
            coroutineScope {
                r.siblings.mapNotNull { s -> known[s.id]?.let { s.id to it } }.map { (relayId, member) ->
                    async { runCatching { r.call(relayId, "POST", Leave.PATH, Leave.notice(seed, me, member, System.currentTimeMillis())) } }
                }.awaitAll()
            }
        }
    }

    fun start() {
        if (relay != null || !paired) return
        relay = open(groupKeys() ?: return, prefs.getString(RELAY, PROJECT_RELAY)!!)
    }

    /** The keys of the group: those it handed over on moving into a fortress, else the ones its words give. */
    private fun groupKeys(): GroupKeys? {
        prefs.getString(KEYS, null)?.let { return Moved.load(String(unwrap(it))) }
        return prefs.getString(SECRET, null)?.let { GroupKeys(unwrap(it)) }
    }

    fun stop() {
        relay?.stop()
        relay = null
        rosterRead = false
        asked.clear()
        askedMoved.clear()
        sessions.clear()
        waitingOn.clear()
    }

    private fun open(keys: GroupKeys, url: String): Relay {
        val key = MemberKey(signSeed())
        // A phone that joined before keeps the id the instances know it by.
        if (!prefs.contains(ID)) prefs.edit().putString(ID, key.phoneId).apply()
        lateinit var r: Relay
        r = Relay(url, keys, key, deviceName.ifEmpty { phoneName }) {
            if (r === relay) adoptMembers()
            onChange()
        }
        r.start()
        return r
    }

    /**
     * The relay id of [member]: the one its hello came from, or else the one
     * the key pinned for it makes, since an instance signs with that key.
     */
    private fun relayId(member: String): String? =
        relayIds[member] ?: servers.memberKey(member)?.let(MemberKey::instanceId)

    /** Notes whether [reply] of [member] says that this phone waits to be let in there. */
    private fun note(member: String, reply: Reply) {
        val changed = if (reply.refused == "memberPending") waitingOn.add(member) else waitingOn.remove(member)
        if (changed) onChange()
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
        val target = relayId(member) ?: return null
        if (!r.reaches(target)) return null
        return try {
            val reply = r.call(target, "GET", path)
            note(member, reply)
            // An instance older than the route has nothing to say here, but
            // its own address may.
            if (reply.status == 404) null else Answer(reply.status, String(reply.body))
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
        val target = relayId(member) ?: return null
        if (!r.reaches(target)) return null
        return try {
            val s = askSession(r, member, target) ?: return null
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

    /** Asks [member], at [target] on the relay, for a session, signed with the phone's own key. */
    private suspend fun askSession(r: Relay, member: String, target: String): JSONObject? {
        val me = myId()
        val at = System.currentTimeMillis()
        val key = signSeed()
        val proof = Proof.sign(key, Proof.message("session", me, member, at, PageWire.payload(me)))
        val ask = JSONObject().put("instanceId", me).put("key", PageWire.b64(Proof.publicKey(key))).put("counter", at).put("proof", PageWire.b64(proof))
        val reply = r.call(target, "POST", "/api/group/peer/session", ask.toString().toByteArray())
        note(member, reply)
        return if (reply.status == 200) JSONObject(String(reply.body)) else null
    }

    /** Fetches a session for [member]'s pages unless one newer than [ended] is there. */
    private suspend fun renewSession(r: Relay, member: String, target: String, ended: String?) = sessionLock.withLock {
        if (sessions[member] != ended) return@withLock
        val s = askSession(r, member, target) ?: return@withLock
        sessions[member] = if (s.optBoolean("needed")) s.getString("value") else ""
    }

    /** The key [member], at [target] on the relay, signs with as it names it now, if it proves it holds it. */
    private suspend fun fetchKey(r: Relay, member: String, target: String): ByteArray? {
        val reply = r.call(target, "GET", "/api/group/peer/key")
        if (reply.status != 200) return null
        val o = JSONObject(String(reply.body))
        val key = PageWire.unb64(o.optString("key"))
        return key.takeIf { Proof.verify(it, Proof.message("key", member, myId(), 0), PageWire.unb64(o.optString("proof"))) }
    }

    /**
     * The key pinned for [member], pinned now if the phone has none yet. A
     * member that names another key later is not taken at its word: the
     * person sees the new key and decides.
     */
    private suspend fun memberKey(r: Relay, member: String, target: String): ByteArray? =
        servers.memberKey(member) ?: fetchKey(r, member, target)?.also { servers.pinMemberKey(member, it) }

    /** Whether group member [member] answers on the relay. */
    suspend fun reaches(member: String): Boolean {
        val target = relayId(member) ?: return false
        return relay?.reaches(target) == true
    }

    /**
     * One request of [server]'s interface through the group, with every part
     * of its answer. The answer counts only signed with the key pinned for
     * the member, so another holder of the words in its place on the relay
     * cannot answer for it.
     */
    suspend fun page(server: Server, method: String, path: String, header: Map<String, String>, body: ByteArray?): PageResult {
        val r = relay ?: return PageResult.Failed
        val member = server.member ?: return PageResult.Failed
        val target = relayId(member) ?: return PageResult.Failed
        if (!r.reaches(target)) return PageResult.Failed
        return try {
            val key = memberKey(r, member, target) ?: return PageResult.Failed
            if (!sessions.containsKey(member)) renewSession(r, member, target, null)
            val ask = PageWire.ask(method, path, header, body)
            var session = sessions[member]
            var at = System.currentTimeMillis()
            var answered = pageCall(r, member, target, ask, session, at)
            if (answered.first == 401) {
                // The session ended, through a sign-out everywhere or a reset.
                renewSession(r, member, target, session)
                session = sessions[member]
                at = System.currentTimeMillis()
                answered = pageCall(r, member, target, ask, session, at)
            }
            val head = answered.second ?: return PageResult.Failed
            val rest = coroutineScope {
                PageWire.offsets(head).map { offset -> async { pageCall(r, member, target, PageWire.part(head.part, offset), session).second?.body } }.awaitAll()
            }
            if (rest.any { it == null }) return PageResult.Failed
            val wire = PageWire.assemble(head, rest.filterNotNull())
            val signed = PageWire.digest(ask, head.status, head.header, wire)
            if (!Proof.verify(key, Proof.message("page-answer", member, myId(), at, signed), head.proof)) {
                val named = fetchKey(r, member, target)
                return if (named != null && !named.contentEquals(key)) PageResult.KeyChanged(named) else PageResult.Failed
            }
            PageResult.Done(PageWire.unpack(head.status, head.header, wire), key)
        } catch (_: Exception) {
            // A call that timed out, an answer that does not read: the page
            // gets a failed request, never a crash of the WebView's thread.
            PageResult.Failed
        } catch (_: OutOfMemoryError) {
            PageResult.Failed
        }
    }

    private suspend fun pageCall(
        r: Relay,
        member: String,
        target: String,
        ask: ByteArray,
        session: String?,
        at: Long = System.currentTimeMillis(),
    ): Pair<Int, PageWire.Piece?> {
        val call = PageWire.call(ask, session, signSeed(), myId(), member, at)
        val reply = pageCalls.withPermit { r.call(target, "POST", PageWire.PATH, call) }
        return reply.status to if (reply.status == 200) PageWire.piece(reply.body) else null
    }

    /** Pins [key] for [member] in place of the one it had, once the person trusted it. */
    fun trustKey(member: String, key: ByteArray) = servers.pinMemberKey(member, key, replace = true)

    /**
     * Asks each instance that turned up which id it goes by and where its
     * pages are, and puts it in the list. One that keeps the phone waiting
     * is asked again a little later.
     */
    private fun adoptMembers() {
        val r = relay ?: return
        for (s in r.siblings) {
            if (!asked.add(s.id)) continue
            scope.launch {
                val hello = try {
                    r.call(s.id, "GET", "/api/group/peer/hello")
                } catch (_: Exception) {
                    asked.remove(s.id)
                    return@launch
                }
                note(s.id, hello)
                if (hello.status != 200) {
                    delay(ASK_AGAIN_MS)
                    asked.remove(s.id)
                    if (r === relay) adoptMembers()
                    return@launch
                }
                val o = runCatching { JSONObject(String(hello.body)) }.getOrNull()
                val member = o?.optString("instanceId").orEmpty()
                if (member.isEmpty()) return@launch
                relayIds[member] = s.id
                // The key is pinned on first contact, before anybody could
                // take the member's place for a page.
                runCatching { memberKey(r, member, s.id) }
                val url = o?.optString("directUrl").orEmpty()
                servers.adopt(member, s.name, if (url.isEmpty()) "" else url.trimEnd('/') + "/", nowSeconds())
                onChange()
                follow(r, member, s.id)
            }
        }
        readRoster(r)
    }

    /**
     * Asks [member], at [target] on the relay, whether the group moved into a
     * fortress, and follows it there with the keys it hands over. They are
     * taken only from an instance whose key the phone pinned, so holding the
     * old words is not enough to send the phone elsewhere.
     */
    private fun follow(r: Relay, member: String, target: String) {
        if (!prefs.contains(SECRET) || !askedMoved.add(target)) return
        val pinned = servers.memberKey(member) ?: return
        if (MemberKey.instanceId(pinned) != target) return
        scope.launch {
            val reply = runCatching { r.call(target, "GET", Moved.PATH) }.getOrNull() ?: return@launch
            if (reply.status != 200 || r !== relay) return@launch
            val keys = Moved.parse(String(reply.body)) ?: return@launch
            prefs.edit().putString(KEYS, wrap(Moved.store(keys).toByteArray())).remove(SECRET).apply()
            stop()
            start()
            onChange()
        }
    }

    /**
     * Reads the group from the first instance on the relay that answers, once
     * per connection, so the list also holds instances the phone does not
     * meet there, such as one that was shut down when the phone joined. An
     * instance that keeps the phone waiting is passed over for the next one.
     */
    private fun readRoster(r: Relay) {
        if (rosterRead) return
        val instances = r.siblings
        if (instances.isEmpty()) return
        rosterRead = true
        scope.launch {
            for (s in instances) {
                val entries = try {
                    val reply = r.call(s.id, "GET", Roster.PATH)
                    if (reply.status != 200) continue
                    Roster.parse(String(reply.body))
                } catch (_: Exception) {
                    continue
                }
                val now = nowSeconds()
                for (e in entries) {
                    // One left behind is out of reach, so reading the roster does not count as seeing it.
                    val seen = if (e.seen > 0 || e.behind) e.seen else now
                    servers.adopt(e.id, e.name, e.url, seen, e.behind)
                }
                onChange()
                return@launch
            }
            rosterRead = false
        }
    }

    private fun nowSeconds() = System.currentTimeMillis() / 1000

    private fun myId(): String = prefs.getString(ID, "") ?: ""

    // The key the phone signs its asks with. Whoever has the words can name
    // the phone, but only the phone holds this, sealed like the secret.
    @Synchronized
    private fun signSeed(): ByteArray {
        seed?.let { return it }
        val stored = prefs.getString(SIGN, null)?.let(::unwrap)
            ?: ByteArray(32).also(SecureRandom()::nextBytes).also { prefs.edit().putString(SIGN, wrap(it)).apply() }
        seed = stored
        return stored
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

    companion object {
        private const val SECRET = "secret"

        // The keys of the pairing the group moved into a fortress with,
        // sealed like the secret they take the place of.
        private const val KEYS = "keys"
        private const val RELAY = "relay"
        private const val ID = "id"
        private const val KEY_ALIAS = "group"
        private const val NAME = "name"
        private const val SIGN = "sign"
        private const val PAGE_CALLS = 12
        private const val LEAVE_WAIT_MS = 5_000L
        private const val ASK_AGAIN_MS = 15_000L
    }
}

/** What a request of a page through the relay came to. */
sealed interface PageResult {
    /** The member signed [answer] with [key]. */
    class Done(val answer: PageAnswer, val key: ByteArray) : PageResult

    /** The member answered with another key than the one pinned for it. */
    class KeyChanged(val key: ByteArray) : PageResult

    object Failed : PageResult
}
