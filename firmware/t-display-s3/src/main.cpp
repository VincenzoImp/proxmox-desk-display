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
constexpr uint8_t SCREEN_COUNT = 19;
constexpr size_t MAX_HOSTS = 12;
constexpr size_t MAX_STORAGES = 24;
constexpr size_t MAX_DISKS = 24;
constexpr size_t MAX_GUESTS = 24;
constexpr size_t MAX_TASKS = 24;
constexpr size_t MAX_ALERTS = 12;
constexpr size_t MAX_ZFS_POOLS = 16;
constexpr size_t MAX_STORAGE_ITEMS = 48;
constexpr size_t MAX_TRENDS = 64;
constexpr size_t MAX_TREND_VALUES = 24;
constexpr size_t MAX_CERTIFICATES = 24;
constexpr size_t MAX_CAPABILITIES = 48;
constexpr size_t MAX_CEPH_CLUSTERS = 8;
constexpr size_t JSON_DOC_CAPACITY = 81920;
constexpr size_t DETAIL_JSON_DOC_CAPACITY = 65536;
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
  int64_t memoryAvailable = 0;
  int swap = 0;
  int64_t swapUsed = 0;
  int64_t swapTotal = 0;
  int ioWait = 0;
  int64_t ksmShared = 0;
  int storage = 0;
  int64_t storageUsed = 0;
  int64_t storageTotal = 0;
  int storageMax = 0;
  String storageMaxName;
  int64_t uptime = 0;
  String loadAvg;
  String pveVersion;
  String kernelVersion;
  String primaryAddress;
  int networkActive = 0;
  int networkTotal = 0;
  int servicesRunning = 0;
  int servicesFailed = 0;
  int servicesTotal = 0;
  int diskCount = 0;
  int diskIssues = 0;
  int failedTasks24h = 0;
  String lastBackupStatus;
  int64_t lastBackupAge = 0;
  String dataWarnings;
  int running = 0;
  int stopped = 0;
  String health = "unknown";
  String error;
};

struct Storage {
  String name;
  String hostName;
  String status;
  String pluginType;
  String content;
  String path;
  String pool;
  String mountpoint;
  bool shared = false;
  int disk = 0;
  int64_t diskUsed = 0;
  int64_t diskTotal = 0;
  int contentItems = 0;
  int backupCount = 0;
  int isoCount = 0;
  int templateCount = 0;
  int imageCount = 0;
  int rootdirCount = 0;
  String health = "unknown";
};

struct Disk {
  String name;
  String hostName;
  String model;
  String serial;
  String type;
  String usedBy;
  int64_t size = 0;
  String smartHealth;
  int wearout = 0;
  String health = "unknown";
};

struct Guest {
  String vmid;
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
  int64_t memoryHost = 0;
  int swap = 0;
  int64_t swapUsed = 0;
  int64_t swapTotal = 0;
  int64_t diskUsed = 0;
  int64_t diskTotal = 0;
  int64_t uptime = 0;
  int64_t netIn = 0;
  int64_t netOut = 0;
  int64_t diskRead = 0;
  int64_t diskWrite = 0;
  String tags;
  String osType;
  String ipAddress;
  bool agentEnabled = false;
  bool agentAvailable = false;
  String agentVersion;
  int agentCommandCount = 0;
  String qmpStatus;
  String runningQemu;
  bool haManaged = false;
  int pressureCPU = 0;
  int pressureIO = 0;
  int pressureMemory = 0;
  bool onBoot = false;
  bool protection = false;
  bool isTemplate = false;
  bool unprivileged = false;
  String configWarning;
  String expected;
  bool pinned = false;
  String health = "unknown";
};

struct Task {
  String type;
  String user;
  String status;
  String target;
  String vmid;
  String guestName;
  String hostName;
  int64_t startedAt = 0;
  int64_t startedAge = 0;
  int64_t endedAt = 0;
  int64_t duration = 0;
  String health = "unknown";
};

struct Alert {
  String severity;
  String title;
  String message;
};

struct ZFSPool {
  String name;
  String hostName;
  String status;
  String state;
  String scan;
  String errors;
  int64_t size = 0;
  int64_t allocated = 0;
  int64_t free = 0;
  int fragmentation = 0;
  int deviceCount = 0;
  int issueCount = 0;
  String health = "unknown";
};

struct StorageItem {
  String storage;
  String hostName;
  String content;
  String volid;
  String vmid;
  String format;
  int64_t size = 0;
  int64_t createdAt = 0;
  bool protectedItem = false;
  String verificationState;
  String health = "unknown";
};

struct MetricTrend {
  String resourceType;
  String resourceName;
  String metric;
  String unit;
  int last = 0;
  int values[MAX_TREND_VALUES];
  size_t valueCount = 0;
};

struct Certificate {
  String filename;
  String hostName;
  String subject;
  String issuer;
  int daysRemaining = 0;
  String health = "unknown";
};

struct Capability {
  String sourceID;
  String name;
  String status;
  int httpStatus = 0;
  String message;
};

struct CephCluster {
  String sourceID;
  String healthText;
  int64_t total = 0;
  int64_t used = 0;
  int64_t available = 0;
  int usage = 0;
  int osds = 0;
  int osdsUp = 0;
  int osdsIn = 0;
  int pgs = 0;
  String health = "unknown";
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
  Disk disks[MAX_DISKS];
  size_t diskCount = 0;
  Guest guests[MAX_GUESTS];
  size_t guestCount = 0;
  Task tasks[MAX_TASKS];
  size_t taskCount = 0;
  Alert alerts[MAX_ALERTS];
  size_t alertCount = 0;
};

struct InventoryDetails {
  String schema;
  String generatedAt;
  bool stale = true;
  ZFSPool zfsPools[MAX_ZFS_POOLS];
  size_t zfsPoolCount = 0;
  StorageItem storageItems[MAX_STORAGE_ITEMS];
  size_t storageItemCount = 0;
  MetricTrend trends[MAX_TRENDS];
  size_t trendCount = 0;
  Certificate certificates[MAX_CERTIFICATES];
  size_t certificateCount = 0;
  Capability capabilities[MAX_CAPABILITIES];
  size_t capabilityCount = 0;
  CephCluster cephClusters[MAX_CEPH_CLUSTERS];
  size_t cephClusterCount = 0;
};

DeviceConfig cfg;
DisplayState state;
InventoryDetails details;
String lastError;
String lastDetailError;
String deviceIP;
unsigned long lastPoll = 0;
unsigned long lastOK = 0;
unsigned long lastDetailOK = 0;
uint8_t screenIndex = 0;
size_t selectedHost = 0;
size_t selectedStorage = 0;
size_t selectedDisk = 0;
size_t selectedGuest = 0;
size_t selectedTask = 0;
size_t selectedAlert = 0;
size_t selectedZFSPool = 0;
size_t selectedStorageItem = 0;
size_t selectedTrend = 0;
size_t selectedCertificate = 0;
size_t selectedCapability = 0;
size_t selectedCephCluster = 0;

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
  const double tib = 1024.0 * 1024.0 * 1024.0 * 1024.0;
  const double gib = 1024.0 * 1024.0 * 1024.0;
  const double mib = 1024.0 * 1024.0;
  if (bytes >= static_cast<int64_t>(tib)) {
    double value = bytes / tib;
    if (value >= 10.0) return String(static_cast<int>(round(value))) + "T";
    return String(value, 1) + "T";
  }
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

String formatAge(int64_t seconds) {
  if (seconds <= 0) return "-";
  return formatUptime(seconds) + " ago";
}

String yesNo(bool value) {
  return value ? "yes" : "no";
}

String firstTag(const String &tags) {
  if (tags.length() == 0) return "";
  int split = tags.indexOf(";");
  if (split < 0) return tags;
  return tags.substring(0, split);
}

uint16_t colorForPercent(int pct) {
  if (pct >= 90) return TFT_RED;
  if (pct >= 75) return TFT_YELLOW;
  return TFT_GREEN;
}

String metricLabel(String metric) {
  metric.replace("_pct", "");
  metric.replace("_kib_s", "");
  metric.replace("_", " ");
  metric.toUpperCase();
  return metric;
}

String resourceLabel(const MetricTrend &trend) {
  String label = trend.resourceType;
  label.toUpperCase();
  if (trend.resourceName.length() > 0) label += " " + trend.resourceName;
  return label;
}

String capabilityHealth(const Capability &cap) {
  if (cap.status == "ok") return "ok";
  if (cap.status == "forbidden" || cap.status == "not_found" || cap.status == "not_available") return "warning";
  return "critical";
}

String trendHealth(const MetricTrend &trend) {
  if (!trend.metric.endsWith("_pct")) return "ok";
  if (trend.last >= 90) return "critical";
  if (trend.last >= 75) return "warning";
  return "ok";
}

String detailSyncLabel() {
  if (lastDetailOK == 0) return "never";
  return String((millis() - lastDetailOK) / 1000) + "s ago";
}

int zfsUsedPct(const ZFSPool &pool) {
  if (pool.size <= 0) return 0;
  return constrain(static_cast<int>((pool.allocated * 100) / pool.size), 0, 100);
}

int cephUsedPct(const CephCluster &ceph) {
  if (ceph.usage > 0) return ceph.usage;
  if (ceph.total <= 0) return 0;
  return constrain(static_cast<int>((ceph.used * 100) / ceph.total), 0, 100);
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
    host.memoryAvailable = h["memory_available_bytes"] | 0;
    host.swap = h["swap_pct"] | 0;
    host.swapUsed = h["swap_used_bytes"] | 0;
    host.swapTotal = h["swap_total_bytes"] | 0;
    host.ioWait = h["iowait_pct"] | 0;
    host.ksmShared = h["ksm_shared_bytes"] | 0;
    host.storage = h["storage_pct"] | 0;
    host.storageUsed = h["storage_used_bytes"] | 0;
    host.storageTotal = h["storage_total_bytes"] | 0;
    host.storageMax = h["storage_max_pct"] | host.storage;
    host.storageMaxName = h["storage_max_name"] | "";
    host.uptime = h["uptime_sec"] | 0;
    JsonArray load = h["load_avg"].as<JsonArray>();
    if (!load.isNull() && load.size() > 0) {
      host.loadAvg = load[0].as<String>();
      if (load.size() > 1) host.loadAvg += "/" + load[1].as<String>();
      if (load.size() > 2) host.loadAvg += "/" + load[2].as<String>();
    }
    host.pveVersion = h["pve_version"] | "";
    host.kernelVersion = h["kernel_version"] | "";
    host.primaryAddress = h["primary_address"] | "";
    host.networkActive = h["network_active"] | 0;
    host.networkTotal = h["network_total"] | 0;
    host.servicesRunning = h["services_running"] | 0;
    host.servicesFailed = h["services_failed"] | 0;
    host.servicesTotal = h["services_total"] | 0;
    host.diskCount = h["disk_count"] | 0;
    host.diskIssues = h["disk_issues"] | 0;
    host.failedTasks24h = h["failed_tasks_24h"] | 0;
    host.lastBackupStatus = h["last_backup_status"] | "";
    host.lastBackupAge = h["last_backup_age_sec"] | 0;
    JsonArray warnings = h["data_warnings"].as<JsonArray>();
    if (!warnings.isNull()) {
      for (JsonVariant warning : warnings) {
        if (host.dataWarnings.length() > 0) host.dataWarnings += ", ";
        host.dataWarnings += warning.as<String>();
        if (host.dataWarnings.length() > 48) break;
      }
    }
    host.running = h["guests_running"] | 0;
    host.stopped = h["guests_stopped"] | 0;
    host.health = h["health"] | "unknown";
    host.error = h["error"] | "";
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
    storage.path = s["path"] | "";
    storage.pool = s["pool"] | "";
    storage.mountpoint = s["mountpoint"] | "";
    storage.shared = s["shared"] | false;
    storage.disk = s["disk_pct"] | 0;
    storage.diskUsed = s["disk_used_bytes"] | 0;
    storage.diskTotal = s["disk_total_bytes"] | 0;
    storage.contentItems = s["content_items"] | 0;
    storage.backupCount = s["backup_count"] | 0;
    storage.isoCount = s["iso_count"] | 0;
    storage.templateCount = s["template_count"] | 0;
    storage.imageCount = s["image_count"] | 0;
    storage.rootdirCount = s["rootdir_count"] | 0;
    storage.health = s["health"] | "unknown";
  }

  if (state.storageCount == 0) {
    selectedStorage = 0;
  } else if (selectedStorage >= state.storageCount) {
    selectedStorage = state.storageCount - 1;
  }

  for (JsonObject d : doc["disks"].as<JsonArray>()) {
    if (state.diskCount >= MAX_DISKS) break;
    Disk &disk = state.disks[state.diskCount++];
    disk.name = d["name"] | "";
    disk.hostName = d["host_name"] | "";
    disk.model = d["model"] | "";
    disk.serial = d["serial"] | "";
    disk.type = d["type"] | "";
    disk.usedBy = d["used_by"] | "";
    disk.size = d["size_bytes"] | 0;
    disk.smartHealth = d["smart_health"] | "";
    disk.wearout = d["wearout_pct"] | 0;
    disk.health = d["health"] | "unknown";
  }

  if (state.diskCount == 0) {
    selectedDisk = 0;
  } else if (selectedDisk >= state.diskCount) {
    selectedDisk = state.diskCount - 1;
  }

  for (JsonObject g : doc["guests"].as<JsonArray>()) {
    if (state.guestCount >= MAX_GUESTS) break;
    Guest &guest = state.guests[state.guestCount++];
    guest.vmid = g["vmid"] | "";
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
    guest.memoryHost = g["memory_host_bytes"] | 0;
    guest.swap = g["swap_pct"] | 0;
    guest.swapUsed = g["swap_used_bytes"] | 0;
    guest.swapTotal = g["swap_total_bytes"] | 0;
    guest.diskUsed = g["disk_used_bytes"] | 0;
    guest.diskTotal = g["disk_total_bytes"] | 0;
    guest.uptime = g["uptime_sec"] | 0;
    guest.netIn = g["net_in_bytes"] | 0;
    guest.netOut = g["net_out_bytes"] | 0;
    guest.diskRead = g["disk_read_bytes"] | 0;
    guest.diskWrite = g["disk_write_bytes"] | 0;
    guest.tags = g["tags"] | "";
    guest.osType = g["os_type"] | "";
    guest.ipAddress = g["ip_address"] | "";
    guest.agentEnabled = g["agent_enabled"] | false;
    guest.agentAvailable = g["agent_available"] | false;
    guest.agentVersion = g["agent_version"] | "";
    guest.agentCommandCount = g["agent_command_count"] | 0;
    guest.qmpStatus = g["qmp_status"] | "";
    guest.runningQemu = g["running_qemu"] | "";
    guest.haManaged = g["ha_managed"] | false;
    guest.pressureCPU = max(g["pressure_cpu_some_pct"] | 0, g["pressure_cpu_full_pct"] | 0);
    guest.pressureIO = max(g["pressure_io_some_pct"] | 0, g["pressure_io_full_pct"] | 0);
    guest.pressureMemory = max(g["pressure_memory_some_pct"] | 0, g["pressure_memory_full_pct"] | 0);
    guest.onBoot = g["onboot"] | false;
    guest.protection = g["protection"] | false;
    guest.isTemplate = g["template"] | false;
    guest.unprivileged = g["unprivileged"] | false;
    guest.configWarning = g["config_warning"] | "";
    guest.pinned = g["pinned"] | false;
    guest.expected = g["expected"] | "";
    guest.health = g["health"] | "unknown";
  }

  if (state.guestCount == 0) {
    selectedGuest = 0;
  } else if (selectedGuest >= state.guestCount) {
    selectedGuest = state.guestCount - 1;
  }

  for (JsonObject t : doc["tasks"].as<JsonArray>()) {
    if (state.taskCount >= MAX_TASKS) break;
    Task &task = state.tasks[state.taskCount++];
    task.type = t["type"] | "";
    task.user = t["user"] | "";
    task.status = t["status"] | "";
    task.target = t["target"] | "";
    task.vmid = t["vmid"] | "";
    task.guestName = t["guest_name"] | "";
    task.hostName = t["host_name"] | "";
    task.startedAt = t["started_at"] | 0;
    task.startedAge = t["started_age_sec"] | 0;
    task.endedAt = t["ended_at"] | 0;
    task.duration = t["duration_sec"] | 0;
    task.health = t["health"] | "unknown";
  }

  if (state.taskCount == 0) {
    selectedTask = 0;
  } else if (selectedTask >= state.taskCount) {
    selectedTask = state.taskCount - 1;
  }

  for (JsonObject a : doc["alerts"].as<JsonArray>()) {
    if (state.alertCount >= MAX_ALERTS) break;
    Alert &alert = state.alerts[state.alertCount++];
    alert.severity = a["severity"] | "unknown";
    alert.title = a["title"] | "";
    alert.message = a["message"] | "";
  }

  if (state.alertCount == 0) {
    selectedAlert = 0;
  } else if (selectedAlert >= state.alertCount) {
    selectedAlert = state.alertCount - 1;
  }

  lastError = "";
  lastOK = millis();
  Serial.printf("json: hosts=%d storages=%d disks=%d guests=%d tasks=%d alerts=%d\n", state.hostCount,
                state.storageCount, state.diskCount, state.guestCount, state.taskCount, state.alertCount);
  return true;
}

bool parseDetailState(const String &payload) {
  DynamicJsonDocument doc(DETAIL_JSON_DOC_CAPACITY);
  DeserializationError err = deserializeJson(doc, payload);
  if (err) {
    lastDetailError = "detail JSON: " + String(err.c_str());
    Serial.printf("detail: parse failed: %s\n", err.c_str());
    return false;
  }

  InventoryDetails next;
  next.schema = doc["schema"] | "";
  next.generatedAt = doc["generated_at"] | "";
  next.stale = doc["stale"] | true;

  for (JsonObject z : doc["zfs_pools"].as<JsonArray>()) {
    if (next.zfsPoolCount >= MAX_ZFS_POOLS) break;
    ZFSPool &pool = next.zfsPools[next.zfsPoolCount++];
    pool.name = z["name"] | "";
    pool.hostName = z["host_name"] | "";
    pool.status = z["status"] | "";
    pool.state = z["state"] | "";
    pool.scan = z["scan"] | "";
    pool.errors = z["errors"] | "";
    pool.size = z["size_bytes"] | 0;
    pool.allocated = z["allocated_bytes"] | 0;
    pool.free = z["free_bytes"] | 0;
    pool.fragmentation = z["fragmentation_pct"] | 0;
    pool.deviceCount = z["device_count"] | 0;
    pool.issueCount = z["issue_count"] | 0;
    pool.health = z["health"] | "unknown";
  }

  for (JsonObject item : doc["storage_items"].as<JsonArray>()) {
    if (next.storageItemCount >= MAX_STORAGE_ITEMS) break;
    StorageItem &storageItem = next.storageItems[next.storageItemCount++];
    storageItem.storage = item["storage"] | "";
    storageItem.hostName = item["host_name"] | "";
    storageItem.content = item["content"] | "";
    storageItem.volid = item["volid"] | "";
    storageItem.vmid = item["vmid"] | "";
    storageItem.format = item["format"] | "";
    storageItem.size = item["size_bytes"] | 0;
    storageItem.createdAt = item["created_at"] | 0;
    storageItem.protectedItem = item["protected"] | false;
    storageItem.verificationState = item["verification_state"] | "";
    storageItem.health = item["health"] | "unknown";
  }

  for (JsonObject trendJSON : doc["metric_trends"].as<JsonArray>()) {
    if (next.trendCount >= MAX_TRENDS) break;
    MetricTrend &trend = next.trends[next.trendCount++];
    trend.resourceType = trendJSON["resource_type"] | "";
    trend.resourceName = trendJSON["resource_name"] | "";
    trend.metric = trendJSON["metric"] | "";
    trend.unit = trendJSON["unit"] | "";
    trend.last = trendJSON["last"] | 0;
    JsonArray values = trendJSON["values"].as<JsonArray>();
    for (JsonVariant value : values) {
      if (trend.valueCount >= MAX_TREND_VALUES) break;
      trend.values[trend.valueCount++] = value | 0;
    }
  }

  for (JsonObject certJSON : doc["certificates"].as<JsonArray>()) {
    if (next.certificateCount >= MAX_CERTIFICATES) break;
    Certificate &cert = next.certificates[next.certificateCount++];
    cert.filename = certJSON["filename"] | "";
    cert.hostName = certJSON["host_name"] | "";
    cert.subject = certJSON["subject"] | "";
    cert.issuer = certJSON["issuer"] | "";
    cert.daysRemaining = certJSON["days_remaining"] | 0;
    cert.health = certJSON["health"] | "unknown";
  }

  for (JsonObject cephJSON : doc["ceph_clusters"].as<JsonArray>()) {
    if (next.cephClusterCount >= MAX_CEPH_CLUSTERS) break;
    CephCluster &ceph = next.cephClusters[next.cephClusterCount++];
    ceph.sourceID = cephJSON["source_id"] | "";
    ceph.healthText = cephJSON["health_text"] | "";
    ceph.total = cephJSON["total_bytes"] | 0;
    ceph.used = cephJSON["used_bytes"] | 0;
    ceph.available = cephJSON["available_bytes"] | 0;
    ceph.usage = cephJSON["usage_pct"] | 0;
    ceph.osds = cephJSON["osds"] | 0;
    ceph.osdsUp = cephJSON["osds_up"] | 0;
    ceph.osdsIn = cephJSON["osds_in"] | 0;
    ceph.pgs = cephJSON["pgs"] | 0;
    ceph.health = cephJSON["health"] | "unknown";
  }

  for (JsonObject capJSON : doc["capabilities"].as<JsonArray>()) {
    if (next.capabilityCount >= MAX_CAPABILITIES) break;
    Capability &cap = next.capabilities[next.capabilityCount++];
    cap.sourceID = capJSON["source_id"] | "";
    cap.name = capJSON["name"] | "";
    cap.status = capJSON["status"] | "";
    cap.httpStatus = capJSON["http_status"] | 0;
    cap.message = capJSON["message"] | "";
  }

  details = next;
  if (selectedZFSPool >= details.zfsPoolCount) selectedZFSPool = details.zfsPoolCount == 0 ? 0 : details.zfsPoolCount - 1;
  if (selectedStorageItem >= details.storageItemCount) {
    selectedStorageItem = details.storageItemCount == 0 ? 0 : details.storageItemCount - 1;
  }
  if (selectedTrend >= details.trendCount) selectedTrend = details.trendCount == 0 ? 0 : details.trendCount - 1;
  if (selectedCertificate >= details.certificateCount) {
    selectedCertificate = details.certificateCount == 0 ? 0 : details.certificateCount - 1;
  }
  if (selectedCapability >= details.capabilityCount) {
    selectedCapability = details.capabilityCount == 0 ? 0 : details.capabilityCount - 1;
  }
  if (selectedCephCluster >= details.cephClusterCount) {
    selectedCephCluster = details.cephClusterCount == 0 ? 0 : details.cephClusterCount - 1;
  }

  lastDetailError = "";
  lastDetailOK = millis();
  Serial.printf("detail: zfs=%d content=%d trends=%d certs=%d ceph=%d caps=%d\n", details.zfsPoolCount,
                details.storageItemCount, details.trendCount, details.certificateCount, details.cephClusterCount,
                details.capabilityCount);
  return true;
}

bool fetchBridgePayload(const String &path, String &payload, String &errorOut, const char *label) {
  if (WiFi.status() != WL_CONNECTED) {
    errorOut = "Wi-Fi disconnected";
    Serial.printf("%s: skipped, Wi-Fi disconnected\n", label);
    return false;
  }

  HTTPClient http;
  String url = trimTrailingSlash(cfg.bridgeURL) + path;
  http.setTimeout(5000);
  if (!http.begin(url)) {
    errorOut = "bad bridge URL";
    Serial.printf("%s: bad URL\n", label);
    return false;
  }
  http.addHeader("Authorization", "Bearer " + cfg.displayToken);
  int code = http.GET();
  if (code != 200) {
    errorOut = String(label) + " HTTP " + String(code);
    Serial.printf("%s: HTTP %d\n", label, code);
    http.end();
    return false;
  }
  payload = http.getString();
  Serial.printf("%s: payload=%d bytes\n", label, payload.length());
  http.end();
  return true;
}

bool fetchDetailState() {
  String payload;
  String error;
  if (!fetchBridgePayload("/api/v1/detail-state", payload, error, "detail")) {
    lastDetailError = error;
    return false;
  }
  return parseDetailState(payload);
}

bool fetchState() {
  String payload;
  String error;
  if (!fetchBridgePayload("/api/v1/display-state", payload, error, "bridge")) {
    lastError = error;
    return false;
  }
  if (!parseState(payload)) {
    return false;
  }
  fetchDetailState();
  return true;
}

void drawBar(int x, int y, int w, int h, int pct, uint16_t color) {
  tft.drawRect(x, y, w, h, TFT_DARKGREY);
  int fill = map(constrain(pct, 0, 100), 0, 100, 0, w - 2);
  tft.fillRect(x + 1, y + 1, fill, h - 2, color);
}

void drawSparkline(int x, int y, int w, int h, const MetricTrend &trend) {
  tft.drawRect(x, y, w, h, TFT_DARKGREY);
  if (trend.valueCount == 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("no samples", x + 6, y + h / 2 - 4);
    return;
  }

  int maxValue = trend.metric.endsWith("_pct") ? 100 : 1;
  for (size_t i = 0; i < trend.valueCount; ++i) {
    if (trend.values[i] > maxValue) maxValue = trend.values[i];
  }

  int lastX = x + 2;
  int lastY = y + h - 3 - map(constrain(trend.values[0], 0, maxValue), 0, maxValue, 0, h - 6);
  size_t denominator = trend.valueCount > 1 ? trend.valueCount - 1 : 1;
  for (size_t i = 1; i < trend.valueCount; ++i) {
    int px = x + 2 + static_cast<int>((i * (w - 5)) / denominator);
    int py = y + h - 3 - map(constrain(trend.values[i], 0, maxValue), 0, maxValue, 0, h - 6);
    tft.drawLine(lastX, lastY, px, py, trend.metric.endsWith("_pct") ? colorForPercent(trend.last) : TFT_CYAN);
    lastX = px;
    lastY = py;
  }
}

void drawMetricRow(const String &label, const String &value, int pct, uint16_t color, int y) {
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString(label, 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(value, 72, y);
  drawBar(205, y, 85, 8, pct, color);
}

int storagePressureForHost(const Host &host) {
  int pressure = max(host.storage, host.storageMax);
  for (size_t i = 0; i < state.storageCount; ++i) {
    const Storage &s = state.storages[i];
    if (s.hostName == host.name) pressure = max(pressure, s.disk);
  }
  return pressure;
}

String storagePressureNameForHost(const Host &host) {
  String name = host.storageMaxName;
  int pressure = max(host.storage, host.storageMax);
  for (size_t i = 0; i < state.storageCount; ++i) {
    const Storage &s = state.storages[i];
    if (s.hostName == host.name && s.disk >= pressure) {
      pressure = s.disk;
      name = s.name;
    }
  }
  return name.length() ? name : "root";
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

  Host &focus = state.hosts[selectedHost];
  int focusStorageMax = storagePressureForHost(focus);

  int y = 52;
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("FOCUS " + String(selectedHost + 1) + "/" + String(state.hostCount), 10, y);
  tft.setTextColor(focus.online ? TFT_WHITE : TFT_RED, TFT_BLACK);
  tft.drawString(clipText(focus.name, 24), 86, y);

  y += 16;
  drawMetricRow("CPU", String(focus.cpu) + "% / " + String(focus.maxCPU) + " cores", focus.cpu, TFT_CYAN, y);
  y += 18;
  drawMetricRow("RAM", formatBytes(focus.memoryUsed) + " / " + formatBytes(focus.memoryTotal), focus.memory,
                focus.memory >= 90 ? TFT_RED : TFT_GREEN, y);
  y += 18;
  drawMetricRow("DSK", "max " + String(focusStorageMax) + "%", focusStorageMax,
                focusStorageMax >= 90 ? TFT_RED : TFT_YELLOW, y);

  y += 18;
  tft.fillRect(11, y + 3, 7, 7, colorForHealth(focus.health));
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("load " + (focus.loadAvg.length() ? focus.loadAvg : "-"), 28, y);
  tft.drawString(String(focus.running) + " run", 160, y);

  y += 16;
  tft.setTextColor(state.summary.alerts > 0 ? colorForHealth(state.summary.health) : TFT_GREEN, TFT_BLACK);
  tft.drawString(state.summary.alerts > 0 ? "!" + String(state.summary.alerts) : "OK", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  if (state.summary.alerts > 0 && state.alertCount > 0) {
    tft.drawString(clipText(state.alerts[0].title, 40), 42, y);
  } else {
    tft.drawString("All configured checks are OK", 42, y);
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
    tft.drawString(String(storagePressureForHost(h)), 286, y);
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

  if (!h.online && h.error.length() > 0) {
    drawWrappedField("ERR", h.error, 58, 4);
    drawFooter();
    return;
  }

  int y = 58;
  drawMetricRow("CPU", String(h.cpu) + "% / " + String(h.maxCPU) + " cores", h.cpu, TFT_CYAN, y);

  y += 18;
  drawMetricRow("RAM", formatBytes(h.memoryUsed) + " / " + formatBytes(h.memoryTotal), h.memory,
                colorForPercent(h.memory), y);

  y += 18;
  if (h.swapTotal > 0) {
    drawMetricRow("SWAP", formatBytes(h.swapUsed) + " / " + formatBytes(h.swapTotal), h.swap,
                  colorForPercent(h.swap), y);
    y += 18;
  }

  int pressure = storagePressureForHost(h);
  String pressureName = clipText(storagePressureNameForHost(h), 10);
  drawMetricRow("STOR", pressureName + " " + String(pressure) + "%", pressure, pressure >= 90 ? TFT_RED : TFT_YELLOW, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("LOAD", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(h.loadAvg.length() ? h.loadAvg : "-", 72, y);
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("IOW", 160, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(h.ioWait) + "%", 190, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("UP", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatUptime(h.uptime) + "  " + String(h.running) + " run/" + String(h.stopped) + " stop", 72, y);

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

void drawHostOps() {
  tft.setTextSize(1);
  if (state.hostCount == 0) {
    drawHeader("OPS", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No host selected", 10, 42);
    drawFooter();
    return;
  }

  Host &h = state.hosts[selectedHost];
  drawHeader("OPS", h.health);
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
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("NET", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String net = String(h.networkActive) + "/" + String(h.networkTotal);
  if (h.primaryAddress.length() > 0) net += "  " + h.primaryAddress;
  tft.drawString(clipText(net, 35), 72, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("SVC", 10, y);
  tft.setTextColor(h.servicesFailed > 0 ? TFT_RED : TFT_WHITE, TFT_BLACK);
  String svc = String(h.servicesRunning) + "/" + String(h.servicesTotal) + " running";
  if (h.servicesFailed > 0) svc += "  " + String(h.servicesFailed) + " failed";
  tft.drawString(clipText(svc, 35), 72, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("DISKS", 10, y);
  tft.setTextColor(h.diskIssues > 0 ? TFT_RED : TFT_WHITE, TFT_BLACK);
  String disks = String(h.diskCount);
  if (h.diskIssues > 0) disks += "  " + String(h.diskIssues) + " issue";
  tft.drawString(disks, 72, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("TASKS", 10, y);
  tft.setTextColor(h.failedTasks24h > 0 ? TFT_YELLOW : TFT_WHITE, TFT_BLACK);
  tft.drawString(String(h.failedTasks24h) + " failed 24h", 72, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("BACKUP", 10, y);
  tft.setTextColor(h.lastBackupStatus == "OK" ? TFT_GREEN : TFT_WHITE, TFT_BLACK);
  String backup = h.lastBackupStatus.length() ? h.lastBackupStatus + "  " + formatAge(h.lastBackupAge) : "-";
  tft.drawString(clipText(backup, 35), 72, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("DATA", 10, y);
  tft.setTextColor(h.dataWarnings.length() > 0 ? TFT_YELLOW : TFT_GREEN, TFT_BLACK);
  tft.drawString(h.dataWarnings.length() > 0 ? clipText(h.dataWarnings, 35) : "all collectors OK", 72, y);

  drawFooter();
}

void drawDisks() {
  tft.setTextSize(1);
  if (state.diskCount == 0) {
    drawHeader("DISKS", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No physical disk data", 10, 42);
    drawFooter();
    return;
  }

  Disk &d = state.disks[selectedDisk];
  drawHeader("DISKS", d.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(d.health), TFT_BLACK);
  tft.drawString(d.smartHealth.length() ? clipText(d.smartHealth, 10) : "UNKNOWN", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(d.name, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedDisk + 1) + "/" + String(state.diskCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("MODEL", d.model, y, 2);
  y = drawWrappedField("HOST", d.hostName, y, 1);
  y = drawWrappedField("SIZE", formatBytes(d.size) + "  " + d.type, y, 1);
  y = drawWrappedField("USED", d.usedBy, y, 1);
  String wear = d.wearout > 0 ? String(d.wearout) + "%" : "-";
  drawWrappedField("WEAR", wear + "  SN " + d.serial, y, 1);

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
  if (s.shared) where += "  shared";
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
  tft.drawString("ITEMS", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String items = String(s.contentItems);
  if (s.imageCount > 0) items += " img " + String(s.imageCount);
  if (s.backupCount > 0) items += " bak " + String(s.backupCount);
  if (s.isoCount > 0) items += " iso " + String(s.isoCount);
  if (s.templateCount > 0) items += " tpl " + String(s.templateCount);
  tft.drawString(clipText(items, 33), 92, y);

  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("PATH", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  String path = s.pool.length() ? s.pool : s.mountpoint;
  if (path.length() == 0) path = s.path;
  if (path.length() == 0) path = s.content;
  tft.drawString(path.length() ? clipText(path, 33) : "-", 92, y);

  drawFooter();
}

void drawStorageContent() {
  tft.setTextSize(1);
  if (details.storageItemCount == 0) {
    drawHeader("CONTENT", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No storage content detail", 10, 42);
    drawFooter();
    return;
  }

  StorageItem &item = details.storageItems[selectedStorageItem];
  drawHeader("CONTENT", item.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(item.health), TFT_BLACK);
  tft.drawString(item.content.length() ? clipText(item.content, 10) : "ITEM", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(item.storage, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedStorageItem + 1) + "/" + String(details.storageItemCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("VOL", item.volid, y, 2);
  y = drawWrappedField("HOST", item.hostName, y, 1);
  y = drawWrappedField("SIZE", formatBytes(item.size) + "  " + item.format, y, 1);
  String flags = item.vmid.length() ? "vmid " + item.vmid : "-";
  if (item.protectedItem) flags += " protected";
  y = drawWrappedField("FLAGS", flags, y, 1);
  drawWrappedField("VERIFY", item.verificationState.length() ? item.verificationState : "-", y, 1);
  drawFooter();
}

void drawZFSPools() {
  tft.setTextSize(1);
  if (details.zfsPoolCount == 0) {
    drawHeader("ZFS", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No ZFS pool detail", 10, 42);
    drawFooter();
    return;
  }

  ZFSPool &pool = details.zfsPools[selectedZFSPool];
  drawHeader("ZFS", pool.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(pool.health), TFT_BLACK);
  tft.drawString(pool.status.length() ? clipText(pool.status, 10) : "POOL", 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(pool.name, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedZFSPool + 1) + "/" + String(details.zfsPoolCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);
  tft.drawString(clipText(pool.hostName, 42), 10, 54);

  int used = zfsUsedPct(pool);
  int y = 74;
  drawMetricRow("USED", formatBytes(pool.allocated) + " / " + formatBytes(pool.size), used, colorForPercent(used), y);
  y += 18;
  drawMetricRow("FRAG", String(pool.fragmentation) + "%", pool.fragmentation, colorForPercent(pool.fragmentation), y);
  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("DEV", 10, y);
  tft.setTextColor(pool.issueCount > 0 ? TFT_RED : TFT_WHITE, TFT_BLACK);
  tft.drawString(String(pool.deviceCount) + " dev  " + String(pool.issueCount) + " issue", 72, y);
  y += 18;
  y = drawWrappedField("SCAN", pool.scan, y, 1);
  drawWrappedField("ERR", pool.errors, y, 1);
  drawFooter();
}

void drawCeph() {
  tft.setTextSize(1);
  if (details.cephClusterCount == 0) {
    drawHeader("CEPH", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No Ceph cluster detail", 10, 42);
    drawFooter();
    return;
  }

  CephCluster &ceph = details.cephClusters[selectedCephCluster];
  drawHeader("CEPH", ceph.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(ceph.health), TFT_BLACK);
  tft.drawString(clipText(ceph.healthText.length() ? ceph.healthText : ceph.health, 10), 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(ceph.sourceID, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedCephCluster + 1) + "/" + String(details.cephClusterCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int used = cephUsedPct(ceph);
  int y = 58;
  drawMetricRow("USED", formatBytes(ceph.used) + " / " + formatBytes(ceph.total), used, colorForPercent(used), y);
  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("AVAIL", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(ceph.available), 92, y);
  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("OSD", 10, y);
  tft.setTextColor(ceph.osdsUp == ceph.osds && ceph.osdsIn == ceph.osds ? TFT_WHITE : TFT_YELLOW, TFT_BLACK);
  tft.drawString(String(ceph.osdsUp) + "/" + String(ceph.osds) + " up  " + String(ceph.osdsIn) + "/" +
                     String(ceph.osds) + " in",
                 92, y);
  y += 20;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("PG", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(String(ceph.pgs), 92, y);
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
    String label = g.vmid.length() ? g.vmid + " " + g.name : g.name;
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
  String where = g.type;
  if (g.vmid.length() > 0) where += " " + g.vmid;
  where += "  " + g.hostName + "  up " + formatUptime(g.uptime);
  if (g.tags.length() > 0) {
    where += "  #" + firstTag(g.tags);
  }
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
  drawBar(205, y, 85, 8, g.memory, colorForPercent(g.memory));

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("DISK", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.diskUsed) + " / " + formatBytes(g.diskTotal), 92, y);
  drawBar(205, y, 85, 8, g.disk, colorForPercent(g.disk));

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("NET", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.netIn) + " in  " + formatBytes(g.netOut) + " out", 92, y);

  y += 18;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("IO", 10, y);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(formatBytes(g.diskRead) + " r  " + formatBytes(g.diskWrite) + " w", 92, y);

  drawFooter();
}

void drawGuestConfig() {
  tft.setTextSize(1);
  if (state.guestCount == 0) {
    drawHeader("GCFG", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No guest selected", 10, 42);
    drawFooter();
    return;
  }

  Guest &g = state.guests[selectedGuest];
  drawHeader("GCFG", g.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(g.health), TFT_BLACK);
  tft.drawString(g.status == "running" ? "RUNNING" : g.status, 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(g.name, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedGuest + 1) + "/" + String(state.guestCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("VMID", g.type + " " + g.vmid + "  " + g.hostName, y, 1);
  y = drawWrappedField("OS/IP", g.osType + "  " + g.ipAddress, y, 1);
  String flags = String("boot ") + yesNo(g.onBoot) + "  prot " + yesNo(g.protection);
  if (g.isTemplate) flags += "  template";
  if (g.unprivileged) flags += "  unpriv";
  y = drawWrappedField("FLAGS", flags, y, 1);
  String agent = yesNo(g.agentEnabled);
  if (g.agentAvailable) agent += " up";
  else if (g.agentEnabled) agent += " down";
  if (g.agentVersion.length() > 0) agent += " v" + g.agentVersion;
  if (g.agentCommandCount > 0) agent += " cmd " + String(g.agentCommandCount);
  y = drawWrappedField("AGENT", agent, y, 1);
  String runtime = g.qmpStatus.length() ? "qmp " + g.qmpStatus : "";
  if (g.runningQemu.length() > 0) runtime += " qemu " + g.runningQemu;
  if (g.haManaged) runtime += " HA";
  y = drawWrappedField("RUN", runtime.length() ? runtime : "-", y, 1);
  String finalLine;
  String finalLabel = "TAG";
  if (g.pressureCPU > 0 || g.pressureIO > 0 || g.pressureMemory > 0) {
    finalLabel = "PRESS";
    finalLine = "cpu " + String(g.pressureCPU) + "% io " + String(g.pressureIO) + "% mem " + String(g.pressureMemory) + "%";
  } else if (g.pinned) {
    finalLabel = "EXPECT";
    finalLine = g.expected;
  } else if (g.configWarning.length() > 0) {
    finalLabel = "DATA";
    finalLine = g.configWarning;
  } else {
    finalLine = firstTag(g.tags);
  }
  drawWrappedField(finalLabel, finalLine, y, 1);

  drawFooter();
}

void drawTrends() {
  tft.setTextSize(1);
  if (details.trendCount == 0) {
    drawHeader("TREND", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No RRD trend detail", 10, 42);
    drawFooter();
    return;
  }

  MetricTrend &trend = details.trends[selectedTrend];
  drawHeader("TREND", trendHealth(trend));
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(trendHealth(trend)), TFT_BLACK);
  tft.drawString(metricLabel(trend.metric), 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(resourceLabel(trend), 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedTrend + 1) + "/" + String(details.trendCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  String last = String(trend.last);
  if (trend.unit.length() > 0) last += " " + trend.unit;
  tft.setTextColor(TFT_LIGHTGREY, TFT_BLACK);
  tft.drawString("LAST", 10, 56);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(last, 72, 56);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("1 hour, " + String(trend.valueCount) + " samples", 10, 72);
  drawSparkline(10, 88, tft.width() - 20, 50, trend);
  drawFooter();
}

void drawTasks() {
  tft.setTextSize(1);
  if (state.taskCount == 0) {
    drawHeader("TASKS", "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString("No recent task data", 10, 42);
    drawFooter();
    return;
  }

  Task &task = state.tasks[selectedTask];
  drawHeader("TASKS", task.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(task.health), TFT_BLACK);
  tft.drawString(clipText(task.status, 10), 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(task.type, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedTask + 1) + "/" + String(state.taskCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("TARGET", task.target, y, 2);
  y = drawWrappedField("HOST", task.hostName, y, 1);
  y = drawWrappedField("USER", task.user, y, 1);
  String timing = String("start ") + formatAge(task.startedAge);
  if (task.duration > 0) timing += "  dur " + formatUptime(task.duration);
  y = drawWrappedField("TIME", timing, y, 1);
  drawWrappedField("STATUS", task.status, y, 2);

  drawFooter();
}

void drawCertificates() {
  tft.setTextSize(1);
  if (details.certificateCount == 0) {
    drawHeader("CERTS", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No certificate detail", 10, 42);
    drawFooter();
    return;
  }

  Certificate &cert = details.certificates[selectedCertificate];
  drawHeader("CERTS", cert.health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(cert.health), TFT_BLACK);
  String days = cert.daysRemaining > 0 ? String(cert.daysRemaining) + "d" : "expired";
  tft.drawString(days, 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(cert.filename, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedCertificate + 1) + "/" + String(details.certificateCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("HOST", cert.hostName, y, 1);
  y = drawWrappedField("SUBJ", cert.subject, y, 2);
  y = drawWrappedField("ISSUER", cert.issuer, y, 2);
  drawWrappedField("LEFT", days, y, 1);
  drawFooter();
}

void drawCapabilities() {
  tft.setTextSize(1);
  if (details.capabilityCount == 0) {
    drawHeader("CAPS", lastDetailError.length() ? "warning" : "");
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.drawString(lastDetailError.length() ? clipText(lastDetailError, 40) : "No endpoint diagnostics", 10, 42);
    drawFooter();
    return;
  }

  Capability &cap = details.capabilities[selectedCapability];
  String health = capabilityHealth(cap);
  drawHeader("CAPS", health);
  tft.setTextSize(1);
  tft.setTextColor(colorForHealth(health), TFT_BLACK);
  tft.drawString(clipText(cap.status, 10), 10, 38);
  tft.setTextColor(TFT_WHITE, TFT_BLACK);
  tft.drawString(clipText(cap.name, 22), 86, 38);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.setTextDatum(TR_DATUM);
  tft.drawString(String(selectedCapability + 1) + "/" + String(details.capabilityCount), tft.width() - 8, 38);
  tft.setTextDatum(TL_DATUM);

  int y = 56;
  y = drawWrappedField("SRC", cap.sourceID, y, 1);
  String status = cap.status;
  if (cap.httpStatus > 0) status += " HTTP " + String(cap.httpStatus);
  y = drawWrappedField("STATE", status, y, 1);
  y = drawWrappedField("MSG", cap.message, y, 2);
  tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
  tft.drawString("detail sync " + detailSyncLabel(), 10, y);
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

  const int rowH = 34;
  size_t visibleRows = static_cast<size_t>((tft.height() - 26 - y) / rowH);
  if (visibleRows == 0) visibleRows = 1;
  size_t start = visibleWindowStart(selectedAlert, state.alertCount, visibleRows);
  size_t end = start + visibleRows;
  if (end > state.alertCount) end = state.alertCount;
  if (state.alertCount > 0) {
    tft.setTextColor(TFT_DARKGREY, TFT_BLACK);
    tft.setTextDatum(TR_DATUM);
    tft.drawString(String(static_cast<int>(start + 1)) + "-" + String(static_cast<int>(end)) + "/" +
                       String(static_cast<int>(state.alertCount)),
                   tft.width() - 8, 38);
    tft.setTextDatum(TL_DATUM);
  }

  for (size_t i = start; i < end && y < tft.height() - 26; ++i) {
    Alert &a = state.alerts[i];
    uint16_t rowBg = i == selectedAlert ? TFT_DARKGREY : TFT_BLACK;
    if (i == selectedAlert) tft.fillRect(6, y - 1, tft.width() - 12, 31, rowBg);
    tft.setTextColor(colorForHealth(a.severity), rowBg);
    tft.drawString(a.severity.substring(0, 4), 10, y);
    tft.setTextColor(TFT_WHITE, rowBg);
    String title = a.title;
    if (title.length() > 31) title = title.substring(0, 31);
    tft.drawString(title, 54, y);
    y += 16;
    tft.setTextColor(i == selectedAlert ? TFT_LIGHTGREY : TFT_DARKGREY, rowBg);
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
  tft.drawString("Detail", 10, y);
  tft.setTextColor(lastDetailError.length() == 0 ? TFT_GREEN : TFT_YELLOW, TFT_BLACK);
  tft.drawString(lastDetailError.length() == 0 ? detailSyncLabel() : lastDetailError.substring(0, 28), 90, y);
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
      drawHostOps();
      break;
    case 5:
      drawDisks();
      break;
    case 6:
      drawStorage();
      break;
    case 7:
      drawStorageContent();
      break;
    case 8:
      drawZFSPools();
      break;
    case 9:
      drawCeph();
      break;
    case 10:
      drawGuests();
      break;
    case 11:
      drawGuestDetail();
      break;
    case 12:
      drawGuestConfig();
      break;
    case 13:
      drawTrends();
      break;
    case 14:
      drawTasks();
      break;
    case 15:
      drawCertificates();
      break;
    case 16:
      drawCapabilities();
      break;
    case 17:
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

void nextDisk() {
  if (state.diskCount == 0) return;
  selectedDisk = (selectedDisk + 1) % state.diskCount;
  render();
}

void nextTask() {
  if (state.taskCount == 0) return;
  selectedTask = (selectedTask + 1) % state.taskCount;
  render();
}

void nextAlert() {
  if (state.alertCount == 0) return;
  selectedAlert = (selectedAlert + 1) % state.alertCount;
  render();
}

void nextZFSPool() {
  if (details.zfsPoolCount == 0) return;
  selectedZFSPool = (selectedZFSPool + 1) % details.zfsPoolCount;
  render();
}

void nextStorageItem() {
  if (details.storageItemCount == 0) return;
  selectedStorageItem = (selectedStorageItem + 1) % details.storageItemCount;
  render();
}

void nextTrend() {
  if (details.trendCount == 0) return;
  selectedTrend = (selectedTrend + 1) % details.trendCount;
  render();
}

void nextCertificate() {
  if (details.certificateCount == 0) return;
  selectedCertificate = (selectedCertificate + 1) % details.certificateCount;
  render();
}

void nextCapability() {
  if (details.capabilityCount == 0) return;
  selectedCapability = (selectedCapability + 1) % details.capabilityCount;
  render();
}

void nextCephCluster() {
  if (details.cephClusterCount == 0) return;
  selectedCephCluster = (selectedCephCluster + 1) % details.cephClusterCount;
  render();
}

void manualRefresh() {
  fetchState();
  render();
}

void buttonBShort() {
  if (screenIndex == 0 || screenIndex == 1 || screenIndex == 2 || screenIndex == 3 || screenIndex == 4) {
    nextHost();
    return;
  }
  if (screenIndex == 5) {
    nextDisk();
    return;
  }
  if (screenIndex == 6) {
    nextStorage();
    return;
  }
  if (screenIndex == 7) {
    nextStorageItem();
    return;
  }
  if (screenIndex == 8) {
    nextZFSPool();
    return;
  }
  if (screenIndex == 9) {
    nextCephCluster();
    return;
  }
  if (screenIndex == 10 || screenIndex == 11 || screenIndex == 12) {
    nextGuest();
    return;
  }
  if (screenIndex == 13) {
    nextTrend();
    return;
  }
  if (screenIndex == 14) {
    nextTask();
    return;
  }
  if (screenIndex == 15) {
    nextCertificate();
    return;
  }
  if (screenIndex == 16) {
    nextCapability();
    return;
  }
  if (screenIndex == 17) {
    nextAlert();
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
