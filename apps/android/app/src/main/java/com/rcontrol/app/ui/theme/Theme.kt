package com.rcontrol.app.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// ---- palette ----
val Ink = Color(0xFF071013)
val Panel = Color(0xFF0C1A1E)
val PanelHi = Color(0xFF11252A)
val Hairline = Color(0xFF1C3A40)
val TextMain = Color(0xFFE4FAFC)
val TextDim = Color(0xFF86A9B0)
val Ice = Color(0xFF38E1F0)
val IceSoft = Color(0xFFB4F4FA)
// Kept for status semantics: online / warning / error.
val Mint = Color(0xFF4ADE80)
val Amber = Color(0xFFFBBF24)
val Coral = Color(0xFFF87171)
val Cyan = Color(0xFF38E1F0)

private val Scheme = darkColorScheme(
    primary = Ice,
    onPrimary = Color(0xFF03191C),
    primaryContainer = Color(0xFF0D3A41),
    onPrimaryContainer = IceSoft,
    secondary = Color(0xFF8FEAF4),
    onSecondary = Color(0xFF04252A),
    tertiary = Mint,
    background = Ink,
    onBackground = TextMain,
    surface = Panel,
    onSurface = TextMain,
    surfaceVariant = PanelHi,
    onSurfaceVariant = TextDim,
    surfaceTint = Ice,
    outline = Hairline,
    outlineVariant = Hairline,
    error = Coral,
    onError = Color.White,
    errorContainer = Color(0xFF3A1B1B),
    onErrorContainer = Color(0xFFFFC9C9),
)

private val AppTypography = Typography(
    displaySmall = TextStyle(fontSize = 34.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.5).sp),
    headlineSmall = TextStyle(fontSize = 24.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.3).sp),
    titleLarge = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold),
    titleSmall = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.6.sp),
    bodyLarge = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.Normal, lineHeight = 22.sp),
    bodyMedium = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.Normal, lineHeight = 20.sp),
    bodySmall = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.Normal, lineHeight = 16.sp),
    labelLarge = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.2.sp),
    labelMedium = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.Medium, letterSpacing = 0.4.sp),
    labelSmall = TextStyle(fontSize = 11.sp, fontWeight = FontWeight.Medium, letterSpacing = 0.6.sp),
)

@Composable
fun RControlTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = Scheme,
        typography = AppTypography,
        content = content
    )
}
