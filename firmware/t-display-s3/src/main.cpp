#include <Arduino.h>
#include <ArduinoJson.h>
#include <DNSServer.h>
#include <HTTPClient.h>
#include <Preferences.h>
#include <TFT_eSPI.h>
#include <WebServer.h>
#include <WiFi.h>

SET_LOOP_TASK_STACK_SIZE(32768);

namespace {

constexpr const char *FW_VERSION = "0.1.0-dev";
constexpr const char *AP_NAME = "PVE-Desk-Setup";
constexpr uint8_t DNS_PORT = 53;
constexpr uint8_t BTN_A = 0;   // BOOT
constexpr uint8_t BTN_B = 14;  // LILYGO user button
constexpr uint8_t BACKLIGHT_PIN = 38;
constexpr unsigned long POLL_MS = 10000;
constexpr unsigned long BUTTON_LONG_MS = 1200;
constexpr uint8_t SCREEN_COUNT = 9;
constexpr size_t MAX_HOSTS = 12;
constexpr size_t MAX_STORAGES = 24;
constexpr size_t MAX_GUESTS = 24;
constexpr size_t MAX_ALERTS = 12;
constexpr size_t JSON_DOC_CAPACITY = 32768;
constexpr int LIST_ROW_H = 14;

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
  int maxCPU = 0;
  String cpuModel;
  int gpuCount = 0;
  String gpuSummary;
  int memory = 0;
  int64_t memoryUsed = 0;
  int64_t memoryTotal = 0;
  int storage = 0;
  int64_t storageUsed = 0;
  int64_t storageTotal = 0;
  int64_t uptime = 0;
  String load1;
  String pveVersion;
  String kernelVersion;
  int running = 0;
  int stopped = 0;
  String health = "unknown";
};

struct Storage {
  String name;
  String hostName;
  String status;
  String pluginType;
  String content;
  bool shared = false;
  int disk = 0;
  int64_t diskUsed = 0;
  int64_t diskTotal = 0;
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
  Storage storages[MAX_STORAGES];
  size_t storageCount = 0;
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
size_t selectedHost = 0;
size_t selectedStorage = 0;
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

String labelForHealth(const String &health) {
  if (health == "ok") return "OK";
  if (health == "warning") return "WARN";
  if (health == "critical") return "CRIT";
  if (health == "stale") return "STALE";
  return health.length() ? "UNK" : "";
}

uint16_t textColorForFill(uint16_t fill) {
  if (fill == TFT_RED || fill == TFT_DARKGREY || fill == TFT_BLUE) return TFT_WHITE;
  return TFT_BLACK;
}

String clipText(String value, size_t maxChars) {
  value.trim();
  if (maxChars == 0 || value.length() <= maxChars) return value;
  if (maxChars <= 1) return value.substring(0, maxChars);
  return value.substring(0, maxChars - 1) + ".";
}

size_t visibleWindowStart(size_t selected, size_t count, size_t visibleRows) {
  if (count <= visibleRows || visibleRows == 0) return 0;
  size_t half = visibleRows / 2;
  if (selected <= half) return 0;
  if (selected + (visibleRows - half) >= count) return count - visibleRows;
  return selected - half;
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

void drawChip(const String &text, uint16_t fill, int right, int y) {
  if (text.length() == 0) return;
  int w = text.length() * 6 + 10;
  int x = right - w;
  tft.fillRect(x, y, w, 15, fill);
  tft.setTextSize(1);
  tft.setTextDatum(TR_DATUM);
  tft.setTextColor(textColorForFill(fill), fill);
  tft.drawString(text, right - 5, y + 4);
  tft.setTextDatum(TL_DATUM);
}

void drawHeader(const String &title, const String &status) {
  tft.fillScreen(TFT_BLACK);
  tft.setTextDatum(TL_DATUM);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.setTextSize(2);
  tft.drawString(title, 8, 6);
  drawChip(labelForHealth(status), colorForHealth(status), tft.width() - 8, 8);
  tft.drawFastHLine(8, 30, tft.width() - 16, TFT_DARKGREY);
  tft.setTextDatum(TL_DATUM);
}

void drawFooter() {
  tft.setTextSize(1);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  String sync = lastOK == 0 ? "never" : String((millis() - lastOK) / 1000) + "s ago";
  tft.drawString("sync " + sync, 8, tft.height() - 14);
  if (state.summary.alerts > 0) {
    tft.setTextDatum(MC_DATUM);
    tft.setTextColor(colorForHealth(state.summary.health), TFT_BLACK);
    tft.drawString("!" + String(state.summary.alerts), tft.width() / 2, tft.height() - 10);
  }
  tft.setTextDatum(TR_DATUM);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
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
  Serial.printf("config: configured=%d ssid=%s bridge=%s\n", cfg.configured, cfg.ssid.c_str(), cfg.bridgeURL.c_str());
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
  Serial.println("setup portal: starting");
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
  Serial.printf("wifi: connecting to %s\n", cfg.ssid.c_str());
  WiFi.mode(WIFI_STA);
  WiFi.begin(cfg.ssid.c_str(), cfg.password.c_str());
  drawBoot("connecting Wi-Fi");

  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    delay(250);
  }
  if (WiFi.status() != WL_CONNECTED) {
    lastError = "Wi-Fi connection failed";
    Serial.printf("wifi: failed status=%d\n", WiFi.status());
    return false;
  }
  deviceIP = WiFi.localIP().toString();
  Serial.printf("wifi: connected ip=%s rssi=%d\n", deviceIP.c_str(), WiFi.RSSI());
  return true;
}

bool parseState(const String &payload) {
  DynamicJsonDocument doc(JSON_DOC_CAPACITY);
  DeserializationError err = deserializeJson(doc, payload);
  if (err) {
    lastError = "JSON parse: " + String(err.c_str());
    Serial.printf("json: parse failed: %s\n", err.c_str());
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
    host.maxCPU = h["max_cpu"] | 0;
    host.cpuModel = h["cpu_model"] | "";
    host.gpuCount = h["gpu_count"] | 0;
    host.gpuSummary = h["gpu_summary"] | "";
    host.memory = h["memory_pct"] | 0;
    host.memoryUsed = h["memory_used_bytes"] | 0;
    host.memoryTotal = h["memory_total_bytes"] | 0;
    host.storage = h["storage_pct"] | 0;
    host.storageUsed = h["storage_used_bytes"] | 0;
    host.storageTotal = h["storage_total_bytes"] | 0;
    host.uptime = h["uptime_sec"] | 0;
    JsonArray load = h["load_avg"].as<JsonArray>();
    if (!load.isNull() && load.size() > 0) host.load1 = load[0].as<String>();
    host.pveVersion = h["pve_version"] | "";
    host.kernelVersion = h["kernel_version"] | "";
    host.running = h["guests_running"] | 0;
    host.stopped = h["guests_stopped"] | 0;
    host.health = h["health"] | "unknown";
  }

  if (state.hostCount == 0) {
    selectedHost = 0;
  } else if (selectedHost >= state.hostCount) {
    selectedHost = state.hostCount - 1;
  }

  for (JsonObject s : doc["storages"].as<JsonArray>()) {
    if (state.storageCount >= MAX_STORAGES) break;
    Storage &storage = state.storages[state.storageCount++];
    storage.name = s["name"] | "";
    storage.hostName = s["host_name"] | "";
    storage.status = s["status"] | "";
    storage.pluginType = s["plugin_type"] | "";
    storage.content = s["content"] | "";
    storage.shared = s["shared"] | false;
    storage.disk = s["disk_pct"] | 0;
    storage.diskUsed = s["disk_used_bytes"] | 0;
    storage.diskTotal = s["disk_total_bytes"] | 0;
    storage.health = s["health"] | "unknown";
  }

  if (state.storageCount == 0) {
    selectedStorage = 0;
  } else if (selectedStorage >= state.storageCount) {
    selectedStorage = state.storageCount - 1;
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
  Serial.printf("json: hosts=%d storages=%d guests=%d alerts=%d\n", state.hostCount, state.storageCount, state.guestCount,
                state.alertCount);
  return true;
}

bool fetchState() {
  if (WiFi.status() != WL_CONNECTED) {
    lastError = "Wi-Fi disconnected";
    Serial.println("bridge: skipped, Wi-Fi disconnected");
    return false;
  }

  HTTPClient http;
  String url = trimTrailingSlash(cfg.bridgeURL) + "/api/v1/display-state";
  http.setTimeout(5000);
  if (!http.begin(url)) {
    lastError = "bad bridge URL";
    Serial.println("bridge: bad URL");
    return false;
  }
  http.addHeader("Authorization", "Bearer " + cfg.displayToken);
  int code = http.GET();
  if (code != 200) {
    lastError = "bridge HTTP " + String(code);
    Serial.printf("bridge: HTTP %d\n", code);
    http.end();
    return false;
  }
  String payload = http.getString();
  Serial.printf("bridge: payload=%d bytes\n", payload.length());
  http.end();
  return parseState(payload);
}

void drawBar(int x, int y, int w, int h, int pct, uint16_t color) {
  tft.drawRect(x, y, w, h, TFT_DARKGREY);
  int fill = map(constrain(pct, 0, 100), 0, 100, 0, w - 2);
  tft.fillRect(x + 1, y + 1, fill, h - 2, color);
}

void drawMetricRow(const String &label, const String &value, int pct, uint16_t color, int y) {
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(label, 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(value, 72, y);
  drawBar(205, y, 85, 8, pct, color);
}

int drawWrappedField(const String &label, String value, int y, size_t maxLines) {
  if (value.length() == 0) value = "-";
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(label, 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  const size_t maxChars = 40;
  for (size_t line = 0; line < maxLines; ++line) {
    if (value.length() == 0) break;
    String part = clipText(value, maxChars);
    if (value.length() > maxChars && line + 1 < maxLines) {
      part = value.substring(0, maxChars);
      value = value.substring(maxChars);
      value.trim();
    } else {
      value = "";
    }
    tft.drawString(part, 72, y + static_cast<int>(line) * 12);
  }
  return y + static_cast<int>(maxLines) * 12 + 4;
}

void drawOverview() {
  drawHeader("PROXMOX", state.summary.health);
  tft.setTextSize(1);
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(String(state.summary.hostsOnline) + "/" + String(state.summary.hostsTotal) + " hosts", 10, 38);
  tft.drawString(String(state.summary.guestsRunning) + " run  " + String(state.summary.guestsStopped) + " stop", 108, 38);
  if (state.stale) {
    tft.setTextColor(TFT_YELLOW, TFT_BLACK);
    tft.drawString("STALE", 250, 38);
  }

  if (state.hostCount == 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No hosts in display state", 10, 62);
    drawFooter();
    return;
  }

  int onlineHosts = 0;
  int cpuSum = 0;
  int cpuMax = 0;
  int memorySum = 0;
  int memoryMax = 0;
  for (size_t i = 0; i < state.hostCount; ++i) {
    Host &h = state.hosts[i];
    if (!h.online) continue;
    onlineHosts++;
    cpuSum += h.cpu;
    memorySum += h.memory;
    cpuMax = max(cpuMax, h.cpu);
    memoryMax = max(memoryMax, h.memory);
  }

  int diskSum = 0;
  int diskMax = 0;
  int diskCount = 0;
  for (size_t i = 0; i < state.storageCount; ++i) {
    Storage &s = state.storages[i];
    if (s.diskTotal <= 0) continue;
    diskCount++;
    diskSum += s.disk;
    diskMax = max(diskMax, s.disk);
  }
  if (diskCount == 0) {
    for (size_t i = 0; i < state.hostCount; ++i) {
      Host &h = state.hosts[i];
      if (h.storageTotal <= 0) continue;
      diskCount++;
      diskSum += h.storage;
      diskMax = max(diskMax, h.storage);
    }
  }

  int cpuAvg = onlineHosts == 0 ? 0 : cpuSum / onlineHosts;
  int memoryAvg = onlineHosts == 0 ? 0 : memorySum / onlineHosts;
  int diskAvg = diskCount == 0 ? 0 : diskSum / diskCount;

  int y = 58;
  drawMetricRow("CPU", "avg " + String(cpuAvg) + "%  max " + String(cpuMax) + "%", cpuAvg, TFT_CYAN, y);
  y += 20;
  drawMetricRow("RAM", "avg " + String(memoryAvg) + "%  max " + String(memoryMax) + "%", memoryAvg,
                memoryMax >= 90 ? TFT_RED : TFT_GREEN, y);
  y += 20;
  drawMetricRow("DSK", "avg " + String(diskAvg) + "%  max " + String(diskMax) + "%", diskAvg,
                diskMax >= 90 ? TFT_RED : TFT_YELLOW, y);

  y += 28;
  tft.setTextColor(state.summary.alerts > 0 ? colorForHealth(state.summary.health) : TFT_GREEN, TFT_BLACK);
  tft.drawString(state.summary.alerts > 0 ? "TOP ALERT" : "STATUS", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  if (state.summary.alerts > 0 && state.alertCount > 0) {
    tft.drawString(clipText(state.alerts[0].title, 36), 10, y + 14);
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(String(state.summary.alerts) + " active alerts", 10, y + 28);
  } else {
    tft.drawString("All configured checks are OK", 10, y + 14);
  }

  drawFooter();
}

void drawHosts() {
  drawHeader("HOSTS", "");
  tft.setTextSize(1);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("HOST", 40, 38);
  tft.drawString("C R D", 218, 38);
  int y = 52;
  if (state.hostCount == 0) {
    tft.drawString("No hosts in display state", 10, y);
    drawFooter();
    return;
  }

  size_t visibleRows = static_cast<size_t>((tft.height() - 18 - y) / LIST_ROW_H);
  if (visibleRows == 0) visibleRows = 1;
  size_t start = visibleWindowStart(selectedHost, state.hostCount, visibleRows);
  size_t end = start + visibleRows;
  if (end > state.hostCount) end = state.hostCount;
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(static_cast<int>(start + 1)) + "-" + String(static_cast<int>(end)) + "/" +
                     String(static_cast<int>(state.hostCount)),
                 tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  for (size_t i = start; i < end && y < tft.height() - 18; ++i) {
    Host &h = state.hosts[i];
    uint16_t rowBg = i == selectedHost ? TFT_DARKGREY : TFT_BLACK;
    if (i == selectedHost) tft.fillRect(6, y - 1, tft.width() - 12, 13, rowBg);
    tft.setTextColor(colorForHealth(h.health), rowBg);
    tft.drawString(h.online ? "ON" : "OFF", 10, y);
    tft.setTextColor(TFT_WHITE, rowBg);
    String label = h.name;
    if (label.length() > 18) label = label.substring(0, 18);
    tft.drawString(label, 40, y);
    tft.setTextColor(TFT_LIGHTGREY, rowBg);
    tft.drawString(String(h.cpu), 210, y);
    tft.drawString(String(h.memory), 246, y);
    tft.drawString(String(h.storage), 286, y);
    y += 14;
  }
  drawFooter();
}

void drawHostDetail() {
  tft.setTextSize(1);
  if (state.hostCount == 0) {
    drawHeader("HOST", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No host selected", 10, 42);
    drawFooter();
    return;
  }

  Host &h = state.hosts[selectedHost];
  drawHeader("HOST", h.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(h.health), TFT_BLACK);
  tft.drawString(h.online ? "ONLINE" : "OFFLINE", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String title = h.name;
  if (title.length() > 22) title = title.substring(0, 22);
  tft.drawString(title, 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedHost + 1) + "/" + String(state.hostCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 58;
  drawMetricRow("CPU", String(h.cpu) + "% / " + String(h.maxCPU) + " cores", h.cpu, TFT_CYAN, y);

  y += 18;
  drawMetricRow("RAM", formatBytes(h.memoryUsed) + " / " + formatBytes(h.memoryTotal), h.memory,
                colorForHealth(h.health), y);

  y += 18;
  drawMetricRow("ROOT", formatBytes(h.storageUsed) + " / " + formatBytes(h.storageTotal), h.storage, TFT_YELLOW, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("LOAD", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(h.load1.length() ? h.load1 : "-", 72, y);
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("UP", 160, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatUptime(h.uptime), 190, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("GUESTS", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(h.running) + " run  " + String(h.stopped) + " stop", 72, y);

  drawFooter();
}

void drawHostSystem() {
  tft.setTextSize(1);
  if (state.hostCount == 0) {
    drawHeader("SYSTEM", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No host selected", 10, 42);
    drawFooter();
    return;
  }

  Host &h = state.hosts[selectedHost];
  drawHeader("SYSTEM", h.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(h.health), TFT_BLACK);
  tft.drawString(h.online ? "ONLINE" : "OFFLINE", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(h.name, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedHost + 1) + "/" + String(state.hostCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("CPU", h.cpuModel, y, 2);
  String gpu = h.gpuSummary;
  if (h.gpuCount > 1 && gpu.indexOf("+") < 0) {
    gpu += " +" + String(h.gpuCount - 1);
  }
  y = drawWrappedField("GPU", gpu, y, 2);
  y = drawWrappedField("PVE", h.pveVersion, y, 1);
  drawWrappedField("KERN", h.kernelVersion, y, 1);

  drawFooter();
}

void drawStorage() {
  tft.setTextSize(1);
  if (state.storageCount == 0) {
    drawHeader("STORAGE", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No storage data", 10, 42);
    drawFooter();
    return;
  }

  Storage &s = state.storages[selectedStorage];
  drawHeader("STORAGE", s.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(s.health), TFT_BLACK);
  tft.drawString(s.status.length() ? s.status : "unknown", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String title = s.name;
  if (title.length() > 22) title = title.substring(0, 22);
  tft.drawString(title, 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedStorage + 1) + "/" + String(state.storageCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  String where = s.pluginType + "  " + s.hostName;
  if (where.length() > 42) where = where.substring(0, 42);
  tft.drawString(where, 10, 54);

  int y = 76;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("USED", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(s.diskUsed) + " / " + formatBytes(s.diskTotal), 92, y);
  drawBar(205, y, 85, 8, s.disk, colorForHealth(s.health));

  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("PCT", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(s.disk) + "%", 92, y);

  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("SHARED", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(s.shared ? "yes" : "no", 92, y);

  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("CONTENT", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String content = s.content;
  if (content.length() > 33) content = content.substring(0, 33);
  tft.drawString(content.length() ? content : "-", 92, y);

  drawFooter();
}

void drawGuests() {
  drawHeader("GUESTS", "");
  tft.setTextSize(1);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("VM/LXC", 43, 38);
  tft.drawString("C R D", 218, 38);
  int y = 52;
  if (state.guestCount == 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No guests in display state", 10, y);
    drawFooter();
    return;
  }

  size_t visibleRows = static_cast<size_t>((tft.height() - 18 - y) / LIST_ROW_H);
  if (visibleRows == 0) visibleRows = 1;
  size_t start = visibleWindowStart(selectedGuest, state.guestCount, visibleRows);
  size_t end = start + visibleRows;
  if (end > state.guestCount) end = state.guestCount;
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(static_cast<int>(start + 1)) + "-" + String(static_cast<int>(end)) + "/" +
                     String(static_cast<int>(state.guestCount)),
                 tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  for (size_t i = start; i < end && y < tft.height() - 18; ++i) {
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
  tft.setTextSize(1);
  if (state.guestCount == 0) {
    drawHeader("DETAIL", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No guest selected", 10, 42);
    drawFooter();
    return;
  }

  Guest &g = state.guests[selectedGuest];
  drawHeader("DETAIL", g.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(g.health), TFT_BLACK);
  tft.drawString(g.status == "running" ? "RUNNING" : g.status, 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String title = g.name;
  if (title.length() > 22) title = title.substring(0, 22);
  tft.drawString(title, 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedGuest + 1) + "/" + String(state.guestCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

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
      drawHosts();
      break;
    case 2:
      drawHostDetail();
      break;
    case 3:
      drawHostSystem();
      break;
    case 4:
      drawStorage();
      break;
    case 5:
      drawGuests();
      break;
    case 6:
      drawGuestDetail();
      break;
    case 7:
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

void nextHost() {
  if (state.hostCount == 0) return;
  selectedHost = (selectedHost + 1) % state.hostCount;
  render();
}

void nextStorage() {
  if (state.storageCount == 0) return;
  selectedStorage = (selectedStorage + 1) % state.storageCount;
  render();
}

void manualRefresh() {
  fetchState();
  render();
}

void buttonBShort() {
  if (screenIndex == 1 || screenIndex == 2 || screenIndex == 3) {
    nextHost();
    return;
  }
  if (screenIndex == 4) {
    nextStorage();
    return;
  }
  if (screenIndex == 5 || screenIndex == 6) {
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
