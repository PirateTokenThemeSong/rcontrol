package com.rcontrol.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController

@Composable
fun App() {
    val nav = rememberNavController()
    // Session details survive navigation; configuration lives in Settings.
    val session = remember { SessionState() }
    NavHost(navController = nav, startDestination = "home") {
        composable("home") {
            HomeScreen(
                session = session,
                onConnect = { nav.navigate("control") },
                onWake = { nav.navigate("wake") },
                onSettings = { nav.navigate("settings") }
            )
        }
        composable("wake") {
            WakeScreen(
                session = session,
                onOnline = { nav.navigate("control") { popUpTo("home") } },
                onSettings = { nav.navigate("settings") },
                onBack = { nav.popBackStack() }
            )
        }
        composable("control") {
            ControlScreen(
                session = session,
                onDisconnect = { nav.popBackStack("home", false) },
                onSettings = { nav.navigate("settings") }
            )
        }
        composable("settings") {
            SettingsScreen(onBack = { nav.popBackStack() })
        }
    }
}
