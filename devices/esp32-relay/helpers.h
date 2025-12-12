#pragma once
#include <WiFi.h>
#include <ArduinoOTA.h>

static WiFiServer telnetServer(23);
static WiFiClient telnetClient;

static const unsigned long HEARTBEAT_INTERVAL_MS = 1000;
static unsigned long hbLast = 0;
static uint8_t hbSeq = 0;

static const unsigned long SHORT_BLINK = 100;
static const unsigned long LONG_SILENCE = 1000;
static const unsigned long DISCONNECTED_BLINK = 500;

static unsigned long blinkLast = 0;
static int blinkPhase = 0;
static bool blinkState = false;
static bool prevConnected = false;

static inline void beginTelnet() {
	telnetServer.begin();
	telnetServer.setNoDelay(true);
	Serial.println("[TELNET] Server started");
}

static inline bool acceptTelnetClient() {
	if (telnetServer.hasClient()) {
		if (telnetClient && telnetClient.connected()) {
			telnetClient.stop();
			Serial.println("[TELNET] Replacing old client");
		}
		telnetClient = telnetServer.available();
		Serial.println("[TELNET] Client connected");
		return true;
	}
	if (telnetClient && !telnetClient.connected()) {
		telnetClient.stop();
		Serial.println("[TELNET] Client disconnected");
	}
	return false;
}

static inline void telnetPrintln(const String &msg) {
	if (telnetClient && telnetClient.connected()) {
		telnetClient.println(msg);
	}
}

static inline void sendHeartbeatSerial() {
	char buf[16];
	snprintf(buf, sizeof(buf), "HB:%02X\n", hbSeq);
	Serial.print(buf);
}

static inline void sendHeartbeatTelnet() {
	if (telnetClient && telnetClient.connected()) {
		char buf[16];
		snprintf(buf, sizeof(buf), "HB:%02X\n", hbSeq);
		telnetClient.print(buf);
	}
}

static inline void setupOTA(const char *hostname, const char *password) {
	ArduinoOTA.setHostname(hostname);
	ArduinoOTA.setPassword(password);

	ArduinoOTA
		.onStart([]() { Serial.println("\n[OTA] Update start"); })
		.onEnd([]() { Serial.println("\n[OTA] Update complete, rebooting..."); })
		.onProgress([](unsigned int progress, unsigned int total) {
			Serial.printf("[OTA] Progress: %u%%\r", (progress / (total / 100)));
		})
		.onError([](ota_error_t error) {
			Serial.printf("\n[OTA] Error[%u]: ", error);
			if (error == OTA_AUTH_ERROR)
				Serial.println("Auth failed");
			else if (error == OTA_BEGIN_ERROR)
				Serial.println("Begin failed");
			else if (error == OTA_CONNECT_ERROR)
				Serial.println("Connect failed");
			else if (error == OTA_RECEIVE_ERROR)
				Serial.println("Receive failed");
			else if (error == OTA_END_ERROR)
				Serial.println("End failed");
		});

	ArduinoOTA.begin();
	Serial.println(String("[OTA] Ready. Use ") + String(hostname) + String(".local or IP for uploads."));
}

static inline void initBlink(int pinLedOnboard) {
	blinkLast = millis();
	blinkPhase = 0;
	blinkState = false;
	prevConnected = (WiFi.status() == WL_CONNECTED);
	if (prevConnected)
		digitalWrite(pinLedOnboard, HIGH);
	else
		digitalWrite(pinLedOnboard, LOW);
}

static inline void updateBlink(bool connected, int pinLedOnboard) {
	unsigned long now = millis();

	if (connected != prevConnected) {
		prevConnected = connected;
		blinkLast = now;
		blinkPhase = 0;
		blinkState = false;
		if (connected)
			digitalWrite(pinLedOnboard, HIGH);
		else
			digitalWrite(pinLedOnboard, LOW);
	}

	if (connected) {
		static const unsigned long connectedDurations[] = {SHORT_BLINK, SHORT_BLINK, SHORT_BLINK, SHORT_BLINK, SHORT_BLINK, LONG_SILENCE};
		static const bool outputs[] = {true, false, true, false, true, false};
		const int phases = sizeof(connectedDurations) / sizeof(connectedDurations[0]);

		if (now - blinkLast >= connectedDurations[blinkPhase]) {
			blinkLast = now;
			blinkPhase = (blinkPhase + 1) % phases;
			bool out = outputs[blinkPhase];
			digitalWrite(pinLedOnboard, out ? HIGH : LOW);
		}
	} else {
		// equal on/off blink
		if (now - blinkLast >= DISCONNECTED_BLINK) {
			blinkLast = now;
			blinkState = !blinkState;
			digitalWrite(pinLedOnboard, blinkState ? HIGH : LOW);
		}
	}
}