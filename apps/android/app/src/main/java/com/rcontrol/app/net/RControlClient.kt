package com.rcontrol.app.net

import android.graphics.BitmapFactory
import androidx.compose.runtime.mutableStateOf
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import org.json.JSONObject
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.util.concurrent.TimeUnit

private val JSON_MEDIA = "application/json; charset=utf-8".toMediaType()

/** Discovered PC on LAN. */
data class ServerInfo(
    val name: String,
    val id: String,
    val host: String,
    val port: Int,
    val auth: Boolean,
)

/** Scan LAN via UDP broadcast. Suspend-friendly: runs blocking IO on Dispatchers.IO. */
suspend fun discoverServers(
    udpPort: Int = 7946,
    timeoutMs: Int = 2500,
): List<ServerInfo> = withContext(Dispatchers.IO) {
    val found = mutableMapOf<String, ServerInfo>()
    val socket = DatagramSocket().apply {
        broadcast = true
        soTimeout = timeoutMs
    }
    try {
        val out = "RC_DISCOVER v1".toByteArray()
        val pkt = DatagramPacket(out, out.size, InetAddress.getByName("255.255.255.255"), udpPort)
        socket.send(pkt)
        val buf = ByteArray(1024)
        val deadline = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < deadline) {
            try {
                val resp = DatagramPacket(buf, buf.size)
                socket.receive(resp)
                val json = JSONObject(String(resp.data, 0, resp.length))
                val host = resp.address.hostAddress ?: continue
                val info = ServerInfo(
                    name = json.optString("name", host),
                    id = json.optString("id", host),
                    host = json.optString("addr", host).ifEmpty { host },
                    port = json.optInt("port", 7945),
                    auth = json.optBoolean("auth", true),
                )
                found[info.id] = info
            } catch (_: java.net.SocketTimeoutException) {
                break
            }
        }
    } catch (_: Exception) {
    } finally {
        socket.close()
    }
    found.values.toList()
}

/** Pair with PIN -> token (POST /pair). Returns token or throws. */
suspend fun pairWithPin(host: String, port: Int, pin: String): String =
    withContext(Dispatchers.IO) {
        val client = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
        val body = JSONObject().put("pin", pin).toString().toRequestBody(JSON_MEDIA)
        val req = Request.Builder().url("http://$host:$port/pair").post(body).build()
        client.newCall(req).execute().use { resp ->
            val text = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) throw IllegalArgumentException("Pair failed (${resp.code}): $text")
            JSONObject(text).getString("token")
        }
    }

enum class PowerAction(val wire: String) {
    Shutdown("shutdown"), Reboot("reboot"), Sleep("sleep"), Lock("lock"), Display("display")
}

/**
 * WebSocket client: receives JPEG binary frames, sends JSON commands.
 * Frame bytes are exposed as decoded [android.graphics.Bitmap] via [frame].
 */
class RControlClient(
    private val scope: CoroutineScope,
    private val host: String,
    private val port: Int,
    private val token: String,
    private val fps: Int = 5,
    private val jpeg: Int = 70,
    private val maxWidth: Int = 960,
) {
    val frame = mutableStateOf<android.graphics.Bitmap?>(null)
    val status = mutableStateOf("connecting…")
    val lastAck = mutableStateOf("")
    val desktopW = mutableStateOf(1920)
    val desktopH = mutableStateOf(1080)
    val locked = mutableStateOf(false)

    private val client = OkHttpClient.Builder()
        .pingInterval(30, TimeUnit.SECONDS)
        .build()
    private var ws: WebSocket? = null

    fun connect() {
        val url = "ws://$host:$port/ws?token=$token"
        val req = Request.Builder().url(url).build()
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                status.value = "connected"
                // Apply the quality chosen in Settings for this whole session.
                webSocket.send(JSONObject().put("type", "lockstate").toString())
                webSocket.send(
                    JSONObject()
                        .put("type", "quality")
                        .put("fps", fps)
                        .put("jpeg", jpeg)
                        .put("maxWidth", maxWidth)
                        .toString()
                )
            }

            override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
                // Binary = JPEG frame.
                val bmp = BitmapFactory.decodeByteArray(bytes.toByteArray(), 0, bytes.size)
                if (bmp != null) frame.value = bmp
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val j = JSONObject(text)
                    when (j.optString("type")) {
                        "power-ack" -> lastAck.value = "OK: ${j.optString("action")}"
                        "launch-ack" -> lastAck.value = "Launched ${j.optString("app")}"
                        "error" -> lastAck.value = "Error: ${j.optString("error")}"
                        "locked" -> locked.value = j.optBoolean("locked", false)
                        "hello" -> {
                            desktopW.value = j.optInt("width", 1920)
                            desktopH.value = j.optInt("height", 1080)
                            status.value = "connected (${desktopW.value}x${desktopH.value})"
                        }
                    }
                } catch (_: Exception) {
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                status.value = "error: ${t.message} (HTTP ${response?.code})"
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                status.value = "closed ($code $reason)"
            }
        })
    }

    fun setQuality(fps: Int = this.fps, jpeg: Int = this.jpeg, maxWidth: Int = this.maxWidth) {
        val msg = JSONObject()
            .put("type", "quality")
            .put("fps", fps)
            .put("jpeg", jpeg)
            .put("maxWidth", maxWidth)
        ws?.send(msg.toString())
    }

    fun power(action: PowerAction) {
        val msg = JSONObject().put("type", "power").put("action", action.wire)
        ws?.send(msg.toString())
        scope.launch { lastAck.value = "sending ${action.wire}…" }
    }

    fun disconnect() {
        try { ws?.close(1000, "bye") } catch (_: Exception) { }
        client.dispatcher.executorService.shutdown()
    }
}
