package com.rcontrol.app.ui

import android.content.Context

/**
 * Every configurable value, in one place.
 *
 * Screens only expose actions; this store is edited from the Settings screen
 * (gear icon on Home) and read read-only by Home / Wake / Control.
 */
data class Settings(
    // connection
    val host: String = "",
    val port: String = "7945",
    val token: String = "",
    val pcName: String = "",
    // streaming
    val fps: Int = 5,
    val jpeg: Int = 70,
    val maxWidth: Int = 960,
    // wake
    val mac: String = "",
    val broadcast: String = "255.255.255.255",
    val wolPort: String = "9",
    val wakeHttpEnabled: Boolean = false,
    val wakeMethod: String = "POST",
    val wakeUrl: String = "",
    val wakeHeaders: String = "",
    val wakeBody: String = "",
) {
    val portInt: Int get() = port.toIntOrNull() ?: 7945
    val wolPortInt: Int get() = wolPort.toIntOrNull() ?: 9
    val hasWakeMethod: Boolean get() = mac.isNotBlank() || wakeHttpEnabled

    /** Human label for the PC, falling back to its address. */
    val label: String get() = pcName.ifBlank { host.ifBlank { "Your PC" } }
}

private const val FILE = "rcontrol_settings"

fun loadSettings(ctx: Context): Settings {
    val p = ctx.getSharedPreferences(FILE, Context.MODE_PRIVATE)
    if (p.all.isEmpty()) {
        // One-time migration from the old split prefs (v0.1 kept host/token in
        // "rcontrol_connect" and wake config in "rcontrol_wake").
        val c = ctx.getSharedPreferences("rcontrol_connect", Context.MODE_PRIVATE)
        val w = ctx.getSharedPreferences("rcontrol_wake", Context.MODE_PRIVATE)
        val migrated = Settings(
            host = c.getString("host", "") ?: "",
            port = c.getString("port", "7945") ?: "7945",
            token = c.getString("token", "") ?: "",
            mac = w.getString("mac", "") ?: "",
            broadcast = w.getString("broadcast", "255.255.255.255") ?: "255.255.255.255",
            wolPort = w.getString("wolPort", "9") ?: "9",
            wakeHttpEnabled = w.getBoolean("httpEnabled", false),
            wakeMethod = w.getString("method", "POST") ?: "POST",
            wakeUrl = w.getString("url", "") ?: "",
            wakeHeaders = w.getString("headersText", "") ?: "",
            wakeBody = w.getString("body", "") ?: "",
        )
        saveSettings(ctx, migrated)
        return migrated
    }
    return Settings(
        host = p.getString("host", "") ?: "",
        port = p.getString("port", "7945") ?: "7945",
        token = p.getString("token", "") ?: "",
        pcName = p.getString("pcName", "") ?: "",
        fps = p.getInt("fps", 5),
        jpeg = p.getInt("jpeg", 70),
        maxWidth = p.getInt("maxWidth", 960),
        mac = p.getString("mac", "") ?: "",
        broadcast = p.getString("broadcast", "255.255.255.255") ?: "255.255.255.255",
        wolPort = p.getString("wolPort", "9") ?: "9",
        wakeHttpEnabled = p.getBoolean("wakeHttpEnabled", false),
        wakeMethod = p.getString("wakeMethod", "POST") ?: "POST",
        wakeUrl = p.getString("wakeUrl", "") ?: "",
        wakeHeaders = p.getString("wakeHeaders", "") ?: "",
        wakeBody = p.getString("wakeBody", "") ?: "",
    )
}

fun saveSettings(ctx: Context, s: Settings) {
    ctx.getSharedPreferences(FILE, Context.MODE_PRIVATE).edit()
        .putString("host", s.host)
        .putString("port", s.port)
        .putString("token", s.token)
        .putString("pcName", s.pcName)
        .putInt("fps", s.fps)
        .putInt("jpeg", s.jpeg)
        .putInt("maxWidth", s.maxWidth)
        .putString("mac", s.mac)
        .putString("broadcast", s.broadcast)
        .putString("wolPort", s.wolPort)
        .putBoolean("wakeHttpEnabled", s.wakeHttpEnabled)
        .putString("wakeMethod", s.wakeMethod)
        .putString("wakeUrl", s.wakeUrl)
        .putString("wakeHeaders", s.wakeHeaders)
        .putString("wakeBody", s.wakeBody)
        .apply()
}
