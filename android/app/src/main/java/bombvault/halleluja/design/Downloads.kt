package bombvault.halleluja.design

import android.content.ContentValues
import android.content.Context
import android.net.Uri
import android.provider.MediaStore
import android.util.Base64
import android.webkit.URLUtil
import java.io.IOException
import java.io.OutputStream

/**
 * Saves what the interface hands out (a flash ZIP, a database dump, a recovery
 * kit) into the phone's Downloads. Android's DownloadManager cannot do it: it
 * neither carries the session cookie nor accepts a self-signed certificate.
 */
object Downloads {
    /** Fetches [url] from [server] and saves it; returns the file name. */
    fun fromUrl(context: Context, server: Server, url: String, disposition: String?, mime: String?): String {
        val conn = open(server, url)
        val status = conn.responseCode
        if (status !in 200..299) throw IOException("HTTP $status")
        val type = conn.contentType ?: mime
        val name = fileName(conn.getHeaderField("Content-Disposition") ?: disposition) ?: URLUtil.guessFileName(url, null, type)
        conn.inputStream.use { input -> save(context, name, type) { input.copyTo(it) } }
        return name
    }

    /** Saves a file the page built itself, which arrives as base64. */
    fun fromPage(context: Context, name: String, mime: String?, base64: String) =
        save(context, name, mime) { it.write(Base64.decode(base64, Base64.DEFAULT)) }

    private fun save(context: Context, name: String, mime: String?, write: (OutputStream) -> Unit) {
        val resolver = context.contentResolver
        val values = ContentValues().apply {
            put(MediaStore.Downloads.DISPLAY_NAME, name)
            put(MediaStore.Downloads.MIME_TYPE, mime ?: "application/octet-stream")
            // Hidden from other apps until the last byte is in.
            put(MediaStore.Downloads.IS_PENDING, 1)
        }
        val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values)
            ?: throw IOException("Downloads refused the file")
        try {
            resolver.openOutputStream(uri)!!.use(write)
        } catch (e: IOException) {
            resolver.delete(uri, null, null)
            throw e
        }
        resolver.update(uri, ContentValues().apply { put(MediaStore.Downloads.IS_PENDING, 0) }, null, null)
    }

    /** The file name in a Content-Disposition header, preferring the UTF-8 form. */
    private fun fileName(disposition: String?): String? {
        if (disposition == null) return null
        Regex("filename\\*=UTF-8''([^;]+)", RegexOption.IGNORE_CASE).find(disposition)?.let {
            return Uri.decode(it.groupValues[1])
        }
        return Regex("filename=\"?([^\";]+)\"?", RegexOption.IGNORE_CASE).find(disposition)?.groupValues?.get(1)
    }
}
