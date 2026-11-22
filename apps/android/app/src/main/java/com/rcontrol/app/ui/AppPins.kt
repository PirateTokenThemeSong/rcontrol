package com.rcontrol.app.ui

import android.content.Context
import com.rcontrol.app.net.RemoteApp

private const val FILE = "rcontrol_pins"
private const val SEP = "|||"

/** Pinned apps (id + label), stored on-device. */
fun loadPins(ctx: Context): List<RemoteApp> =
    ctx.getSharedPreferences(FILE, Context.MODE_PRIVATE)
        .getStringSet("pins", emptySet()).orEmpty()
        .mapNotNull { raw ->
            val i = raw.indexOf(SEP)
            if (i < 0) null else RemoteApp(raw.substring(0, i), raw.substring(i + SEP.length))
        }
        .sortedBy { it.label.lowercase() }

fun isPinned(ctx: Context, id: String): Boolean =
    loadPins(ctx).any { it.id == id }

/** Toggle pin state; returns the updated list. */
fun togglePin(ctx: Context, app: RemoteApp): List<RemoteApp> {
    val prefs = ctx.getSharedPreferences(FILE, Context.MODE_PRIVATE)
    val set = prefs.getStringSet("pins", emptySet()).orEmpty().toMutableSet()
    val key = app.id + SEP + app.label
    if (set.any { it.startsWith(app.id + SEP) }) {
        set.removeAll { it.startsWith(app.id + SEP) }
    } else {
        set.add(key)
    }
    prefs.edit().putStringSet("pins", set).apply()
    return loadPins(ctx)
}
