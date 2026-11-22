@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.rcontrol.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Switch
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.rcontrol.app.net.pairWithPin
import com.rcontrol.app.ui.components.Panel
import com.rcontrol.app.ui.components.PillTone
import com.rcontrol.app.ui.components.SectionTitle
import com.rcontrol.app.ui.components.StatusPill
import kotlinx.coroutines.launch
import kotlin.math.roundToInt

/**
 * Everything you'd otherwise have to type on Home: address, token, pairing,
 * streaming quality and the wake configuration. Nothing here is an action —
 * it only writes to [Settings], which the other screens read.
 */
@Composable
fun SettingsScreen(onBack: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    var s by remember { mutableStateOf(loadSettings(ctx)) }
    var pin by remember { mutableStateOf("") }
    var pairing by remember { mutableStateOf(false) }
    var notice by remember { mutableStateOf<Pair<String, Boolean>?>(null) }

    fun set(block: (Settings) -> Settings) {
        s = block(s)
        saveSettings(ctx, s)
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Settings", style = MaterialTheme.typography.titleLarge) },
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
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 28.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            // ---- connection ----
            item {
                Panel {
                    SectionTitle("Connection", trailing = {
                        if (s.token.isNotBlank()) StatusPill("paired", PillTone.Good)
                    })
                    SettingsField("PC name", s.pcName, "optional, shown on Home") { v ->
                        set { it.copy(pcName = v) }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SettingsField(
                            "Address", s.host, "192.168.1.20",
                            modifier = Modifier.weight(2.2f)
                        ) { v -> set { it.copy(host = v) } }
                        SettingsField(
                            "Port", s.port, "",
                            keyboard = KeyboardType.Number,
                            modifier = Modifier.weight(1f)
                        ) { v -> set { it.copy(port = v) } }
                    }
                    if (s.token.isBlank()) {
                        Text(
                            "Pairing",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 18.dp)
                        )
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            OutlinedTextField(
                                value = pin,
                                onValueChange = { pin = it },
                                label = { Text("PIN (000-000)") },
                                leadingIcon = { Icon(Icons.Default.Shield, null) },
                                singleLine = true,
                                shape = RoundedCornerShape(14.dp),
                                colors = settingsFieldColors(),
                                modifier = Modifier
                                    .weight(1f)
                                    .padding(top = 10.dp)
                            )
                            OutlinedButton(
                                enabled = pin.isNotBlank() && s.host.isNotBlank() && !pairing,
                                onClick = {
                                    pairing = true
                                    notice = null
                                    scope.launch {
                                        try {
                                            val token = pairWithPin(s.host.trim(), s.portInt, pin.trim())
                                            set { it.copy(token = token) }
                                            pin = ""
                                            notice = "Paired." to false
                                        } catch (e: Exception) {
                                            notice = (e.message ?: "pairing failed") to true
                                        } finally {
                                            pairing = false
                                        }
                                    }
                                },
                                shape = RoundedCornerShape(14.dp),
                                modifier = Modifier
                                    .height(56.dp)
                                    .padding(top = 10.dp)
                            ) {
                                if (pairing) {
                                    CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                                } else {
                                    Text("Pair")
                                }
                            }
                        }
                        Text(
                            "The PIN is shown on the PC (also in ~/.rcontrol/pin). Pairing stores a token on this phone — you never type it.",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 8.dp)
                        )
                    } else {
                        Row(
                            modifier = Modifier.padding(top = 18.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text("Paired", style = MaterialTheme.typography.titleSmall)
                                Text(
                                    "Token stored on this phone.",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                            OutlinedButton(
                                onClick = {
                                    set { it.copy(token = "") }
                                    notice = "Unpaired — pair again with the PIN." to false
                                },
                                shape = RoundedCornerShape(14.dp)
                            ) { Text("Unpair") }
                        }
                    }

                    notice?.let { (text, isError) ->
                        Text(
                            text,
                            style = MaterialTheme.typography.bodySmall,
                            color = if (isError) MaterialTheme.colorScheme.error
                            else MaterialTheme.colorScheme.tertiary,
                            modifier = Modifier.padding(top = 10.dp)
                        )
                    }
                }
            }

            // ---- streaming ----
            item {
                Panel {
                    SectionTitle("Stream", trailing = {
                        Text(
                            "${s.fps} fps · ${s.jpeg}%",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                    })
                    SliderRow("Frames per second", "${s.fps}") {
                        Slider(
                            value = s.fps.toFloat(),
                            onValueChange = { v -> set { it.copy(fps = v.roundToInt()) } },
                            valueRange = 1f..15f,
                            steps = 13
                        )
                    }
                    SliderRow("JPEG quality", "${s.jpeg}") {
                        Slider(
                            value = s.jpeg.toFloat(),
                            onValueChange = { v -> set { it.copy(jpeg = v.roundToInt()) } },
                            valueRange = 30f..90f,
                            steps = 11
                        )
                    }
                    SliderRow("Max width", "${s.maxWidth} px") {
                        Slider(
                            value = s.maxWidth.toFloat(),
                            onValueChange = { v -> set { it.copy(maxWidth = v.roundToInt()) } },
                            valueRange = 320f..1920f,
                            steps = 9
                        )
                    }
                    Text(
                        "Lower quality = smoother over weak Wi-Fi.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            // ---- wake ----
            item {
                Panel {
                    SectionTitle("Wake on LAN")
                    SettingsField("MAC address", s.mac, "AA:BB:CC:DD:EE:FF") { v ->
                        set { it.copy(mac = v) }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SettingsField(
                            "Broadcast", s.broadcast, "",
                            modifier = Modifier.weight(2f)
                        ) { v -> set { it.copy(broadcast = v) } }
                        SettingsField(
                            "Port", s.wolPort, "",
                            keyboard = KeyboardType.Number,
                            modifier = Modifier.weight(1f)
                        ) { v -> set { it.copy(wolPort = v) } }
                    }
                    Text(
                        "The app sends the magic packet itself; your PC must allow WoL in the BIOS and NIC.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(top = 10.dp)
                    )
                }
            }

            // ---- custom wake request ----
            item {
                Panel {
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(Modifier.weight(1f)) {
                            SectionTitle("Custom wake request")
                        }
                        Switch(
                            checked = s.wakeHttpEnabled,
                            onCheckedChange = { v -> set { it.copy(wakeHttpEnabled = v) } }
                        )
                    }
                    Text(
                        "Fire an HTTP request at a router, Home Assistant or always-on Pi.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )

                    if (s.wakeHttpEnabled) {
                        SettingsField("Method", s.wakeMethod, "POST") { v ->
                            set { it.copy(wakeMethod = v) }
                        }
                        SettingsField("URL", s.wakeUrl, "http://…") { v ->
                            set { it.copy(wakeUrl = v) }
                        }
                        SettingsField(
                            "Headers (one per line)", s.wakeHeaders, "Authorization: Bearer …",
                            singleLine = false,
                            minLines = 2
                        ) { v -> set { it.copy(wakeHeaders = v) } }
                        SettingsField(
                            "Body", s.wakeBody, "{\"mac\":\"…\"}",
                            singleLine = false,
                            minLines = 3
                        ) { v -> set { it.copy(wakeBody = v) } }
                    }
                }
            }

            // ---- about ----
            item {
                Text(
                    "rcontrol 0.1.0 · LAN only, no cloud, no TLS in v1.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(start = 4.dp, top = 2.dp)
                )
            }
        }
    }
}

@Composable
private fun SettingsField(
    label: String,
    value: String,
    placeholder: String = "",
    keyboard: KeyboardType = KeyboardType.Text,
    singleLine: Boolean = true,
    minLines: Int = 1,
    modifier: Modifier = Modifier,
    onChange: (String) -> Unit,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        placeholder = { if (placeholder.isNotBlank()) Text(placeholder) },
        singleLine = singleLine,
        minLines = minLines,
        keyboardOptions = KeyboardOptions(keyboardType = keyboard),
        shape = RoundedCornerShape(14.dp),
        colors = settingsFieldColors(),
        modifier = modifier
            .fillMaxWidth()
            .padding(top = 10.dp)
    )
}

@Composable
private fun SliderRow(label: String, value: String, content: @Composable () -> Unit) {
    Column(Modifier.padding(top = 14.dp)) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Text(
                label,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                value,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
        content()
    }
}

@Composable
private fun settingsFieldColors() = OutlinedTextFieldDefaults.colors(
    focusedBorderColor = MaterialTheme.colorScheme.primary,
    unfocusedBorderColor = MaterialTheme.colorScheme.outline,
    focusedLabelColor = MaterialTheme.colorScheme.primary,
    unfocusedLabelColor = MaterialTheme.colorScheme.onSurfaceVariant,
    focusedTextColor = MaterialTheme.colorScheme.onSurface,
    unfocusedTextColor = MaterialTheme.colorScheme.onSurface,
    cursorColor = MaterialTheme.colorScheme.primary,
    focusedPlaceholderColor = MaterialTheme.colorScheme.onSurfaceVariant,
    unfocusedPlaceholderColor = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
)
