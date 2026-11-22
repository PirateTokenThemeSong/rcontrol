.PHONY: server run tidy build android-debug

server:
	cd apps/server && go build -o bin/rcontrol-server.exe .

run:
	cd apps/server && go run . --no-auth

tidy:
	cd apps/server && go mod tidy

android-debug:
	cd apps/android && ./gradlew assembleDebug
