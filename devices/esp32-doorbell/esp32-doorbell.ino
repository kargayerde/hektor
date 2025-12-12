#pragma once

#include "config.h"
#include "helpers.h"

// literals are gpio numbers
//============================= HiLetgo ESP32-D =============================
//		| Physical	  | GPIO	| Function		| Alt		|	Alt2		|
#define PHYSICAL_PIN_1	   //	| 3V3			|			|				|
#define PHYSICAL_PIN_2 36  //	| EN 			| Reset		|				|
#define PHYSICAL_PIN_3 36  //	| ADC0			|			|				|
#define PHYSICAL_PIN_4 39  //	| ADC3			|			|				|
#define PHYSICAL_PIN_5 34  //	| ADC6			|			|				|
#define PHYSICAL_PIN_6 35  //	| ADC7			|			|				|
#define PHYSICAL_PIN_7 32  //	| ADC4			| Touch9	|				|
#define PHYSICAL_PIN_8 33  //	| ADC5			| Touch8	|				|
#define PHYSICAL_PIN_9 25  //	| ADC18			| DAC1		|				|
#define PHYSICAL_PIN_10 26 //	| ADC19			| DAC2		|				|
#define PHYSICAL_PIN_11 27 //	| ADC17			| Touch7	|				|
#define PHYSICAL_PIN_12 14 //	| ADC16			| Touch6	|				|
#define PHYSICAL_PIN_13 12 //	| ADC15			| Touch5	|				|
#define PHYSICAL_PIN_14	   //	| GND			|			|				|
#define PHYSICAL_PIN_15 13 //	| ADC14			| Touch4	|				|
#define PHYSICAL_PIN_16 9  //	| Flash D2		| RX1		|				|
#define PHYSICAL_PIN_17 10 //	| Flash D3		| TX1		|				|
#define PHYSICAL_PIN_18 11 //	| Flash CMD		|			|				|
#define PHYSICAL_PIN_19	   //	| VIN 5V		|			|				|
//------------------------------|---------------|-----------|---------------|
#define PHYSICAL_PIN_20 6  //	| Flash CK		|			|				|
#define PHYSICAL_PIN_21 7  //	| Flash D0		|			|				|
#define PHYSICAL_PIN_22 8  //	| Flash D1		|			|				|
#define PHYSICAL_PIN_23 15 //	| ADC13			| Touch3	|				|
#define PHYSICAL_PIN_24 2  //	| ADC12			| Touch2	| ONBOARD LED	|
#define PHYSICAL_PIN_25 0  //	| ADC11			| Touch1	|				|
#define PHYSICAL_PIN_26 4  //	| ADC10			| Touch0	|				|
#define PHYSICAL_PIN_27 16 //	| RX2			|			|				|
#define PHYSICAL_PIN_28 17 //	| TX2			|			|				|
#define PHYSICAL_PIN_29 5  //	| VSPI SS		|			|				|
#define PHYSICAL_PIN_30 18 //	| VSPI SCK		|			|				|
#define PHYSICAL_PIN_31 19 //	| VSPI MISO		|			|				| // doorbell (ring) connected here (active LOW)
#define PHYSICAL_PIN_32	   //	| GND			|			|				|
#define PHYSICAL_PIN_33 21 //	| I2C SDA		|			|				| // buzzer switched to this instead
#define PHYSICAL_PIN_34 3  //	| RX0			|			|				|
#define PHYSICAL_PIN_35 1  //	| TX0			|			|				| // apparently not safe during boot (gets pulled high)
#define PHYSICAL_PIN_36 22 //	| I2C SCL		|			|				|
#define PHYSICAL_PIN_37 23 //	| VSPI MOSI		|			|				|
#define PHYSICAL_PIN_38	   //	| GND			|			|				|
//==========================================================================|

enum RELAY_INDEXES {
	BUZZER = 0,
	RELAY_COUNT
};

int relayPins[] = {
	// PHYSICAL_PIN_35,
	PHYSICAL_PIN_33,
};

const int PIN_LED_ONBOARD = 2;
const int PIN_DOORBELL_RING = PHYSICAL_PIN_31; // active LOW input

// ring monitoring state
bool lastDoorbellState = HIGH; // assume idle (pulled up)
unsigned long lastRingChangeMs = 0;
// const unsigned long RING_DEBOUNCE_MS = 50;
const unsigned long RING_DEBOUNCE_MS = 500;

void pulseRelay(int index, int durationMs) {
	int pin = relayPins[index];
	digitalWrite(pin, HIGH);
	delay(durationMs);
	digitalWrite(pin, LOW);
}

void buzzDoor() {
	pulseRelay(BUZZER, 400);
}

// monitor doorbell ring input, print when ringing
void monitorDoorRing() {
	unsigned long now = millis();
	int current = digitalRead(PIN_DOORBELL_RING);

	if (current == LOW) {
		telnetPrintln("------------------------------- DOORBELL STATE: " + String(current));
	}

	// debounce
	if (current != lastDoorbellState) {
		if (now - lastRingChangeMs < RING_DEBOUNCE_MS) {
			return;
		}
		lastRingChangeMs = now;

		// detect transition to active LOW (ringing)
		if (current == LOW && lastDoorbellState == HIGH) {
			Serial.println("*************** Door is ringing");
			telnetPrintln("*************** Door is ringing");
			delay(2000);
			buzzDoor();
		}

		lastDoorbellState = current;
	}
}

void handleTelnet() {
	if (acceptTelnetClient()) {
		telnetPrintln("Connected to ESP32 Telnet console.");
		telnetPrintln("Press Ctrl-C or Ctrl-D to disconnect.");
	}

	if (!(telnetClient && telnetClient.connected()))
		return;

	while (telnetClient.available()) {
		int inByte = telnetClient.read();
		if (inByte < 0)
			break;
		char c = (char) inByte;

		if (c == 3 || c == 4) { // Ctrl-C or Ctrl-D
			telnetClient.println();
			telnetClient.println("Session terminated by client (Ctrl-C/Ctrl-D).");
			telnetClient.stop();
			Serial.println("[TELNET] Client detached via control character");
			break;
		}

		if (c == '\r' || c == '\n')
			continue;

		if (c == '1') {
			telnetClient.print("Buzzing:\n");
			buzzDoor();
			telnetClient.print("Buzzed:\n");
		}
	}
}

void setup() {
	Serial.begin(115200);
	delay(1000);
	Serial.println();
	Serial.println("Connecting to WiFi...");

	WiFi.mode(WIFI_STA);
	WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

	unsigned long start = millis();
	while (WiFi.status() != WL_CONNECTED) {
		delay(500);
		Serial.print(".");
		if (millis() - start > 15000) {
			Serial.println("\nConnection failed!");
			return;
		}
	}

	Serial.println("\nWiFi connected!");
	Serial.print("----> Local IP: ");
	Serial.println(WiFi.localIP());

	setupOTA(OTA_HOSTNAME, OTA_PASSWORD);

	beginTelnet();

	for (int i = 0; i < RELAY_COUNT; i++) {
		pinMode(relayPins[i], OUTPUT);
		digitalWrite(relayPins[i], LOW);
	}

	pinMode(PIN_LED_ONBOARD, OUTPUT);
	digitalWrite(PIN_LED_ONBOARD, LOW);

	pinMode(PIN_DOORBELL_RING, INPUT);

	initBlink(PIN_LED_ONBOARD);
}

void loop() {
	ArduinoOTA.handle();
	handleTelnet();

	monitorDoorRing();

	if (Serial.available() > 0) {
		int inByte = Serial.read();
		if (inByte == '1') {
			Serial.print("[SERIAL] buzzing door\n");
			buzzDoor();
			Serial.print("[SERIAL] buzzed door\n");
		}
	}

	unsigned long now = millis();
	bool connected = (WiFi.status() == WL_CONNECTED);

	if (now - hbLast >= HEARTBEAT_INTERVAL_MS) {
		hbLast = now;
		sendHeartbeatSerial();
		sendHeartbeatTelnet();
		hbSeq++;
	}

	updateBlink(connected, PIN_LED_ONBOARD);
}
