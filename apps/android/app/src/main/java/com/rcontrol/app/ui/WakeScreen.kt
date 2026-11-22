@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.rcontrol.app.ui

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.rcontrol.app.net.WakeHttpConfig
import com.rcontrol.app.net.sendMagicPacket
import com.rcontrol.app.net.sendWakeHttp
import com.rcontrol.app.net.waitForServer
import com.rcontrol.app.ui.components.Panel
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

/**
 * Wake = one action. Everything it needs (MAC, broadcast, HTTP request) comes
 * from Settings; the screen only shows what is happening right now.
 */
@Composable
fun WakeScreen(
    session: SessionState,
    onOnline: () -> Unit,
    onSettings: () -> Unit,
    onBack: () -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val settings = remember { loadSettings(ctx) }

    var busy by remember { mutableStateOf(false) }
    var status by remember { mutableStateOf("") }
    var error by remember { mutableStateOf("") }
    var job by remember { mutableStateOf<Job?>(null) }

    fun start() {
        saveSettings(ctx, settings)
        session.host = settings.host.trim()
        session.port = settings.port
        session.pcName = settings.pcName
        busy = true
        error = ""
        status = "Sending wake signal…"
        job = scope.launch {
            try {
                if (settings.mac.isNotBlank()) {
                    sendMagicPacket(settings.mac, settings.broadcast, settings.wolPortInt)
                    status = "Magic packet sent."
                }
                if (settings.wakeHttpEnabled) {
                    val code = sendWakeHttp(
                        WakeHttpConfig(
                            method = settings.wakeMethod,
                            url = settings.wakeUrl,
                            headersText = settings.wakeHeaders,
                            body = settings.wakeBody
                        )
                    )
                    status = "Wake request sent (HTTP $code)."
                }
                status = "Waiting for the server to answer…"
                val up = waitForServer(settings.host.trim(), settings.portInt, onTick = { s ->
                    status = "Waiting for the server… (${s}s)"
                })
                if (up) {
                    status = "PC is online."
                    onOnline()
                } else {
                    error = "No answer after 5 min. Check that WoL is enabled in the BIOS/NIC."
                }
            } catch (e: Exception) {
                error = e.message.orEmpty()
            } finally {
                busy = false
            }
        }
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Wake", style = MaterialTheme.typography.titleLarge) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { pad ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            item {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 18.dp, bottom = 6.dp)
                ) {
                    Box(
                        Modifier
                            .size(84.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Box(
                            Modifier
                                .size(84.dp)
                                .border(
                                    2.dp,
                                    MaterialTheme.colorScheme.surfaceVariant,
                                    CircleShape
                                )
                        )
                        if (busy) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(84.dp),
                                color = MaterialTheme.colorScheme.primary,
                                strokeWidth = 2.dp
                            )
                        }
                        Icon(
                            Icons.Default.PowerSettingsNew,
                            contentDescription = null,
                            tint = if (error.isNotEmpty()) MaterialTheme.colorScheme.error
                            else MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(34.dp)
                        )
                    }
                    Text(
                        "Wake ${settings.label}",
                        style = MaterialTheme.typography.headlineSmall,
                        textAlign = TextAlign.Center,
                        modifier = Modifier.padding(top = 16.dp)
                    )
                    Text(
                        "${settings.host}:${settings.port}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(top = 2.dp)
                    )
                }
            }

            if (!settings.hasWakeMethod) {
                item {
                    Panel {
                        Text("No wake method set up", style = MaterialTheme.typography.titleMedium)
                        Text(
                            "Add the PC's MAC address (or a custom HTTP wake request) in Settings.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 6.dp)
                        )
                        Button(
                            onClick = onSettings,
                            shape = RoundedCornerShape(16.dp),
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(50.dp)
                                .padding(top = 12.dp)
                        ) { Text("Open settings") }
                    }
                }
            } else {
                item {
                    Button(
                        onClick = { start() },
                        enabled = !busy,
                        shape = RoundedCornerShape(18.dp),
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(60.dp)
                    ) {
                        if (busy) {
                            CircularProgressIndicator(
                                Modifier.size(22.dp),
                                color = MaterialTheme.colorScheme.onPrimary,
                                strokeWidth = 2.dp
                            )
                        } else {
                            Icon(Icons.Default.PowerSettingsNew, contentDescription = null)
                            Text("Send wake signal", modifier = Modifier.padding(start = 10.dp))
                        }
                    }
                }
            }

            if (settings.hasWakeMethod) {
                item {
                    Panel {
                        Column {
                            Text(
                                "METHOD",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                            if (settings.mac.isNotBlank()) {
                                Text(
                                    "Magic packet → ${settings.mac}",
                                    style = MaterialTheme.typography.bodyMedium,
                                    modifier = Modifier.padding(top = 8.dp)
                                )
                                Text(
                                    "broadcast ${settings.broadcast}:${settings.wolPort}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                            if (settings.wakeHttpEnabled) {
                                Text(
                                    "${settings.wakeMethod} ${settings.wakeUrl}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.padding(top = 8.dp)
                                )
                            }
                            Text(
                                "Then polls /health every 2.5 s for 5 min.",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(top = 8.dp)
                            )
                        }
                    }
                }
            }

            if (busy || status.isNotBlank() || error.isNotBlank()) {
                item {
                    Panel {
                        if (busy) {
                            LinearProgressIndicator(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(bottom = 12.dp),
                                color = MaterialTheme.colorScheme.primary
                            )
                        }
                        if (status.isNotBlank()) {
                            Text(status, style = MaterialTheme.typography.bodyMedium)
                        }
                        if (error.isNotBlank()) {
                            Text(
                                error,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.error,
                                modifier = Modifier.padding(top = 4.dp)
                            )
                        }
                        if (busy) {
                            Row(
                                horizontalArrangement = Arrangement.spacedBy(10.dp),
                                modifier = Modifier.padding(top = 12.dp)
                            ) {
                                OutlinedButton(
                                    onClick = {
                                        job?.cancel()
                                        busy = false
                                        status = "Cancelled."
                                    },
                                    shape = RoundedCornerShape(14.dp),
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .height(48.dp)
                                ) { Text("Stop waiting") }
                            }
                        }
                    }
                }
            }
        }
    }
}
