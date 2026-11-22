@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.rcontrol.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.Image
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PushPin
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.WebAsset
import androidx.compose.material.icons.outlined.PushPin
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.produceState
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.rcontrol.app.net.RemoteApp
import com.rcontrol.app.net.RemoteWindow
import com.rcontrol.app.net.fetchApps
import com.rcontrol.app.net.fetchIcon
import com.rcontrol.app.net.fetchWindows
import com.rcontrol.app.net.focusWindow
import com.rcontrol.app.net.launchApp
import com.rcontrol.app.net.searchApps
import com.rcontrol.app.ui.components.PillTone
import com.rcontrol.app.ui.components.StatusPill
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private enum class Pane { Windows, Apps }

/** Cache of real logos fetched from the PC (null = this id has no icon). */
private val iconCache = java.util.Collections.synchronizedMap(HashMap<String, android.graphics.Bitmap?>())

/**
 * One compact block for the two "pick something on the PC" actions:
 * switch to an open window, or launch an app. A segmented control swaps the
 * list, so the screen never grows two competing stacks of rows.
 */
@Composable
fun SwitchLaunchPanel(
    host: String,
    port: Int,
    token: String,
    onAck: (String) -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    var pane by remember { mutableStateOf(Pane.Windows) }
    var expanded by remember { mutableStateOf(false) }

    var windows by remember { mutableStateOf<List<RemoteWindow>>(emptyList()) }
    var loadingWindows by remember { mutableStateOf(false) }

    var apps by remember { mutableStateOf<List<RemoteApp>>(emptyList()) }
    var results by remember { mutableStateOf<List<RemoteApp>>(emptyList()) }
    var query by remember { mutableStateOf("") }
    var searching by remember { mutableStateOf(false) }
    var pinned by remember { mutableStateOf(loadPins(ctx)) }

    LaunchedEffect(host, port, token) {
        windows = try { fetchWindows(host, port, token) } catch (_: Exception) { emptyList() }
        apps = try { fetchApps(host, port, token) } catch (_: Exception) { emptyList() }
    }

    // Search as you type (no Go button): debounce, then query the Start Menu.
    LaunchedEffect(query, pane) {
        if (pane != Pane.Apps || query.isBlank()) {
            results = emptyList()
            searching = false
            return@LaunchedEffect
        }
        delay(300)
        searching = true
        results = try { searchApps(host, port, token, query) } catch (_: Exception) { emptyList() }
        searching = false
    }

    fun refreshWindows() {
        scope.launch {
            loadingWindows = true
            windows = try { fetchWindows(host, port, token) } catch (_: Exception) { emptyList() }
            loadingWindows = false
        }
    }

    fun focus(w: RemoteWindow) {
        scope.launch {
            onAck(focusWindow(host, port, token, w.id, w.title))
            windows = try { fetchWindows(host, port, token) } catch (_: Exception) { windows }
        }
    }

    fun launch(app: RemoteApp) {
        scope.launch { onAck(launchApp(host, port, token, app.id, app.label)) }
    }

    // Pinned apps first, then the allowlist; search results replace both.
    val appRows = if (query.isBlank()) {
        pinned + apps.filter { a -> pinned.none { it.id == a.id } }
    } else {
        results
    }
    val rows: List<Any> = if (pane == Pane.Windows) windows else appRows
    val visible = if (expanded) rows else rows.take(6)

    Surface(
        shape = RoundedCornerShape(22.dp),
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column {
            Row(
                Modifier.padding(start = 12.dp, end = 6.dp, top = 10.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Segmented(pane) {
                    pane = it
                    expanded = false
                }
                Spacer(Modifier.weight(1f))
                if (pane == Pane.Windows) {
                    IconButton(onClick = { refreshWindows() }, modifier = Modifier.size(38.dp)) {
                        if (loadingWindows) {
                            CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                        } else {
                            Icon(
                                Icons.Default.Refresh,
                                contentDescription = "Refresh windows",
                                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.size(18.dp)
                            )
                        }
                    }
                }
            }

            if (pane == Pane.Apps) {
                OutlinedTextField(
                    value = query,
                    onValueChange = { query = it },
                    placeholder = { Text("Search apps on the PC") },
                    leadingIcon = {
                        Icon(Icons.Default.Search, null, Modifier.size(18.dp))
                    },
                    trailingIcon = {
                        if (searching) {
                            CircularProgressIndicator(Modifier.size(15.dp), strokeWidth = 2.dp)
                        } else if (query.isNotEmpty()) {
                            TextButton(onClick = { query = "" }) { Text("Clear") }
                        }
                    },
                    singleLine = true,
                    shape = RoundedCornerShape(12.dp),
                    colors = panelFieldColors(),
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp)
                        .padding(bottom = 10.dp)
                        .height(52.dp)
                )
            }

            when {
                rows.isEmpty() && pane == Pane.Windows -> EmptyRow("No windows found — Windows PC only for now.")
                rows.isEmpty() && pane == Pane.Apps ->
                    EmptyRow(if (query.isBlank()) "No apps configured." else "No apps match “$query”.")
            }

            visible.forEachIndexed { i, item ->
                if (i > 0) {
                    HorizontalDivider(
                        color = MaterialTheme.colorScheme.outline.copy(alpha = 0.55f),
                        modifier = Modifier.padding(start = 52.dp)
                    )
                }
                when (item) {
                    is RemoteWindow -> WindowPick(host, port, token, item) { focus(item) }
                    is RemoteApp -> AppPick(
                        host = host,
                        port = port,
                        token = token,
                        app = item,
                        pinned = pinned.any { it.id == item.id },
                        onClick = { launch(item) },
                        onTogglePin = { pinned = togglePin(ctx, item) }
                    )
                }
            }

            if (rows.size > 6) {
                HorizontalDivider(color = MaterialTheme.colorScheme.outline.copy(alpha = 0.55f))
                TextButton(
                    onClick = { expanded = !expanded },
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(42.dp)
                ) {
                    Text(if (expanded) "Show less" else "Show all ${rows.size}")
                }
            }
        }
    }
}

@Composable
private fun Segmented(pane: Pane, onChange: (Pane) -> Unit) {
    Row(
        Modifier
            .background(MaterialTheme.colorScheme.surfaceVariant, RoundedCornerShape(50))
            .padding(3.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp)
    ) {
        SegItem("Windows", pane == Pane.Windows) { onChange(Pane.Windows) }
        SegItem("Apps", pane == Pane.Apps) { onChange(Pane.Apps) }
    }
}

@Composable
private fun SegItem(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(50),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else Color.Transparent,
        contentColor = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
        else MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.height(30.dp)
    ) {
        Box(Modifier.padding(horizontal = 14.dp), contentAlignment = Alignment.Center) {
            Text(label, style = MaterialTheme.typography.labelMedium)
        }
    }
}

@Composable
private fun WindowPick(host: String, port: Int, token: String, w: RemoteWindow, onClick: () -> Unit) {
    PickRow(
        onClick = onClick,
        leading = {
            RemoteIcon("window", w.id, host, port, token) {
                Box(
                    Modifier
                        .size(28.dp)
                        .background(MaterialTheme.colorScheme.surfaceVariant, RoundedCornerShape(8.dp)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        Icons.Default.WebAsset,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.secondary,
                        modifier = Modifier.size(15.dp)
                    )
                }
            }
        },
        title = w.title,
        trailing = {
            when {
                w.active -> StatusPill("active", PillTone.Good)
                w.minimized -> StatusPill("min", PillTone.Warn)
            }
        }
    )
}

@Composable
private fun AppPick(
    host: String,
    port: Int,
    token: String,
    app: RemoteApp,
    pinned: Boolean,
    onClick: () -> Unit,
    onTogglePin: () -> Unit,
) {
    PickRow(
        onClick = onClick,
        leading = {
            RemoteIcon("app", app.id, host, port, token) {
                Box(
                    Modifier
                        .size(28.dp)
                        .background(MaterialTheme.colorScheme.surfaceVariant, CircleShape),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        app.label.take(1).uppercase(),
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.primary,
                        fontWeight = FontWeight.Bold
                    )
                }
            }
        },
        title = app.label,
        trailing = {
            IconButton(onClick = onTogglePin, modifier = Modifier.size(30.dp)) {
                Icon(
                    if (pinned) Icons.Default.PushPin else Icons.Outlined.PushPin,
                    contentDescription = if (pinned) "Unpin" else "Pin",
                    tint = if (pinned) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(16.dp)
                )
            }
        }
    )
}

@Composable
private fun PickRow(
    onClick: () -> Unit,
    leading: @Composable () -> Unit,
    title: String,
    trailing: @Composable () -> Unit = {},
) {
    Row(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        leading()
        Text(
            title,
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f)
        )
        trailing()
    }
}

@Composable
private fun EmptyRow(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 14.dp)
    )
}

@Composable
private fun panelFieldColors() = OutlinedTextFieldDefaults.colors(
    focusedBorderColor = MaterialTheme.colorScheme.primary,
    unfocusedBorderColor = MaterialTheme.colorScheme.outline,
    focusedTextColor = MaterialTheme.colorScheme.onSurface,
    unfocusedTextColor = MaterialTheme.colorScheme.onSurface,
    cursorColor = MaterialTheme.colorScheme.primary,
    focusedPlaceholderColor = MaterialTheme.colorScheme.onSurfaceVariant,
    unfocusedPlaceholderColor = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
    focusedLeadingIconColor = MaterialTheme.colorScheme.primary,
    unfocusedLeadingIconColor = MaterialTheme.colorScheme.onSurfaceVariant
)

/** Loads the real icon from the PC; falls back to the placeholder until it arrives. */
@Composable
private fun RemoteIcon(
    kind: String,
    id: String,
    host: String,
    port: Int,
    token: String,
    fallback: @Composable () -> Unit,
) {
    val key = "$kind:$id"
    val bmp by produceState<android.graphics.Bitmap?>(initialValue = iconCache[key], kind, id) {
        if (iconCache.containsKey(key)) {
            value = iconCache[key]
            return@produceState
        }
        val fetched = fetchIcon(host, port, token, kind, id)
        iconCache[key] = fetched
        value = fetched
    }
    val current = bmp
    if (current != null) {
        Image(
            bitmap = current.asImageBitmap(),
            contentDescription = null,
            modifier = Modifier.size(26.dp),
            contentScale = ContentScale.Fit
        )
    } else {
        fallback()
    }
}
