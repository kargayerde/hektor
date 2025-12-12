#pragma once

#include "helpers.h"
#include "config.h"

// literals are gpio numbers
//============================= HiSense ESP32-D =============================
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
#define PHYSICAL_PIN_27 16 //	| RX2			|			|				| r1
#define PHYSICAL_PIN_28 17 //	| TX2			|			|				| r2
#define PHYSICAL_PIN_29 5  //	| VSPI SS		|			|				| r3
#define PHYSICAL_PIN_30 18 //	| VSPI SCK		|			|				| r4
#define PHYSICAL_PIN_31 19 //	| VSPI MISO		|			|				| r5
#define PHYSICAL_PIN_32	   //	| GND			|			|				|
#define PHYSICAL_PIN_33 21 //	| I2C SDA		|			|				| r6
#define PHYSICAL_PIN_34 3  //	| RX0			|			|				| r7
#define PHYSICAL_PIN_35 1  //	| TX0			|			|				| r8
#define PHYSICAL_PIN_36 22 //	| I2C SCL		|			|				|
#define PHYSICAL_PIN_37 23 //	| VSPI MOSI		|			|				|
#define PHYSICAL_PIN_38	   //	| GND			|			|				|
//==========================================================================|

enum RELAY_INDEXES {
	RELAY1 = 0,
	RELAY2,
	RELAY3,
	RELAY4,
	RELAY5,
	RELAY6,
	RELAY7,
	RELAY8
};

int relayPins[] = {
	PHYSICAL_PIN_35, // Relays 1-8 will be connected to physical pins 35, 34, 33, 31, 30, 29, 28, 27
	PHYSICAL_PIN_34,
	PHYSICAL_PIN_33,
	PHYSICAL_PIN_31,
	PHYSICAL_PIN_30,
	PHYSICAL_PIN_29,
	PHYSICAL_PIN_28,
	PHYSICAL_PIN_27,
};

const int PIN_LED_ONBOARD = 2; // ??????
const int PIN_LED = PHYSICAL_PIN_26;

uint8_t getRelayStatesByte() {
	uint8_t relayStates = 0;
	for (int i = 0; i < 8; i++) {
		int state = digitalRead(relayPins[i]) ? 1 : 0;
		relayStates |= (state << i);
	}
	return relayStates;
}

void toggleRelayAtIndex(int index) {
	if (index < 0 || index >= 8)
		return;
	int pin = relayPins[index];
	digitalWrite(pin, digitalRead(pin) == HIGH ? LOW : HIGH);
}

void reportRelayStatesSerial() {
    uint8_t states = getRelayStatesByte();
    char buf[16];
    snprintf(buf, sizeof(buf), "RELAYS:%02X\n", states);
    Serial.print(buf);
}

void reportRelayStatesTelnet() {
    if (telnetClient && telnetClient.connected()) {
        uint8_t states = getRelayStatesByte();
        char buf[16];
        snprintf(buf, sizeof(buf), "RELAYS:%02X\n", states);
        telnetClient.print(buf);
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

	for (int i = 0; i < 8; i++) {
		pinMode(relayPins[i], OUTPUT);
		digitalWrite(relayPins[i], LOW);
	}

	pinMode(PIN_LED, OUTPUT);
	pinMode(PIN_LED_ONBOARD, OUTPUT);
	digitalWrite(PIN_LED, LOW);
	digitalWrite(PIN_LED_ONBOARD, LOW);

	initBlink(PIN_LED_ONBOARD);

	hbLast = millis();
	hbSeq = 0;
}

void loop() {
	ArduinoOTA.handle();

	if (acceptTelnetClient()) {
		telnetClient.println("Connected to ESP32 Telnet console.");
		telnetClient.println("Press Ctrl-C or Ctrl-D to disconnect.");
		reportRelayStatesTelnet();
	}

	if (telnetClient && telnetClient.connected() && telnetClient.available()) {
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

			if (c >= '1' && c <= '8') {
				int relIndex = c - '1';
				toggleRelayAtIndex(relIndex);
				delay(20);
				reportRelayStatesTelnet();
				Serial.print("[TELNET] Toggled relay ");
				Serial.println(relIndex + 1);
			}
		}
	}

	if (Serial.available() > 0) {
		int inByte = Serial.read();
		if (inByte >= '1' && inByte <= '8') {
			int relIndex = inByte - '1';
			toggleRelayAtIndex(relIndex);
			delay(20);
			reportRelayStatesSerial();
			Serial.print("[SERIAL] Toggled relay ");
			Serial.println(relIndex + 1);
		}
	}

	unsigned long now = millis();
	bool connected = (WiFi.status() == WL_CONNECTED);

	if (now - hbLast >= HEARTBEAT_INTERVAL_MS) {
		hbLast = now;
		hbSeq++;
		sendHeartbeatSerial();
		sendHeartbeatTelnet();
	}

	updateBlink(connected, PIN_LED_ONBOARD);
}
