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
import androidx.compose.ui.res.stringResource
import com.rcontrol.app.R
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
                title = { Text(stringResource(R.string.action_settings), style = MaterialTheme.typography.titleLarge) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.cd_back))
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
                    SectionTitle(stringResource(R.string.section_connection), trailing = {
                        if (s.token.isNotBlank()) StatusPill(stringResource(R.string.pill_paired), PillTone.Good)
                    })
                    SettingsField(stringResource(R.string.field_pc_name), s.pcName, stringResource(R.string.placeholder_pc_name)) { v ->
                        set { it.copy(pcName = v) }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SettingsField(
                            stringResource(R.string.field_address), s.host, stringResource(R.string.placeholder_address),
                            modifier = Modifier.weight(2.2f)
                        ) { v -> set { it.copy(host = v) } }
                        SettingsField(
                            stringResource(R.string.field_port), s.port, "",
                            keyboard = KeyboardType.Number,
                            modifier = Modifier.weight(1f)
                        ) { v -> set { it.copy(port = v) } }
                    }
                    if (s.token.isBlank()) {
                        Text(
                            stringResource(R.string.section_pairing),
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
                                label = { Text(stringResource(R.string.field_pin)) },
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
                                            notice = ctx.getString(R.string.notice_paired) to false
                                        } catch (e: Exception) {
                                            notice = ctx.getString(R.string.pair_failed, e.message ?: "") to true
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
                                    Text(stringResource(R.string.action_pair))
                                }
                            }
                        }
                        Text(
                            stringResource(R.string.pairing_help),
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
                                Text(stringResource(R.string.paired_title), style = MaterialTheme.typography.titleSmall)
                                Text(
                                    stringResource(R.string.paired_hint),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                            OutlinedButton(
                                onClick = {
                                    set { it.copy(token = "") }
                                    notice = ctx.getString(R.string.notice_unpaired) to false
                                },
                                shape = RoundedCornerShape(14.dp)
                            ) { Text(stringResource(R.string.action_unpair)) }
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
                    SectionTitle(stringResource(R.string.section_stream), trailing = {
                        Text(
                            stringResource(R.string.stream_summary, s.fps, s.jpeg),
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.primary
                        )
                    })
                    SliderRow(stringResource(R.string.stream_fps), "${s.fps}") {
                        Slider(
                            value = s.fps.toFloat(),
                            onValueChange = { v -> set { it.copy(fps = v.roundToInt()) } },
                            valueRange = 1f..15f,
                            steps = 13
                        )
                    }
                    SliderRow(stringResource(R.string.stream_jpeg), "${s.jpeg}") {
                        Slider(
                            value = s.jpeg.toFloat(),
                            onValueChange = { v -> set { it.copy(jpeg = v.roundToInt()) } },
                            valueRange = 30f..90f,
                            steps = 11
                        )
                    }
                    SliderRow(stringResource(R.string.stream_max_width), stringResource(R.string.stream_max_width_value, s.maxWidth)) {
                        Slider(
                            value = s.maxWidth.toFloat(),
                            onValueChange = { v -> set { it.copy(maxWidth = v.roundToInt()) } },
                            valueRange = 320f..1920f,
                            steps = 9
                        )
                    }
                    Text(
                        stringResource(R.string.stream_hint),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            // ---- wake ----
            item {
                Panel {
                    SectionTitle(stringResource(R.string.section_wol))
                    SettingsField(stringResource(R.string.field_mac), s.mac, stringResource(R.string.placeholder_mac)) { v ->
                        set { it.copy(mac = v) }
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        SettingsField(
                            stringResource(R.string.field_broadcast), s.broadcast, "",
                            modifier = Modifier.weight(2f)
                        ) { v -> set { it.copy(broadcast = v) } }
                        SettingsField(
                            stringResource(R.string.field_port), s.wolPort, "",
                            keyboard = KeyboardType.Number,
                            modifier = Modifier.weight(1f)
                        ) { v -> set { it.copy(wolPort = v) } }
                    }
                    Text(
                        stringResource(R.string.wol_hint),
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
                            SectionTitle(stringResource(R.string.section_custom_wake))
                        }
                        Switch(
                            checked = s.wakeHttpEnabled,
                            onCheckedChange = { v -> set { it.copy(wakeHttpEnabled = v) } }
                        )
                    }
                    Text(
                        stringResource(R.string.custom_wake_hint),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )

                    if (s.wakeHttpEnabled) {
                        SettingsField(stringResource(R.string.field_method), s.wakeMethod, stringResource(R.string.placeholder_method)) { v ->
                            set { it.copy(wakeMethod = v) }
                        }
                        SettingsField(stringResource(R.string.field_url), s.wakeUrl, stringResource(R.string.placeholder_url)) { v ->
                            set { it.copy(wakeUrl = v) }
                        }
                        SettingsField(
                            stringResource(R.string.field_headers), s.wakeHeaders, stringResource(R.string.placeholder_headers),
                            singleLine = false,
                            minLines = 2
                        ) { v -> set { it.copy(wakeHeaders = v) } }
                        SettingsField(
                            stringResource(R.string.field_body), s.wakeBody, stringResource(R.string.placeholder_body),
                            singleLine = false,
                            minLines = 3
                        ) { v -> set { it.copy(wakeBody = v) } }
                    }
                }
            }

            // ---- about ----
            item {
                Text(
                    stringResource(R.string.about),
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
