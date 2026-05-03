#include <Arduino.h>
#include <ArduinoJson.h>
#include <DNSServer.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <TFT_eSPI.h>
#include <WebServer.h>
#include <WiFi.h>

namespace {

constexpr const char *FW_VERSION = "0.1.0-dev";
constexpr const char *AP_NAME = "PVE-Desk-Setup";
constexpr uint8_t DNS_PORT = 53;
constexpr uint8_t BTN_A = 0;   // BOOT
constexpr uint8_t BTN_B = 14;  // LILYGO user button
constexpr uint8_t BACKLIGHT_PIN = 38;
constexpr unsigned long POLL_MS = 10000;
constexpr unsigned long BUTTON_LONG_MS = 1200;
constexpr uint8_t SCREEN_COUNT = 5;
constexpr size_t MAX_HOSTS = 6;
constexpr size_t MAX_GUESTS = 10;
constexpr size_t MAX_ALERTS = 8;

TFT_eSPI tft;
Preferences prefs;
DNSServer dnsServer;
WebServer webServer(80);

struct DeviceConfig {
  String ssid;
  String password;
  String bridgeURL;
  String displayToken;
  String deviceName;
  uint8_t brightness = 220;
  bool configured = false;
};

struct Summary {
  String health = "unknown";
  int hostsOnline = 0;
  int hostsTotal = 0;
  int guestsRunning = 0;
  int guestsStopped = 0;
  int alerts = 0;
};

struct Host {
  String name;
  bool online = false;
  int cpu = 0;
  int memory = 0;
  int storage = 0;
  int running = 0;
  int stopped = 0;
  String health = "unknown";
};

struct Guest {
  String name;
  String type;
  String hostName;
  String status;
  int cpu = 0;
  int maxCPU = 0;
  int memory = 0;
  int disk = 0;
  int64_t memoryUsed = 0;
  int64_t memoryTotal = 0;
  int64_t diskUsed = 0;
  int64_t diskTotal = 0;
  int64_t uptime = 0;
  int64_t netIn = 0;
  int64_t netOut = 0;
  int64_t diskRead = 0;
  int64_t diskWrite = 0;
  bool pinned = false;
  String health = "unknown";
};

struct Alert {
  String severity;
  String title;
  String message;
};

struct DisplayState {
  String schema;
  String generatedAt;
  bool stale = true;
  Summary summary;
  Host hosts[MAX_HOSTS];
  size_t hostCount = 0;
  Guest guests[MAX_GUESTS];
  size_t guestCount = 0;
  Alert alerts[MAX_ALERTS];
  size_t alertCount = 0;
};

DeviceConfig cfg;
DisplayState state;
String lastError;
String deviceIP;
unsigned long lastPoll = 0;
unsigned long lastOK = 0;
uint8_t screenIndex = 0;
size_t selectedGuest = 0;

struct ButtonState {
  bool previous = false;
  unsigned long pressedAt = 0;
  bool longHandled = false;
};

ButtonState buttonA;
ButtonState buttonB;

uint16_t colorForHealth(const String &health) {
  if (health == "ok") return TFT_GREEN;
  if (health == "warning") return TFT_YELLOW;
  if (health == "critical") return TFT_RED;
  return TFT_DARKGREY;
}

String htmlEscape(String value) {
  value.replace("&", "&amp;");
  value.replace("<", "&lt;");
  value.replace(">", "&gt;");
  value.replace("\"", "&quot;");
  return value;
}

String trimTrailingSlash(String value) {
  while (value.endsWith("/")) {
    value.remove(value.length() - 1);
  }
  return value;
}

String formatBytes(int64_t bytes) {
  if (bytes <= 0) return "-";
  const double gib = 1024.0 * 1024.0 * 1024.0;
  const double mib = 1024.0 * 1024.0;
  if (bytes >= static_cast<int64_t>(gib)) {
    double value = bytes / gib;
    if (value >= 10.0) return String(static_cast<int>(round(value))) + "G";
    return String(value, 1) + "G";
  }
  if (bytes >= static_cast<int64_t>(mib)) {
    return String(static_cast<int>(round(bytes / mib))) + "M";
  }
  return String(bytes / 1024) + "K";
}

String formatUptime(int64_t seconds) {
  if (seconds <= 0) return "-";
  int64_t days = seconds / 86400;
  int64_t hours = (seconds % 86400) / 3600;
  int64_t minutes = (seconds % 3600) / 60;
  if (days > 0) return String(days) + "d " + String(hours) + "h";
  if (hours > 0) return String(hours) + "h " + String(minutes) + "m";
  return String(minutes) + "m";
}

void setBacklight(uint8_t brightness) {
  pinMode(BACKLIGHT_PIN, OUTPUT);
  analogWrite(BACKLIGHT_PIN, brightness);
}

void drawHeader(const String &title, const String &status) {
  tft.fillScreen(TFT_BLACK);
  tft.setTextDatum(TL_DATUM);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.setTextSize(2);
  tft.drawString(title, 8, 6);
  tft.setTextDatum(TR_DATUM);
  tft.setTextColor(colorForHealth(status), TFT_BLACK);
  tft.drawString(status, tft.width() - 8, 6);
  tft.drawFastHLine(8, 30, tft.width() - 16, TFT_DARKGREY);
  tft.setTextDatum(TL_DATUM);
}

void drawFooter() {
  tft.setTextSize(1);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  String sync = lastOK == 0 ? "never" : String((millis() - lastOK) / 1000) + "s ago";
  tft.drawString("sync " + sync, 8, tft.height() - 14);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(screenIndex + 1) + "/" + String(SCREEN_COUNT), tft.width() - 8, tft.height() - 14);
  tft.setTextDatum(TL_DATUM);
}

void drawBoot(const String &message) {
  tft.fillScreen(TFT_BLACK);
  tft.setTextDatum(MC_DATUM);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.setTextSize(2);
  tft.drawString("PVE Desk", tft.width() / 2, 58);
  tft.setTextSize(1);
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(message, tft.width() / 2, 92);
  tft.setTextDatum(TL_DATUM);
}

void loadConfig() {
  prefs.begin("pve-desk", true);
  cfg.ssid = prefs.getString("ssid", "");
  cfg.password = prefs.getString("password", "");
  cfg.bridgeURL = prefs.getString("bridge", "");
  cfg.displayToken = prefs.getString("token", "");
  cfg.deviceName = prefs.getString("name", "desk-display");
  cfg.brightness = prefs.getUChar("bright", 220);
  prefs.end();
  cfg.configured = cfg.ssid.length() > 0 && cfg.bridgeURL.length() > 0 && cfg.displayToken.length() > 0;
}

void saveConfig() {
  prefs.begin("pve-desk", false);
  prefs.putString("ssid", cfg.ssid);
  prefs.putString("password", cfg.password);
  prefs.putString("bridge", trimTrailingSlash(cfg.bridgeURL));
  prefs.putString("token", cfg.displayToken);
  prefs.putString("name", cfg.deviceName.length() == 0 ? "desk-display" : cfg.deviceName);
  prefs.putUChar("bright", cfg.brightness);
  prefs.end();
}

void clearConfig() {
  prefs.begin("pve-desk", false);
  prefs.clear();
  prefs.end();
}

String setupPage() {
  String page;
  page.reserve(3600);
  page += "<!doctype html><html><head><meta name='viewport' content='width=device-width,initial-scale=1'>";
  page += "<title>PVE Desk Setup</title><style>";
  page += "body{font-family:system-ui;margin:24px;max-width:520px;color:#17202a}";
  page += "label{display:block;margin-top:14px;font-weight:600}input{box-sizing:border-box;width:100%;padding:10px;margin-top:6px}";
  page += "button{margin-top:18px;padding:12px 16px;background:#17202a;color:white;border:0;border-radius:6px}";
  page += ".hint{color:#566573;font-size:14px}</style></head><body>";
  page += "<h1>PVE Desk Setup</h1>";
  page += "<p class='hint'>Configure Wi-Fi and bridge connection. Proxmox tokens stay on the bridge, not on this device.</p>";
  page += "<form method='post' action='/save'>";
  page += "<label>Wi-Fi SSID<input name='ssid' value='" + htmlEscape(cfg.ssid) + "' required></label>";
  page += "<label>Wi-Fi Password<input name='password' type='password' value='" + htmlEscape(cfg.password) + "'></label>";
  page += "<label>Bridge URL<input name='bridge' placeholder='http://192.168.1.20:8765' value='" + htmlEscape(cfg.bridgeURL) + "' required></label>";
  page += "<label>Display Token<input name='token' type='password' value='" + htmlEscape(cfg.displayToken) + "' required></label>";
  page += "<label>Device Name<input name='name' value='" + htmlEscape(cfg.deviceName) + "'></label>";
  page += "<label>Brightness 0-255<input name='bright' type='number' min='0' max='255' value='" + String(cfg.brightness) + "'></label>";
  page += "<button type='submit'>Save and reboot</button></form>";
  page += "<form method='post' action='/reset'><button type='submit'>Reset saved config</button></form>";
  page += "</body></html>";
  return page;
}

void startConfigPortal() {
  drawBoot("setup Wi-Fi: " + String(AP_NAME));
  WiFi.mode(WIFI_AP);
  IPAddress apIP(192, 168, 4, 1);
  IPAddress gateway(192, 168, 4, 1);
  IPAddress subnet(255, 255, 255, 0);
  WiFi.softAPConfig(apIP, gateway, subnet);
  WiFi.softAP(AP_NAME);
  dnsServer.start(DNS_PORT, "*", apIP);

  webServer.on("/", HTTP_GET, []() {
    webServer.send(200, "text/html", setupPage());
  });
  webServer.on("/save", HTTP_POST, []() {
    cfg.ssid = webServer.arg("ssid");
    cfg.password = webServer.arg("password");
    cfg.bridgeURL = trimTrailingSlash(webServer.arg("bridge"));
    cfg.displayToken = webServer.arg("token");
    cfg.deviceName = webServer.arg("name");
    cfg.brightness = constrain(webServer.arg("bright").toInt(), 0, 255);
    saveConfig();
    webServer.send(200, "text/html", "<p>Saved. Rebooting...</p>");
    delay(800);
    ESP.restart();
  });
  webServer.on("/reset", HTTP_POST, []() {
    clearConfig();
    webServer.send(200, "text/html", "<p>Reset. Rebooting...</p>");
    delay(800);
    ESP.restart();
  });
  webServer.onNotFound([]() {
    webServer.sendHeader("Location", "/", true);
    webServer.send(302, "text/plain", "");
  });
  webServer.begin();

  while (true) {
    dnsServer.processNextRequest();
    webServer.handleClient();
    delay(10);
  }
}

bool connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(cfg.ssid.c_str(), cfg.password.c_str());
  drawBoot("connecting Wi-Fi");

  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    delay(250);
  }
  if (WiFi.status() != WL_CONNECTED) {
    lastError = "Wi-Fi connection failed";
    return false;
  }
  deviceIP = WiFi.localIP().toString();
  return true;
}

bool parseState(const String &payload) {
  DynamicJsonDocument doc(16384);
  DeserializationError err = deserializeJson(doc, payload);
  if (err) {
    lastError = "JSON parse: " + String(err.c_str());
    return false;
  }

  state = DisplayState();
  state.schema = doc["schema"] | "";
  state.generatedAt = doc["generated_at"] | "";
  state.stale = doc["stale"] | true;
  JsonObject summary = doc["summary"];
  state.summary.health = summary["health"] | "unknown";
  state.summary.hostsOnline = summary["hosts_online"] | 0;
  state.summary.hostsTotal = summary["hosts_total"] | 0;
  state.summary.guestsRunning = summary["guests_running"] | 0;
  state.summary.guestsStopped = summary["guests_stopped"] | 0;
  state.summary.alerts = summary["alerts"] | 0;

  for (JsonObject h : doc["hosts"].as<JsonArray>()) {
    if (state.hostCount >= MAX_HOSTS) break;
    Host &host = state.hosts[state.hostCount++];
    host.name = h["name"] | "";
    host.online = h["online"] | false;
    host.cpu = h["cpu_pct"] | 0;
    host.memory = h["memory_pct"] | 0;
    host.storage = h["storage_pct"] | 0;
    host.running = h["guests_running"] | 0;
    host.stopped = h["guests_stopped"] | 0;
    host.health = h["health"] | "unknown";
  }

  for (JsonObject g : doc["guests"].as<JsonArray>()) {
    if (state.guestCount >= MAX_GUESTS) break;
    Guest &guest = state.guests[state.guestCount++];
    guest.name = g["name"] | "";
    guest.type = g["type"] | "";
    guest.hostName = g["host_name"] | "";
    guest.status = g["status"] | "";
    guest.cpu = g["cpu_pct"] | 0;
    guest.maxCPU = g["max_cpu"] | 0;
    guest.memory = g["memory_pct"] | 0;
    guest.disk = g["disk_pct"] | 0;
    guest.memoryUsed = g["memory_used_bytes"] | 0;
    guest.memoryTotal = g["memory_total_bytes"] | 0;
    guest.diskUsed = g["disk_used_bytes"] | 0;
    guest.diskTotal = g["disk_total_bytes"] | 0;
    guest.uptime = g["uptime_sec"] | 0;
    guest.netIn = g["net_in_bytes"] | 0;
    guest.netOut = g["net_out_bytes"] | 0;
    guest.diskRead = g["disk_read_bytes"] | 0;
    guest.diskWrite = g["disk_write_bytes"] | 0;
    guest.pinned = g["pinned"] | false;
    guest.health = g["health"] | "unknown";
  }

  if (state.guestCount == 0) {
    selectedGuest = 0;
  } else if (selectedGuest >= state.guestCount) {
    selectedGuest = state.guestCount - 1;
  }

  for (JsonObject a : doc["alerts"].as<JsonArray>()) {
    if (state.alertCount >= MAX_ALERTS) break;
    Alert &alert = state.alerts[state.alertCount++];
    alert.severity = a["severity"] | "unknown";
    alert.title = a["title"] | "";
    alert.message = a["message"] | "";
  }

  lastError = "";
  lastOK = millis();
  return true;
}

bool fetchState() {
  if (WiFi.status() != WL_CONNECTED) {
    lastError = "Wi-Fi disconnected";
    return false;
  }

  HTTPClient http;
  String url = trimTrailingSlash(cfg.bridgeURL) + "/api/v1/display-state";
  http.setTimeout(5000);
  if (!http.begin(url)) {
    lastError = "bad bridge URL";
    return false;
  }
  http.addHeader("Authorization", "Bearer " + cfg.displayToken);
  int code = http.GET();
  if (code != 200) {
    lastError = "bridge HTTP " + String(code);
    http.end();
    return false;
  }
  String payload = http.getString();
  http.end();
  return parseState(payload);
}

void drawBar(int x, int y, int w, int h, int pct, uint16_t color) {
  tft.drawRect(x, y, w, h, TFT_DARKGREY);
  int fill = map(constrain(pct, 0, 100), 0, 100, 0, w - 2);
  tft.fillRect(x + 1, y + 1, fill, h - 2, color);
}

void drawOverview() {
  drawHeader("PROXMOX", state.summary.health);
  tft.setTextSize(1);
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(String(state.summary.hostsOnline) + "/" + String(state.summary.hostsTotal) + " hosts", 10, 38);
  tft.drawString(String(state.summary.guestsRunning) + " run  " + String(state.summary.guestsStopped) + " stop", 110, 38);
  if (state.stale) {
    tft.setTextColor(TFT_YELLOW, TFT_BLACK);
    tft.drawString("STALE", 250, 38);
  }

  int y = 58;
  for (size_t i = 0; i < state.hostCount && i < 2; ++i) {
    Host &h = state.hosts[i];
    tft.setTextColor(h.online ? TFT_WHITE : TFT_RED, TFT_BLACK);
    tft.setTextSize(1);
    tft.drawString(h.name.substring(0, 15), 10, y);
    tft.setTextColor(colorForHealth(h.health), TFT_BLACK);
    tft.drawString(h.online ? "online" : "offline", 230, y);
    tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
    tft.drawString("CPU " + String(h.cpu) + "%", 10, y + 16);
    drawBar(62, y + 16, 50, 8, h.cpu, TFT_CYAN);
    tft.drawString("RAM " + String(h.memory) + "%", 124, y + 16);
    drawBar(178, y + 16, 50, 8, h.memory, TFT_GREEN);
    tft.drawString("STOR " + String(h.storage) + "%", 10, y + 30);
    drawBar(72, y + 30, 50, 8, h.storage, colorForHealth(h.health));
    tft.drawString(String(h.running) + " run", 138, y + 30);
    y += 48;
  }
  drawFooter();
}

void drawGuests() {
  drawHeader("GUESTS", state.summary.health);
  tft.setTextSize(1);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("B selects  A screen", 10, 38);
  tft.drawString("CPU RAM DSK", 214, 38);
  int y = 52;
  if (state.guestCount == 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No guests in display state", 10, y);
  }
  for (size_t i = 0; i < state.guestCount && y < tft.height() - 18; ++i) {
    Guest &g = state.guests[i];
    uint16_t rowBg = i == selectedGuest ? TFT_DARKGREY : TFT_BLACK;
    if (i == selectedGuest) tft.fillRect(6, y - 1, tft.width() - 12, 13, rowBg);
    tft.setTextColor(colorForHealth(g.health), rowBg);
    tft.drawString(g.status == "running" ? "RUN" : "STOP", 10, y);
    tft.setTextColor(TFT_WHITE, rowBg);
    String label = g.name;
    if (label.length() > 18) label = label.substring(0, 18);
    tft.drawString(label, 43, y);
    tft.setTextColor(TFT_LIGHTGREY, rowBg);
    tft.drawString(String(g.cpu), 216, y);
    tft.drawString(String(g.memory), 250, y);
    tft.drawString(String(g.disk), 286, y);
    y += 14;
  }
  drawFooter();
}

void drawGuestDetail() {
  drawHeader("DETAIL", state.summary.health);
  tft.setTextSize(1);
  if (state.guestCount == 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No guest selected", 10, 42);
    drawFooter();
    return;
  }

  Guest &g = state.guests[selectedGuest];
  tft.setTextColor(colorForHealth(g.health), TFT_BLACK);
  tft.drawString(g.status == "running" ? "RUNNING" : g.status, 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String title = g.name;
  if (title.length() > 27) title = title.substring(0, 27);
  tft.drawString(title, 86, 38);

  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  String where = g.type + "  " + g.hostName;
  if (where.length() > 42) where = where.substring(0, 42);
  tft.drawString(where, 10, 54);

  int y = 74;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("CPU", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(g.cpu) + "% / " + String(g.maxCPU) + " cores", 92, y);
  drawBar(205, y, 85, 8, g.cpu, TFT_CYAN);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("RAM", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.memoryUsed) + " / " + formatBytes(g.memoryTotal), 92, y);
  drawBar(205, y, 85, 8, g.memory, TFT_GREEN);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("DISK", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.diskUsed) + " / " + formatBytes(g.diskTotal), 92, y);
  drawBar(205, y, 85, 8, g.disk, TFT_YELLOW);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("UP", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatUptime(g.uptime), 92, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("NET", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.netIn) + " in  " + formatBytes(g.netOut) + " out", 92, y);

  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedGuest + 1) + "/" + String(state.guestCount) + "  B next", tft.width() - 8, 54);
  tft.setTextDatum(TL_DATUM);
  drawFooter();
}

void drawAlerts() {
  drawHeader("ALERTS", state.summary.health);
  tft.setTextSize(1);
  int y = 40;
  if (state.alertCount == 0) {
    tft.setTextColor(TFT_GREEN, TFT_BLACK);
    tft.setTextSize(2);
    tft.drawString("NO ALERTS", 10, y);
    tft.setTextSize(1);
    tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
    tft.drawString("All configured checks are OK", 10, y + 28);
  }
  for (size_t i = 0; i < state.alertCount && y < tft.height() - 26; ++i) {
    Alert &a = state.alerts[i];
    tft.setTextColor(colorForHealth(a.severity), TFT_BLACK);
    tft.drawString(a.severity.substring(0, 4), 10, y);
    tft.setTextColor(TFT_WHITE, TFT_BLACK);
    String title = a.title;
    if (title.length() > 31) title = title.substring(0, 31);
    tft.drawString(title, 54, y);
    y += 16;
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    String msg = a.message;
    if (msg.length() > 36) msg = msg.substring(0, 36);
    tft.drawString(msg, 54, y);
    y += 18;
  }
  drawFooter();
}

void drawDevice() {
  drawHeader("DEVICE", lastError.length() == 0 ? "ok" : "warning");
  tft.setTextSize(1);
  int y = 42;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("Name", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(cfg.deviceName, 90, y);
  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("IP", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(deviceIP, 90, y);
  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("RSSI", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(WiFi.RSSI()) + " dBm", 90, y);
  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("Bridge", 10, y);
  tft.setTextColor(lastError.length() == 0 ? TFT_GREEN : TFT_RED, TFT_BLACK);
  tft.drawString(lastError.length() == 0 ? "OK" : lastError.substring(0, 28), 90, y);
  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("FW", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(FW_VERSION, 90, y);
  drawFooter();
}

void render() {
  switch (screenIndex) {
    case 0:
      drawOverview();
      break;
    case 1:
      drawGuests();
      break;
    case 2:
      drawGuestDetail();
      break;
    case 3:
      drawAlerts();
      break;
    default:
      drawDevice();
      break;
  }
}

void toggleBrightness() {
  if (cfg.brightness > 180) cfg.brightness = 60;
  else if (cfg.brightness > 80) cfg.brightness = 220;
  else cfg.brightness = 140;
  setBacklight(cfg.brightness);
  saveConfig();
}

void pollButton(uint8_t pin, ButtonState &button, void (*shortPress)(), void (*longPress)()) {
  bool pressed = digitalRead(pin) == LOW;
  unsigned long now = millis();
  if (pressed && !button.previous) {
    button.pressedAt = now;
    button.longHandled = false;
  }
  if (pressed && !button.longHandled && now - button.pressedAt > BUTTON_LONG_MS) {
    button.longHandled = true;
    longPress();
  }
  if (!pressed && button.previous && !button.longHandled) {
    shortPress();
  }
  button.previous = pressed;
}

void nextScreen() {
  screenIndex = (screenIndex + 1) % SCREEN_COUNT;
  render();
}

void prevScreen() {
  screenIndex = screenIndex == 0 ? SCREEN_COUNT - 1 : screenIndex - 1;
  render();
}

void nextGuest() {
  if (state.guestCount == 0) return;
  selectedGuest = (selectedGuest + 1) % state.guestCount;
  render();
}

void manualRefresh() {
  fetchState();
  render();
}

void buttonBShort() {
  if (screenIndex == 1 || screenIndex == 2) {
    nextGuest();
    return;
  }
  manualRefresh();
}

void factoryReset() {
  drawBoot("resetting config");
  clearConfig();
  delay(800);
  ESP.restart();
}

void handleButtons() {
  if (digitalRead(BTN_A) == LOW && digitalRead(BTN_B) == LOW) {
    unsigned long start = millis();
    while (digitalRead(BTN_A) == LOW && digitalRead(BTN_B) == LOW) {
      if (millis() - start > BUTTON_LONG_MS) {
        factoryReset();
      }
      delay(10);
    }
  }
  pollButton(BTN_A, buttonA, nextScreen, prevScreen);
  pollButton(BTN_B, buttonB, buttonBShort, toggleBrightness);
}

}  // namespace

void setup() {
  Serial.begin(115200);
  pinMode(BTN_A, INPUT_PULLUP);
  pinMode(BTN_B, INPUT_PULLUP);

  tft.init();
  tft.setRotation(1);
  setBacklight(220);
  drawBoot("booting");

  loadConfig();
  setBacklight(cfg.brightness);

  if (!cfg.configured) {
    startConfigPortal();
  }
  if (!connectWiFi()) {
    startConfigPortal();
  }
  fetchState();
  render();
}

void loop() {
  handleButtons();
  if (millis() - lastPoll > POLL_MS) {
    lastPoll = millis();
    fetchState();
    render();
  }
  delay(20);
}
