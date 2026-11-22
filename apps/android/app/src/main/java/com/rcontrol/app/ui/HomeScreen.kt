@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.rcontrol.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.filled.Cable
import androidx.compose.material.icons.filled.Cast
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.rcontrol.app.R
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.rcontrol.app.net.ServerInfo
import com.rcontrol.app.net.discoverServers
import com.rcontrol.app.net.pingServer
import com.rcontrol.app.ui.components.Panel
import com.rcontrol.app.ui.components.PillTone
import com.rcontrol.app.ui.components.SectionTitle
import com.rcontrol.app.ui.components.StatusPill
import kotlinx.coroutines.launch

/**
 * Home = the only screen with actions on it: pick a PC, connect, wake it.
 * Nothing here is typed by hand — addresses, tokens and wake settings live
 * behind the gear icon.
 */
@Composable
fun HomeScreen(
    session: SessionState,
    onConnect: () -> Unit,
    onWake: () -> Unit,
    onSettings: () -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    var settings by remember { mutableStateOf(loadSettings(ctx)) }
    var servers by remember { mutableStateOf<List<ServerInfo>>(emptyList()) }
    var scanning by remember { mutableStateOf(false) }
    var online by remember { mutableStateOf<Boolean?>(null) }
    var msg by remember { mutableStateOf("") }

    // Cheap reachability check so the card can say online / offline.
    LaunchedEffect(settings.host, settings.port) {
        online = if (settings.host.isBlank()) null
        else pingServer(settings.host.trim(), settings.portInt)
    }

    fun connectTo(target: Settings) {
        val host = target.host.trim()
        if (host.isBlank()) {
            onSettings()
            return
        }
        saveSettings(ctx, target.copy(host = host))
        session.host = host
        session.port = target.port
        session.token = target.token.trim()
        session.pcName = target.pcName
        onConnect()
    }

    fun scan() {
        scanning = true
        msg = ""
        scope.launch {
            try {
                servers = discoverServers()
                if (servers.isEmpty()) msg = ctx.getString(R.string.home_scan_empty)
            } catch (e: Exception) {
                msg = e.message ?: ctx.getString(R.string.home_scan_failed)
            } finally {
                scanning = false
            }
        }
    }

    Scaffold(containerColor = MaterialTheme.colorScheme.background) { pad ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            item {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp, bottom = 2.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(stringResource(R.string.app_name), style = MaterialTheme.typography.displaySmall)
                        Text(
                            stringResource(R.string.home_tagline),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = onSettings) {
                        Icon(
                            Icons.Default.Settings,
                            contentDescription = stringResource(R.string.action_settings),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            // ---- the target ----
            item {
                Panel {
                    SectionTitle(stringResource(R.string.section_your_pc), trailing = {
                        when (online) {
                            true -> StatusPill(stringResource(R.string.status_online), PillTone.Good)
                            false -> StatusPill(stringResource(R.string.status_offline), PillTone.Warn)
                            null -> if (settings.host.isNotBlank())
                                StatusPill(stringResource(R.string.status_saved), PillTone.Idle)
                        }
                    })

                    if (settings.host.isBlank()) {
                        Text(
                            stringResource(R.string.home_no_pc),
                            style = MaterialTheme.typography.titleLarge,
                            modifier = Modifier.padding(top = 10.dp)
                        )
                        Text(
                            stringResource(R.string.home_no_pc_hint),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 4.dp)
                        )
                    } else {
                        Text(
                            settings.label,
                            style = MaterialTheme.typography.titleLarge,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.padding(top = 10.dp)
                        )
                        Text(
                            "${settings.host}:${settings.port}",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 2.dp)
                        )
                    }

                    Row(
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        modifier = Modifier.padding(top = 16.dp)
                    ) {
                        Button(
                            onClick = { if (settings.host.isBlank()) onSettings() else connectTo(settings) },
                            shape = RoundedCornerShape(16.dp),
                            modifier = Modifier
                                .weight(1.5f)
                                .height(54.dp)
                        ) {
                            Icon(Icons.Default.Cast, contentDescription = null, modifier = Modifier.size(19.dp))
                            Text(
                                stringResource(if (settings.host.isBlank()) R.string.action_set_up_pc else R.string.action_connect),
                                modifier = Modifier.padding(start = 8.dp)
                            )
                        }
                        OutlinedButton(
                            onClick = onWake,
                            enabled = settings.host.isNotBlank(),
                            shape = RoundedCornerShape(16.dp),
                            modifier = Modifier
                                .weight(1f)
                                .height(54.dp)
                        ) {
                            Icon(
                                Icons.Default.PowerSettingsNew,
                                contentDescription = null,
                                modifier = Modifier.size(18.dp)
                            )
                            Text(stringResource(R.string.action_wake), modifier = Modifier.padding(start = 8.dp))
                        }
                    }
                }
            }

            // ---- discovered PCs ----
            item {
                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    SectionTitle(stringResource(R.string.section_network), trailing = {
                        if (scanning) {
                            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        } else {
                            TextButton(onClick = { scan() }) {
                                Icon(
                                    Icons.Default.Refresh,
                                    contentDescription = null,
                                    modifier = Modifier.size(17.dp)
                                )
                                Text(stringResource(R.string.action_scan), modifier = Modifier.padding(start = 6.dp))
                            }
                        }
                    })

                    if (servers.isEmpty()) {
                        Text(
                            stringResource(
                                if (scanning) R.string.home_scanning else R.string.home_scan_hint
                            ),
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }

                    servers.forEach { s ->
                        ServerRow(
                            info = s,
                            selected = s.host == settings.host && s.port.toString() == settings.port,
                            onConnect = {
                                connectTo(
                                    settings.copy(
                                        host = s.host,
                                        port = s.port.toString(),
                                        pcName = s.name,
                                        token = if (s.auth) settings.token else ""
                                    )
                                )
                            }
                        )
                    }

                    if (msg.isNotBlank()) {
                        Text(
                            msg,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 2.dp)
                        )
                    }
                }
            }

            item { Spacer(Modifier.height(4.dp)) }
        }
    }
}

@Composable
private fun ServerRow(info: ServerInfo, selected: Boolean, onConnect: () -> Unit) {
    Surface(
        onClick = onConnect,
        shape = RoundedCornerShape(16.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surface,
        border = BorderStroke(
            1.dp,
            if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outline
        ),
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                Modifier
                    .size(36.dp)
                    .background(MaterialTheme.colorScheme.surfaceVariant, RoundedCornerShape(12.dp)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    Icons.Default.Cable,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(19.dp)
                )
            }
            Column(Modifier.weight(1f)) {
                Text(
                    info.name,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    "${info.host}:${info.port}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            if (info.auth) StatusPill(stringResource(R.string.pill_auth), PillTone.Good)
            else StatusPill(stringResource(R.string.pill_open), PillTone.Warn)
            Icon(
                Icons.AutoMirrored.Filled.ArrowForward,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(18.dp)
            )
        }
    }
}
