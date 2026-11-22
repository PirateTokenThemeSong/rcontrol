package com.rcontrol.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import com.rcontrol.app.ui.App
import com.rcontrol.app.ui.theme.RControlTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            RControlTheme {
                App()
            }
        }
    }
}
