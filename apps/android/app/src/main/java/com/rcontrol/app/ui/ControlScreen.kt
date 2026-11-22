@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.rcontrol.app.ui

import android.graphics.Bitmap
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bedtime
import androidx.compose.material.icons.filled.Cast
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.LinkOff
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.TvOff
import androidx.compose.material.icons.filled.WebAsset
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.rcontrol.app.net.PowerAction
import com.rcontrol.app.net.RControlClient
import com.rcontrol.app.net.unlockPc
import com.rcontrol.app.ui.components.PillTone
import com.rcontrol.app.ui.components.Panel
import com.rcontrol.app.ui.components.StatusPill
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ControlScreen(session: SessionState, onDisconnect: () -> Unit, onSettings: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val settings = remember { loadSettings(ctx) }
    val client = remember(session.host, session.port, session.token) {
        RControlClient(
            scope = scope,
            host = session.host.trim(),
            port = session.port.toIntOrNull() ?: 7945,
            token = session.token.trim(),
            fps = settings.fps,
            jpeg = settings.jpeg,
            maxWidth = settings.maxWidth
        ).also {
            it.connect()
            session.client = it
        }
    }
    DisposableEffect(client) {
        onDispose { client.disconnect(); session.client = null }
    }

    var confirm by remember { mutableStateOf<PowerAction?>(null) }
    var secret by remember { mutableStateOf("") }
    var unlocking by remember { mutableStateOf(false) }
    // Drop the PIN from memory as soon as the lock screen is gone.
    LaunchedEffect(client.locked.value) {
        if (!client.locked.value) {
            secret = ""
            unlocking = false
        }
    }

    val statusText = client.status.value
    val needsPairing = statusText.contains("4401") || statusText.contains("unauthorized", ignoreCase = true)
    val linking = statusText.startsWith("connecting") || statusText == "idle"
    val aspect = (client.desktopW.value.toFloat() / client.desktopH.value.coerceAtLeast(1).toFloat())
        .coerceIn(0.3f, 3.5f)

    val pulse by rememberInfiniteTransition(label = "pulse").animateFloat(
        initialValue = 0.45f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(900), RepeatMode.Reverse),
        label = "pulseAlpha"
    )

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                ),
                title = {
                    Text(
                        session.label,
                        style = MaterialTheme.typography.titleLarge,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                },
                actions = {
                    if (client.locked.value) {
                        StatusPill("locked", PillTone.Warn, Modifier.padding(end = 4.dp))
                    }
                    IconButton(onClick = {
                        client.disconnect()
                        onDisconnect()
                    }) {
                        Icon(Icons.Default.LinkOff, contentDescription = "Disconnect")
                    }
                }
            )
        }
    ) { pad ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            // ---- live screen ----
            item {
                androidx.compose.material3.Surface(
                    shape = RoundedCornerShape(22.dp),
                    color = MaterialTheme.colorScheme.surface,
                    border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Box {
                        val bmp: Bitmap? = client.frame.value
                        if (bmp != null) {
                            Image(
                                bitmap = bmp.asImageBitmap(),
                                contentDescription = "PC screen",
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .aspectRatio(aspect),
                                contentScale = ContentScale.Fit
                            )
                        } else {
                            Box(
                                Modifier
                                    .fillMaxWidth()
                                    .aspectRatio(aspect)
                                    .background(MaterialTheme.colorScheme.surfaceVariant),
                                contentAlignment = Alignment.Center
                            ) {
                                Column(
                                    horizontalAlignment = Alignment.CenterHorizontally,
                                    verticalArrangement = Arrangement.spacedBy(10.dp)
                                ) {
                                    Icon(
                                        Icons.Default.Cast,
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                        modifier = Modifier.alpha(if (linking) pulse else 1f)
                                    )
                                    Text(
                                        if (linking) "Linking to your PC…"
                                        else "No frames — check token / port ${session.port}",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                }
                            }
                        }
                    }
                }
            }

            if (needsPairing) {
                item {
                    Panel {
                        Text("This PC needs pairing", style = MaterialTheme.typography.titleMedium)
                        Text(
                            "Enter the PIN shown on the PC console in Settings.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 4.dp)
                        )
                        OutlinedButton(
                            onClick = onSettings,
                            shape = RoundedCornerShape(14.dp),
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(46.dp)
                                .padding(top = 12.dp)
                        ) {
                            Icon(Icons.Default.Settings, null, Modifier.size(18.dp))
                            Text("Open settings", modifier = Modifier.padding(start = 8.dp))
                        }
                    }
                }
            }

            // ---- locked vs. action panel ----
            if (client.locked.value) {
                item {
                    Panel {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Icon(
                                Icons.Default.Lock,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.size(22.dp)
                            )
                            Column(Modifier.weight(1f)) {
                                Text("PC is locked", style = MaterialTheme.typography.titleMedium)
                                Text(
                                    "Type the Windows PIN or password to unlock it remotely.",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.padding(top = 2.dp)
                                )
                            }
                        }
                        OutlinedTextField(
                            value = secret,
                            onValueChange = { secret = it },
                            label = { Text("PIN or password") },
                            singleLine = true,
                            enabled = !unlocking,
                            visualTransformation = PasswordVisualTransformation(),
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 14.dp)
                        )
                        Button(
                            enabled = secret.isNotBlank() && !unlocking,
                            onClick = {
                                unlocking = true
                                scope.launch {
                                    client.lastAck.value = unlockPc(
                                        session.host.trim(),
                                        session.port.toIntOrNull() ?: 7945,
                                        session.token.trim(),
                                        secret
                                    )
                                    unlocking = false
                                }
                            },
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(48.dp)
                                .padding(top = 10.dp)
                        ) {
                            if (unlocking) {
                                CircularProgressIndicator(
                                    Modifier.size(18.dp),
                                    color = MaterialTheme.colorScheme.onPrimary,
                                    strokeWidth = 2.dp
                                )
                            } else {
                                Text("Unlock")
                            }
                        }
                    }
                }
            } else {
                item {
                    SwitchLaunchPanel(
                        host = session.host.trim(),
                        port = session.port.toIntOrNull() ?: 7945,
                        token = session.token.trim(),
                        onAck = { client.lastAck.value = it }
                    )
                }
            }

            // ---- power: one control, every action in a menu ----
            item {
                PowerMenu { action ->
                    when (action) {
                        PowerAction.Shutdown, PowerAction.Reboot -> confirm = action
                        else -> client.power(action)
                    }
                }
            }

            if (client.lastAck.value.isNotEmpty()) {
                item {
                    Surface(
                        shape = RoundedCornerShape(14.dp),
                        color = MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Text(
                            client.lastAck.value,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)
                        )
                    }
                }
            }

        }
    }

    confirm?.let { action ->
        AlertDialog(
            onDismissRequest = { confirm = null },
            containerColor = MaterialTheme.colorScheme.surfaceVariant,
            title = { Text("Confirm ${action.wire}?") },
            text = { Text("Your PC will ${action.wire} in ~5 seconds. Save your work first.") },
            confirmButton = {
                TextButton(
                    onClick = {
                        client.power(action)
                        confirm = null
                    },
                    colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
                ) { Text("Yes, ${action.wire}") }
            },
            dismissButton = {
                TextButton(onClick = { confirm = null }) { Text("Cancel") }
            }
        )
    }
}

@Composable
private fun PowerMenu(onAction: (PowerAction) -> Unit) {
    var open by remember { mutableStateOf(false) }

    Box {
        Surface(
            onClick = { open = true },
            shape = RoundedCornerShape(18.dp),
            color = MaterialTheme.colorScheme.surface,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
            modifier = Modifier.fillMaxWidth()
        ) {
            Row(
                Modifier.padding(horizontal = 16.dp, vertical = 13.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Icon(
                    Icons.Default.PowerSettingsNew,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary
                )
                Text(
                    "Power",
                    style = MaterialTheme.typography.titleSmall,
                    modifier = Modifier.weight(1f)
                )
                Icon(
                    Icons.Default.KeyboardArrowDown,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }

        DropdownMenu(
            expanded = open,
            onDismissRequest = { open = false },
            modifier = Modifier.width(280.dp)
        ) {
            PowerEntry(Icons.Default.Bedtime, "Sleep", "standby") {
                open = false
                onAction(PowerAction.Sleep)
            }
            PowerEntry(Icons.Default.Lock, "Lock", "secure") {
                open = false
                onAction(PowerAction.Lock)
            }
            PowerEntry(Icons.Default.TvOff, "Turn displays off", "screens sleep, PC stays up") {
                open = false
                onAction(PowerAction.Display)
            }
            HorizontalDivider(
                color = MaterialTheme.colorScheme.outline,
                modifier = Modifier.padding(vertical = 6.dp)
            )
            PowerEntry(Icons.Default.PowerSettingsNew, "Shut down", "5s delay", danger = true) {
                open = false
                onAction(PowerAction.Shutdown)
            }
            PowerEntry(Icons.Default.Refresh, "Reboot", "5s delay", danger = true) {
                open = false
                onAction(PowerAction.Reboot)
            }
        }
    }
}

@Composable
private fun PowerEntry(
    icon: ImageVector,
    label: String,
    sub: String,
    danger: Boolean = false,
    onClick: () -> Unit
) {
    DropdownMenuItem(
        onClick = onClick,
        leadingIcon = {
            Icon(
                icon,
                contentDescription = null,
                tint = if (danger) MaterialTheme.colorScheme.error
                else MaterialTheme.colorScheme.onSurfaceVariant
            )
        },
        text = {
            Column {
                Text(
                    label,
                    style = MaterialTheme.typography.titleSmall,
                    color = if (danger) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onSurface
                )
                Text(
                    sub,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    )
}
