package com.rcontrol.app.net

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.coroutines.Dispatchers
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.URLEncoder
import java.util.concurrent.TimeUnit

private val JSON_MEDIA = "application/json; charset=utf-8".toMediaType()

/** Fully user-configurable HTTP request fired to trigger a wake (e.g. router,
 * Home Assistant, or an always-on Pi that sends the magic packet). */
data class WakeHttpConfig(
    val enabled: Boolean = false,
    val method: String = "POST",
    val url: String = "",
    /** One "Name: value" header per line. */
    val headersText: String = "",
    val body: String = "",
)

fun parseHeaders(text: String): Map<String, String> =
    text.lineSequence()
        .map { it.trim() }
        .filter { it.isNotEmpty() && it.contains(":") }
        .associate {
            val i = it.indexOf(":")
            it.substring(0, i).trim() to it.substring(i + 1).trim()
        }

/** Send a Wake-on-LAN magic packet directly from the phone. Throws on bad MAC. */
suspend fun sendMagicPacket(
    mac: String,
    broadcastIp: String = "255.255.255.255",
    port: Int = 9,
) = withContext(Dispatchers.IO) {
    val macBytes = parseMac(mac)
    val packet = ByteArray(6 + 16 * 6)
    for (i in 0 until 6) packet[i] = 0xFF.toByte()
    for (i in 0 until 16) System.arraycopy(macBytes, 0, packet, 6 + i * 6, 6)
    DatagramSocket().use { s ->
        s.broadcast = true
        s.send(DatagramPacket(packet, packet.size, InetAddress.getByName(broadcastIp.ifBlank { "255.255.255.255" }), port))
    }
}

fun parseMac(mac: String): ByteArray {
    val clean = mac.trim().replace("-", ":")
    val parts = clean.split(":")
    require(parts.size == 6) { "MAC must have 6 octets (AA:BB:CC:DD:EE:FF), got: $mac" }
    return ByteArray(6) { i ->
        parts[i].toIntOrNull(16)?.toByte()
            ?: throw IllegalArgumentException("Bad MAC octet: ${parts[i]}")
    }
}

/** Fire the user-configured wake HTTP request. Returns the HTTP status code. */
suspend fun sendWakeHttp(cfg: WakeHttpConfig): Int = withContext(Dispatchers.IO) {
    require(cfg.url.isNotBlank()) { "Wake URL is empty" }
    val client = OkHttpClient.Builder().callTimeout(15, TimeUnit.SECONDS).build()
    val method = cfg.method.trim().uppercase().ifEmpty { "POST" }
    val body: RequestBody? =
        if (cfg.body.isEmpty() && (method == "GET" || method == "HEAD" || method == "DELETE")) null
        else cfg.body.toRequestBody(JSON_MEDIA)
    val builder = Request.Builder().url(cfg.url.trim()).method(method, body)
    parseHeaders(cfg.headersText).forEach { (k, v) -> builder.header(k, v) }
    client.newCall(builder.build()).execute().use { resp -> resp.code }
}

/** Poll GET /health until the PC's server answers or the timeout expires. */
suspend fun waitForServer(
    host: String,
    port: Int,
    timeoutMs: Long = 5 * 60 * 1000,
    intervalMs: Long = 2500,
    onTick: (elapsedSecs: Long) -> Unit = {},
): Boolean = withContext(Dispatchers.IO) {
    val client = OkHttpClient.Builder()
        .connectTimeout(3, TimeUnit.SECONDS)
        .readTimeout(3, TimeUnit.SECONDS)
        .build()
    val start = System.currentTimeMillis()
    while (System.currentTimeMillis() - start < timeoutMs) {
        try {
            val req = Request.Builder().url("http://$host:$port/health").get().build()
            client.newCall(req).execute().use { resp ->
                if (resp.isSuccessful) return@withContext true
            }
        } catch (_: Exception) {
            // PC still down — keep polling.
        }
        onTick((System.currentTimeMillis() - start) / 1000)
        delay(intervalMs)
    }
    false
}

/** One quick /health probe for the Home card (no retries, short timeouts). */
suspend fun pingServer(host: String, port: Int, timeoutMs: Long = 1500): Boolean =
    withContext(Dispatchers.IO) {
        if (host.isBlank()) return@withContext false
        val client = OkHttpClient.Builder()
            .connectTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .readTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .callTimeout(timeoutMs, TimeUnit.MILLISECONDS)
            .build()
        try {
            val req = Request.Builder().url("http://$host:$port/health").get().build()
            client.newCall(req).execute().use { it.isSuccessful }
        } catch (_: Exception) {
            false
        }
    }

// ---- lock screen ----

/** Outcome of a remote unlock request. */
sealed interface UnlockResult {
    data object Ok : UnlockResult
    data object Timeout : UnlockResult
    data class Error(val message: String) : UnlockResult
}

/** POST /unlock — ask the helper to type the secret into the lock screen. */
suspend fun unlockPc(host: String, port: Int, token: String, secret: String): UnlockResult =
    withContext(Dispatchers.IO) {
        // Keep the UI honest: 10 s is enough for the helper to reveal the form,
        // type and press Enter.
        val client = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
        val body = JSONObject().put("secret", secret).toString().toRequestBody(JSON_MEDIA)
        val req = Request.Builder().url("http://$host:$port/unlock?token=$token").post(body).build()
        try {
            client.newCall(req).execute().use { resp ->
                if (resp.isSuccessful) {
                    UnlockResult.Ok
                } else {
                    val text = resp.body?.string().orEmpty()
                    val why = try {
                        JSONObject(text).optString("error", "HTTP ${resp.code}")
                    } catch (_: Exception) {
                        "HTTP ${resp.code}"
                    }
                    UnlockResult.Error(why)
                }
            }
        } catch (_: java.io.IOException) {
            UnlockResult.Timeout
        }
    }

/** GET /icon — real icon for an app id or window id, or null. */
suspend fun fetchIcon(
    host: String,
    port: Int,
    token: String,
    kind: String,
    id: String,
): Bitmap? = withContext(Dispatchers.IO) {
    val client = OkHttpClient.Builder().callTimeout(8, TimeUnit.SECONDS).build()
    val url = "http://$host:$port/icon?token=$token&kind=$kind&id=" + URLEncoder.encode(id, "UTF-8")
    try {
        client.newCall(Request.Builder().url(url).get().build()).execute().use { resp ->
            if (!resp.isSuccessful) return@withContext null
            val bytes = resp.body?.bytes() ?: return@withContext null
            BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
        }
    } catch (_: Exception) {
        null
    }
}

// ---- open windows ----

data class RemoteWindow(
    val id: String,
    val title: String,
    val active: Boolean,
    val minimized: Boolean,
)

/** GET /windows — visible top-level windows, front-most first. */
suspend fun fetchWindows(host: String, port: Int, token: String): List<RemoteWindow> =
    withContext(Dispatchers.IO) {
        val client = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
        val req = Request.Builder().url("http://$host:$port/windows?token=$token").get().build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) throw IllegalArgumentException("windows: HTTP ${resp.code} $text")
            val arr = JSONObject(text).optJSONArray("windows") ?: return@withContext emptyList()
            List(arr.length()) { i ->
                val o = arr.getJSONObject(i)
                RemoteWindow(
                    id = o.getString("id"),
                    title = o.optString("title", "(untitled)"),
                    active = o.optBoolean("active", false),
                    minimized = o.optBoolean("minimized", false),
                )
            }
        }
    }

/** POST /focus — brings one window to the front. Returns null on success, else an error. */
suspend fun focusWindow(
    host: String,
    port: Int,
    token: String,
    id: String,
): String? = withContext(Dispatchers.IO) {
    val client = OkHttpClient.Builder().callTimeout(15, TimeUnit.SECONDS).build()
    val body = JSONObject().put("id", id).toString().toRequestBody(JSON_MEDIA)
    val req = Request.Builder().url("http://$host:$port/focus?token=$token").post(body).build()
    client.newCall(req).execute().use { resp ->
        if (resp.isSuccessful) null else "HTTP ${resp.code} " + resp.body?.string().orEmpty()
    }
}

// ---- remote app launching ----

data class RemoteApp(val id: String, val label: String)

/** GET /apps — allowlisted apps (ids + labels only). */
suspend fun fetchApps(host: String, port: Int, token: String): List<RemoteApp> =
    withContext(Dispatchers.IO) {
        val client = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
        val req = Request.Builder().url("http://$host:$port/apps?token=$token").get().build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) throw IllegalArgumentException("apps: HTTP ${resp.code} $text")
            val arr = JSONObject(text).optJSONArray("apps") ?: return@withContext emptyList()
            List(arr.length()) { i ->
                val o = arr.getJSONObject(i)
                RemoteApp(o.getString("id"), o.optString("label", o.getString("id")))
            }
        }
    }

/** POST /launch — null on success, else an error. */
suspend fun launchApp(
    host: String,
    port: Int,
    token: String,
    id: String,
): String? =
    withContext(Dispatchers.IO) {
        val client = OkHttpClient.Builder().callTimeout(15, TimeUnit.SECONDS).build()
        val body = JSONObject().put("app", id).toString().toRequestBody(JSON_MEDIA)
        val req = Request.Builder().url("http://$host:$port/launch?token=$token").post(body).build()
        client.newCall(req).execute().use { resp ->
            if (resp.isSuccessful) null else "HTTP ${resp.code} " + resp.body?.string().orEmpty()
        }
    }

/** GET /search — Start Menu apps matching q (ids + labels only). */
suspend fun searchApps(host: String, port: Int, token: String, q: String): List<RemoteApp> =
    withContext(Dispatchers.IO) {
        if (q.isBlank()) return@withContext emptyList()
        val client = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
        val enc = URLEncoder.encode(q, "UTF-8")
        val req = Request.Builder().url("http://$host:$port/search?token=$token&q=$enc").get().build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) throw IllegalArgumentException("search: HTTP ${resp.code} $text")
            val arr = JSONObject(text).optJSONArray("apps") ?: return@withContext emptyList()
            List(arr.length()) { i ->
                val o = arr.getJSONObject(i)
                RemoteApp(o.getString("id"), o.optString("label", o.getString("id")))
            }
        }
    }
