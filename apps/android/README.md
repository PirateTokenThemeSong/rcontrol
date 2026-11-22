# rcontrol Android app (native, Kotlin + Compose)
#
# Open in Android Studio (Hedgehog or newer) — it provisions Gradle 8.7
# via gradle/wrapper/gradle-wrapper.properties automatically.
#
# NOTE: system Gradle 9.x does NOT work with AGP 8.5.x. Use the wrapper:
#   cd apps/android
#   ./gradlew assembleDebug        # macOS/Linux
#   .\gradlew.bat assembleDebug    # Windows
#
# Then: install app-debug.apk on a physical device on the same Wi-Fi as your PC.
# Home only shows actions (Connect / Wake / Scan). Everything you type lives
# behind the gear icon: address, port, token, PIN pairing, stream quality and
# the wake configuration.
#
# Wake screen: set the PC MAC for direct WoL magic packets and/or configure a
# custom HTTP request (method/URL/headers/body) to whatever wakes your PC
# (router, Home Assistant, Pi). The app polls /health until the PC is back,
# then drops you into the control session.
#
# Control screen: live stream, power actions and an Apps section (from server
# GET /apps) — each row launches that allowlisted program on the PC.
#
# If `./gradlew` is missing the jar (fresh checkout without Studio), run once:
#   gradle wrapper --gradle-version 8.7
