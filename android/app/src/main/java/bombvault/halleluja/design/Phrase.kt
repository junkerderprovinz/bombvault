package bombvault.halleluja.design

import android.content.Context
import java.security.MessageDigest

/** Why twelve words did not decode, in the terms BombVault's own pairing card uses. */
sealed class PhraseError {
    data class WordCount(val count: Int) : PhraseError()
    data class UnknownWord(val word: String, val position: Int) : PhraseError()
    data object Checksum : PhraseError()
}

/**
 * Phrase decodes the twelve pairing words into the group secret, the way
 * internal/seedphrase does on the server: BIP39 English, 128 bits and a 4-bit
 * checksum.
 */
class Phrase(words: List<String>) {
    constructor(context: Context) : this(context.assets.open("bip39-english.txt").bufferedReader().readLines().filter { it.isNotBlank() })

    private val index = words.withIndex().associate { (i, w) -> w to i }

    fun decode(phrase: String): Result<ByteArray> {
        val got = phrase.lowercase().split(Regex("\\s+")).filter { it.isNotEmpty() }
        if (got.size != WORDS) return Result.failure(Refused(PhraseError.WordCount(got.size)))
        val full = ByteArray(SECRET_LEN + 1)
        for ((i, w) in got.withIndex()) {
            val idx = index[w] ?: return Result.failure(Refused(PhraseError.UnknownWord(w, i + 1)))
            for (b in 0 until BITS) {
                if (idx and (1 shl (BITS - 1 - b)) != 0) {
                    val bit = i * BITS + b
                    full[bit / 8] = (full[bit / 8].toInt() or (1 shl (7 - bit % 8))).toByte()
                }
            }
        }
        val secret = full.copyOf(SECRET_LEN)
        val want = (MessageDigest.getInstance("SHA-256").digest(secret)[0].toInt() and 0xff) shr (8 - CHECKSUM_BITS)
        val have = (full[SECRET_LEN].toInt() and 0xff) shr (8 - CHECKSUM_BITS)
        return if (want == have) Result.success(secret) else Result.failure(Refused(PhraseError.Checksum))
    }

    class Refused(val reason: PhraseError) : Exception()

    companion object {
        private const val WORDS = 12
        private const val BITS = 11
        private const val SECRET_LEN = 16
        private const val CHECKSUM_BITS = 4
    }
}
