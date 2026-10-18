package check

// The certification-exam checks (EXAM): what the three problems leave in the exam station
// (Cristian_Angeles_Cert) and in its two modules, Cristian_Angeles_Cert-rt (problems 1 and 2) and
// websiteDriver-rt (problem 3). Keys are the problems P1-P3 of certification-exam.md. Defaults
// from the solution sources (sources/n4-dev-course/solutions/cert-exam/, S/ below): BLimiter
// out/in/max/min BDouble.DEFAULT 0.0 (S/.../BLimiter.java:17-38); BHistoryExportService historyId
// BHistoryId.DEFAULT, empty (S/.../BHistoryExportService.java:29-33); BWebsiteDriverDevice website ""
// (S/websiteDriver/.../BWebsiteDriverDevice.java:46-50); BPingHealth lastOkTime BAbsTime.DEFAULT
// (organized/driver/driver-rt/decompiled/javax/baja/driver/ping/BPingHealth.java:42-44).

const (
	certRT        = "Cristian_Angeles_Cert-rt"
	websiteRT     = "websiteDriver-rt"
	limiter       = "Cristian_Angeles_Cert:Limiter"
	historyExport = "Cristian_Angeles_Cert:HistoryExportService"
	websiteDevice = "websiteDriver:WebsiteDriverDevice"
	historyCSV    = "shared/history.csv" // outputFile default file:^history.csv
)

var examSpec = []Check{
	// Problem 1: a Limiter with Min 25 and Max 75 in Config/Test, fed by a kitControl Ramp.
	{ID: "CEXAM.1", Lab: "EXAM", Keys: []string{"P1"}, Kind: kindType, Desc: "a Limiter in Config/Test",
		Type: limiter, Under: "Test", Op: opMin, N: 1},
	{ID: "CEXAM.2", Lab: "EXAM", Keys: []string{"P1"}, Kind: kindSlot, Desc: "Limiter min = 25",
		Type: limiter, Slot: "min", Want: "25.0", Default: "0.0"},
	{ID: "CEXAM.3", Lab: "EXAM", Keys: []string{"P1"}, Kind: kindSlot, Desc: "Limiter max = 75",
		Type: limiter, Slot: "max", Want: "75.0", Default: "0.0"},
	{ID: "CEXAM.4", Lab: "EXAM", Keys: []string{"P1"}, Kind: kindLink, Desc: "Ramp.out -> Limiter.in",
		From: "kitControl:Ramp", FromSlot: "out", Type: limiter, Slot: "in", Op: opMin, N: 1},
	{ID: "CEXAM.5", Lab: "EXAM", Keys: []string{"P1"},
		Unobserved: map[string]string{"P1": "that out followed in between min and max (the bog holds the last out)"}, Kind: kindSlot,
		Desc: "Limiter computed out (out stored)", Type: limiter, Slot: "out", Set: true},
	{ID: "CEXAM.6", Lab: "EXAM", Keys: []string{"P1", "P2"}, Kind: kindJarSigned, Desc: "Cristian_Angeles_Cert-rt is signed", Jar: certRT},
	{ID: "CEXAM.7", Lab: "EXAM", Keys: []string{"P1"}, Kind: kindJarEntry, Desc: "BLimiter.class built",
		Jar: certRT, Glob: "**/BLimiter.class", Op: opMin, N: 1},

	// Problem 2: the service in Services, History Id = the station's AuditHistory, Export invoked.
	{ID: "CEXAM.8", Lab: "EXAM", Keys: []string{"P2"}, Kind: kindType, Desc: "one HistoryExportService, under Services",
		Type: historyExport, Within: services, Op: opEq, N: 1},
	{ID: "CEXAM.9", Lab: "EXAM", Keys: []string{"P2"}, Kind: kindSlot, Desc: "historyId = /<station>/AuditHistory",
		Type: historyExport, Slot: "historyId", Want: "/Cristian_Angeles_Cert/AuditHistory", Default: ""},
	{ID: "CEXAM.10", Lab: "EXAM", Keys: []string{"P2"}, Kind: kindFiles, Desc: "history.csv in the station's Files space",
		Glob: historyCSV, Op: opEq, N: 1},
	{ID: "CEXAM.11", Lab: "EXAM", Keys: []string{"P2"}, Kind: kindFileText, Desc: "history.csv header Timestamp,Operation,Target,SlotName",
		Glob: historyCSV, Line: 1, Match: `^Timestamp,Operation,Target,SlotName$`},
	{ID: "CEXAM.12", Lab: "EXAM", Keys: []string{"P2"},
		Unobserved: map[string]string{"P2": "that the export ran off the client thread (Flags.ASYNC)"}, Kind: kindFileText,
		Desc: "history.csv rows: timestamp, operation, target, slot", Glob: historyCSV,
		Match: `^\d{1,2} \S+ \d{4} \d{2}:\d{2}(?::\d{2})? \S+,[A-Za-z]+,[^,]*,`, Op: opMin, N: 1},
	{ID: "CEXAM.13", Lab: "EXAM", Keys: []string{"P2"}, Kind: kindJarEntry, Desc: "BHistoryExportService.class built",
		Jar: certRT, Glob: "**/BHistoryExportService.class", Op: opMin, N: 1},

	// Problem 3: Drivers -> WebsiteDriverNetwork -> WebsiteDriverDevice, Website set, Ping ok. The class
	// names are those of the live websiteDriver-rt.jar (com/certExam/websiteDriver/, 2026-10-16).
	{ID: "CEXAM.14", Lab: "EXAM", Keys: []string{"P3"}, Kind: kindType, Desc: "a WebsiteDriverDevice under Drivers",
		Type: websiteDevice, Within: "driver:DriverContainer", Op: opMin, N: 1},
	{ID: "CEXAM.15", Lab: "EXAM", Keys: []string{"P3"}, Kind: kindSlot, Desc: "device website set",
		Type: websiteDevice, Slot: "website", Set: true},
	{ID: "CEXAM.16", Lab: "EXAM", Keys: []string{"P3"},
		Unobserved: map[string]string{"P3": "the {ok} device status (the bog reader keeps no status slot)"}, Kind: kindSlot,
		Desc: "device pinged ok (health.lastOkTime stored)", Type: websiteDevice, Slot: "health.lastOkTime", Set: true},
	{ID: "CEXAM.17", Lab: "EXAM", Keys: []string{"P3"}, Kind: kindJarSigned, Desc: "websiteDriver-rt is signed", Jar: websiteRT},
	{ID: "CEXAM.18", Lab: "EXAM", Keys: []string{"P3"}, Kind: kindJarEntry, Desc: "network, device and folder classes built",
		Jar: websiteRT, Glob: "**/websiteDriver/BWebsiteDriver{Network,Device,DeviceFolder}.class", Op: opEq, N: 3},
	{ID: "CEXAM.19", Lab: "EXAM", Keys: []string{"P3"}, Kind: kindJarText, Desc: "websiteDriver requests NETWORK_COMMUNICATION",
		Jar: websiteRT, Glob: trainingXML, Contains: []string{"<name>NETWORK_COMMUNICATION</name>"}},
}
