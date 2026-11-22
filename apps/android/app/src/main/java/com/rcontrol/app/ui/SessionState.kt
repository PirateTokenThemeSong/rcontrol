package com.rcontrol.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.rcontrol.app.net.RControlClient

/** Connection details for the current session (not the saved settings). */
class SessionState {
    var host by mutableStateOf("")
    var port by mutableStateOf("7945")
    var token by mutableStateOf("")
    var pcName by mutableStateOf("")
    var client: RControlClient? = null

    val label: String get() = pcName.ifBlank { host }
}
