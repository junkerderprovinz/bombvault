package bombvault.halleluja.design

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.os.Build
import android.os.Handler
import android.os.Looper
import java.net.Inet4Address
import java.net.InetAddress

/** A BombVault announcing itself on the local network. */
data class Found(val name: String, val url: String, val version: String)

/**
 * Discovery listens for the mDNS announcement BombVault makes of its web
 * interface and reports every instance it finds, on the main thread.
 */
class Discovery(context: Context, private val onChange: (List<Found>) -> Unit) {
    private val nsd = context.getSystemService(NsdManager::class.java)
    private val main = Handler(Looper.getMainLooper())
    private val listeners = mutableListOf<NsdManager.DiscoveryListener>()
    private val found = LinkedHashMap<String, Found>()

    // Before Android 14 NsdManager resolves one service at a time and fails
    // the rest, so they wait their turn here.
    private val queue = ArrayDeque<Pair<NsdServiceInfo, String>>()
    private var resolving = false

    fun start() {
        if (listeners.isNotEmpty()) return
        for ((type, scheme) in TYPES) {
            val listener = Listener(scheme)
            listeners += listener
            nsd.discoverServices(type, NsdManager.PROTOCOL_DNS_SD, listener)
        }
    }

    fun stop() {
        for (l in listeners) runCatching { nsd.stopServiceDiscovery(l) }
        listeners.clear()
        main.post {
            queue.clear()
            found.clear()
        }
    }

    private inner class Listener(private val scheme: String) : NsdManager.DiscoveryListener {
        override fun onServiceFound(info: NsdServiceInfo) {
            // The service type is shared with every other web interface; the
            // instance name is BombVault's own.
            if (!info.serviceName.startsWith("BombVault")) return
            main.post {
                queue.addLast(info to scheme)
                resolveNext()
            }
        }

        override fun onServiceLost(info: NsdServiceInfo) {
            main.post { if (found.remove(info.serviceName) != null) onChange(found.values.toList()) }
        }

        override fun onDiscoveryStarted(serviceType: String) {}
        override fun onDiscoveryStopped(serviceType: String) {}
        override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {}
        override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {}
    }

    private fun resolveNext() {
        if (resolving) return
        val (info, scheme) = queue.removeFirstOrNull() ?: return
        resolving = true
        @Suppress("DEPRECATION")
        nsd.resolveService(info, object : NsdManager.ResolveListener {
            override fun onServiceResolved(resolved: NsdServiceInfo) {
                main.post {
                    add(resolved, scheme)
                    resolving = false
                    resolveNext()
                }
            }

            override fun onResolveFailed(failed: NsdServiceInfo, errorCode: Int) {
                main.post {
                    resolving = false
                    resolveNext()
                }
            }
        })
    }

    private fun add(info: NsdServiceInfo, scheme: String) {
        val host = address(info) ?: return
        val path = info.attributes["path"]?.decodeToString() ?: "/"
        val version = info.attributes["version"]?.decodeToString() ?: ""
        found[info.serviceName] = Found(info.serviceName, "$scheme://$host:${info.port}$path", version)
        onChange(found.values.toList())
    }

    /** The IPv4 address to reach [info] by; a WebView cannot resolve .local names. */
    private fun address(info: NsdServiceInfo): String? {
        val all: List<InetAddress> = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            info.hostAddresses
        } else {
            @Suppress("DEPRECATION")
            listOfNotNull(info.host)
        }
        return all.firstOrNull { it is Inet4Address }?.hostAddress
    }

    companion object {
        private val TYPES = listOf("_https._tcp" to "https", "_http._tcp" to "http")
    }
}
