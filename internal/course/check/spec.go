package check

// The lab check spec: what a correct solution of each lab leaves in a station (components,
// slot values, links, output files) or in the install (built, signed jars). It is data; the
// kinds are run by check.go. Types are "module:Type" with the module name (training,
// trafficLightDriver, ...), never the bog's per-station prefix. Expected defaults are cited
// from the lab sources under sources/n4-dev-course/course/Labs/ (LAB/ below) or, for a
// framework type, from organized/.
//
// Lab 8 (Investigating ORDs) has no checks: it writes no code and changes nothing in the
// station. Lab 5 (code signing) leaves nothing in the station, so its checks read the jars.

const (
	numericWritable = "control:NumericWritable"
	trainingRT      = "training-rt"
	trainingXML     = "META-INF/module.xml"
	services        = "baja:ServiceContainer"
	csvGlob         = "shared/{station}_NumericWritable*.csv" // the Lab 10 export (LI:1976-1977)
)

var Spec = []Check{
	// Lab 1. BFirstComponent.java:15-20 timeExecuted, default BAbsTime.DEFAULT (not stored);
	// doExecute (:28-32) sets it to now.
	{ID: "C1.1", Lab: "1", Keys: []string{"L1.24"}, Kind: kindType, Desc: "a FirstComponent in the station",
		Type: "training:FirstComponent", Op: opMin, N: 1},
	{ID: "C1.2", Lab: "1", Keys: []string{"L1.25"},
		Unobserved: map[string]string{"L1.25": "the Hello, World! line in the station console"}, Kind: kindSlot, Desc: "FirstComponent executed (timeExecuted stored)",
		Type: "training:FirstComponent", Slot: "timeExecuted", Set: true},
	{ID: "C1.3", Lab: "1", Keys: []string{"L1.13", "L1.15"}, Kind: kindJar, Desc: "training-rt jar built", Jar: trainingRT},
	{ID: "C1.4", Lab: "1", Keys: []string{"L1.15"}, Kind: kindJar, Desc: "training-wb jar built", Jar: "training-wb"},
	{ID: "C1.5", Lab: "1", Keys: []string{"L1.16"}, Kind: kindJarEntry, Desc: "BFirstComponent.class in training-rt",
		Jar: trainingRT, Glob: "com/tridiumuniversity/training/BFirstComponent.class", Op: opMin, N: 1},
	{ID: "C1.6", Lab: "1", Keys: []string{"L1.14"},
		Unobserved: map[string]string{"L1.14": "module-include.xml itself and the class attribute (the jar holds the built module.xml)"}, Kind: kindJarText, Desc: "module.xml registers FirstComponent",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{`name="FirstComponent"`}},
	{ID: "C1.7", Lab: "1", Keys: []string{"L1.11", "L1.18"}, Kind: kindJarText, Desc: "palette lists FirstComponent",
		Jar: trainingRT, Glob: "module.palette", Contains: []string{`n="FirstComponent"`}},

	// Lab 2. LAB/Lab2 Add Part 1/BAdd.java:15-42: out, in1..in4, BStatusNumeric.
	{ID: "C2.1", Lab: "2", Keys: []string{"L2.8"}, Kind: kindType, Desc: "an Add in the station",
		Type: "training:Add", Op: opMin, N: 1},
	{ID: "C2.2", Lab: "2", Keys: []string{"L2.1", "L2.7"}, Kind: kindJarEntry, Desc: "BAdd.class in training-rt",
		Jar: trainingRT, Glob: "com/tridiumuniversity/training/kitControl/BAdd.class", Op: opMin, N: 1},
	{ID: "C2.3", Lab: "2", Keys: []string{"L2.6"}, Kind: kindJarText, Desc: "palette has the kitControl folder with Add",
		Jar: trainingRT, Glob: "module.palette", Contains: []string{`n="kitControl"`, `n="Add"`}},
	{ID: "C2.4", Lab: "2", Keys: []string{"L2.1"}, Kind: kindJarText, Desc: "module.xml registers Add",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{`name="Add"`}},

	// Lab 3. LAB/Lab3 Add Part 2/BAdd.java:18-45 (same slots); changed() writes out (:49-106),
	// so a stored out proves the callback ran. Links per Lab Instructions LI:560-576. C3.5 does
	// not link L3.9 (Output = Point1 + Point2 + Point3): a stored Add.out says nothing of the
	// Output point, which the live TrainingStation does not even have (C3.4).
	{ID: "C3.1", Lab: "3", Keys: []string{"L3.5"}, Kind: kindLink, Desc: "NumericWritable.out -> Add.in1",
		From: numericWritable, FromSlot: "out", Type: "training:Add", Slot: "in1", Op: opMin, N: 1},
	{ID: "C3.2", Lab: "3", Keys: []string{"L3.5"}, Kind: kindLink, Desc: "NumericWritable.out -> Add.in2",
		From: numericWritable, FromSlot: "out", Type: "training:Add", Slot: "in2", Op: opMin, N: 1},
	{ID: "C3.3", Lab: "3", Keys: []string{"L3.6"}, Kind: kindLink, Desc: "NumericWritable.out -> Add.in3 (link dialog)",
		From: numericWritable, FromSlot: "out", Type: "training:Add", Slot: "in3", Op: opMin, N: 1},
	{ID: "C3.4", Lab: "3", Keys: []string{"L3.7"}, Kind: kindLink, Desc: "Add.out -> NumericWritable.in10 (Output)",
		From: "training:Add", FromSlot: "out", Type: numericWritable, Slot: "in10", Op: opMin, N: 1},
	{ID: "C3.5", Lab: "3", Keys: []string{"L3.8"}, Kind: kindSlot, Desc: "Add computed a sum (out stored)",
		Type: "training:Add", Slot: "out", Set: true},

	// Lab 4. BTstat.java:35-44 diff default 0, action default BLoopAction.DEFAULT;
	// BLoopAction.java:17 default "direct". kitControl BTstat nullOnInControl default true
	// (organized/kitControl/kitControl-rt/pipeline/procyon/com/tridium/kitControl/hvac/BTstat.java:24).
	{ID: "C4.1", Lab: "4", Keys: []string{"L4.6"}, Kind: kindType, Desc: "a training Tstat (My Tstat)",
		Type: "training:Tstat", Op: opMin, N: 1},
	{ID: "C4.2", Lab: "4", Keys: []string{"L4.6"}, Kind: kindType, Desc: "a kitControl Tstat (Reference Tstat)",
		Type: "kitControl:Tstat", Op: opMin, N: 1},
	{ID: "C4.3", Lab: "4", Keys: []string{"L4.10"}, Kind: kindLink, Desc: "NumericWritable.out -> Tstat.cv",
		From: numericWritable, FromSlot: "out", Type: "training:Tstat", Slot: "cv", Op: opMin, N: 1},
	{ID: "C4.4", Lab: "4", Keys: []string{"L4.10"}, Kind: kindLink, Desc: "NumericWritable.out -> Tstat.sp",
		From: numericWritable, FromSlot: "out", Type: "training:Tstat", Slot: "sp", Op: opMin, N: 1},
	{ID: "C4.5", Lab: "4", Keys: []string{"L4.13"}, Kind: kindSlot, Desc: "training Tstat diff = 2",
		Type: "training:Tstat", Slot: "diff", Want: "2.0", Default: "0.0"},
	{ID: "C4.6", Lab: "4", Keys: []string{"L4.15"},
		Unobserved: map[string]string{"L4.15": "the outputs at 72.5 (both false, held in band)"}, Kind: kindSlot, Desc: "training Tstat action = reverse",
		Type: "training:Tstat", Slot: "action", Want: "reverse", Default: "direct"},
	{ID: "C4.7", Lab: "4", Keys: []string{"L4.7"}, Kind: kindSlot, Desc: "Reference Tstat nullOnInControl = false",
		Type: "kitControl:Tstat", Slot: "nullOnInControl", Want: "false", Default: "true"},

	// Lab 5: nothing in the station. jarsigner names the signature files after the alias
	// CodeSigning, upper-cased and cut to 8 characters: META-INF/CODESIGN.SF (L5.13-L5.14).
	{ID: "C5.1", Lab: "5", Keys: []string{"L5.21", "L5.24"}, Kind: kindJarSigned, Desc: "training-rt is signed", Jar: trainingRT},
	{ID: "C5.2", Lab: "5", Keys: []string{"L5.24"}, Kind: kindJarSigned, Desc: "training-wb is signed", Jar: "training-wb"},
	{ID: "C5.3", Lab: "5", Keys: []string{"L5.13", "L5.14"}, Kind: kindJarEntry, Desc: "signed with the CodeSigning alias",
		Jar: trainingRT, Glob: "META-INF/CODESIGN.SF", Op: opMin, N: 1},

	// Lab 6. BWorkOrderServiceWithSerialization.java:55-59 lastSuccess default BAbsTime.DEFAULT;
	// BIRestrictedComponent: one instance, under Services (L6.7).
	{ID: "C6.1", Lab: "6", Keys: []string{"L6.16", "L6.17"}, Kind: kindType, Desc: "one WorkOrderService, under Services",
		Type: "training:WorkOrderService", Within: services, Op: opEq, N: 1},
	{ID: "C6.2", Lab: "6", Keys: []string{"L6.23", "L6.33"}, Kind: kindSlot, Desc: "a submit succeeded (lastSuccess stored)",
		Type: "training:WorkOrderService", Slot: "lastSuccess", Set: true},
	{ID: "C6.3", Lab: "6", Keys: []string{"L6.9"}, Kind: kindJarText, Desc: "training-rt depends on net-rt",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{`name="net-rt"`}},
	{ID: "C6.4", Lab: "6", Keys: []string{"L6.21"}, Kind: kindJarText, Desc: "NETWORK_COMMUNICATION requested",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{"<name>NETWORK_COMMUNICATION</name>"}},
	{ID: "C6.5", Lab: "6", Keys: []string{"L6.31"}, Kind: kindJarText, Desc: "REFLECTION requested",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{"<name>REFLECTION</name>"}},
	{ID: "C6.6", Lab: "6", Keys: []string{"L6.28"}, Kind: kindJarEntry, Desc: "jackson-databind in the uberjar",
		Jar: trainingRT, Glob: "META-INF/maven/com.fasterxml.jackson.core/jackson-databind/pom.properties", Op: opMin, N: 1},

	// Lab 7. BWorkOrderServiceWithSerialization.java:38-41 url default
	// "http://localhost:3001/workOrders": restored after the bad_url experiment (L7.16-L7.18).
	{ID: "C7.1", Lab: "7", Keys: []string{"L7.7", "L7.8"}, Kind: kindJar, Desc: "training-rtTest jar built", Jar: "training-rtTest"},
	{ID: "C7.2", Lab: "7", Keys: []string{"L7.1", "L7.2"}, Kind: kindJarEntry, Desc: "BWorkOrderServiceTest.class in training-rtTest",
		Jar: "training-rtTest", Glob: "test/com/tridiumuniversity/training/workOrder/BWorkOrderServiceTest.class", Op: opMin, N: 1},
	{ID: "C7.3", Lab: "7", Keys: []string{"L7.3"}, Kind: kindJarEntry, Desc: "Mockito in the test uberjar",
		Jar: "training-rtTest", Glob: "org/mockito/**", Op: opMin, N: 1},
	{ID: "C7.4", Lab: "7", Keys: []string{"L7.13"}, Kind: kindJarEntry, Desc: "training-rt.lexicon in training-rt",
		Jar: trainingRT, Glob: "training-rt.lexicon", Op: opMin, N: 1},
	{ID: "C7.5", Lab: "7", Keys: []string{"L7.18"},
		Unobserved: map[string]string{"L7.18": "faultCause cleared, the fault bit and the {ok} status"}, Kind: kindSlot, Desc: "WorkOrderService url restored",
		Type: "training:WorkOrderService", Slot: "url", Want: "http://localhost:3001/workOrders",
		Default: "http://localhost:3001/workOrders"},

	// Lab 9. BWorkerThreadExecutor.java:28-31 worker = new BTrainingWorker() (a child component).
	{ID: "C9.1", Lab: "9", Keys: []string{"L9.8"}, Kind: kindType, Desc: "a ClientThreadExecutor",
		Type: "training:ClientThreadExecutor", Op: opMin, N: 1},
	{ID: "C9.2", Lab: "9", Keys: []string{"L9.8"}, Kind: kindType, Desc: "a DedicatedThreadExecutor",
		Type: "training:DedicatedThreadExecutor", Op: opMin, N: 1},
	{ID: "C9.3", Lab: "9", Keys: []string{"L9.8"}, Kind: kindType, Desc: "an EngineThreadExecutor",
		Type: "training:EngineThreadExecutor", Op: opMin, N: 1},
	{ID: "C9.4", Lab: "9", Keys: []string{"L9.8"}, Kind: kindType, Desc: "an ExecutorServiceExecutor",
		Type: "training:ExecutorServiceExecutor", Op: opMin, N: 1},
	{ID: "C9.5", Lab: "9", Keys: []string{"L9.8", "L9.14"}, Kind: kindType, Desc: "a TrainingWorker inside a WorkerThreadExecutor",
		Type: "training:TrainingWorker", Within: "training:WorkerThreadExecutor", Op: opMin, N: 1},
	{ID: "C9.6", Lab: "9", Keys: []string{"L9.8"}, Kind: kindType, Desc: "a kitControl CurrentTime",
		Type: "kitControl:CurrentTime", Op: opMin, N: 1},
	{ID: "C9.7", Lab: "9", Keys: []string{"L9.6"}, Kind: kindJarText, Desc: "MANAGE_EXECUTION requested",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{"<name>MANAGE_EXECUTION</name>"}},

	// Lab 10. BHistoryExt enabled default false
	// (organized/history/history-rt/decompiled/javax/baja/history/ext/BHistoryExt.java:106).
	// The lab copies only the point, "Keep all links" (LI:1934-1936): one Ramp feeds all ten
	// points, so C10.2 wants one Ramp, not ten (the live station has one Ramp per point).
	{ID: "C10.1", Lab: "10", Keys: []string{"L10.4", "L10.22"},
		Unobserved: map[string]string{"L10.22": "the Success state and the job log lines"}, Kind: kindType, Desc: "one HistoriesToCsvService, under Services",
		Type: "training:HistoriesToCsvService", Within: services, Op: opEq, N: 1},
	{ID: "C10.2", Lab: "10", Keys: []string{"L10.16"}, Kind: kindType, Desc: "a kitControl Ramp",
		Type: "kitControl:Ramp", Op: opMin, N: 1},
	{ID: "C10.3", Lab: "10", Keys: []string{"L10.17", "L10.19"},
		Unobserved: map[string]string{"L10.19": "the copy names NumericWritable1-9 and that they share the one Ramp"}, Kind: kindType, Desc: "ten NumericCov history extensions",
		Type: "history:NumericCovHistoryExt", Op: opMin, N: 10},
	{ID: "C10.4", Lab: "10", Keys: []string{"L10.16", "L10.19"},
		Unobserved: map[string]string{"L10.19": "the copy names NumericWritable1-9 and that they share the one Ramp"}, Kind: kindLink, Desc: "Ramp.out -> NumericWritable.in16, ten times",
		From: "kitControl:Ramp", FromSlot: "out", Type: numericWritable, Slot: "in16", Op: opMin, N: 10},
	{ID: "C10.5", Lab: "10", Keys: []string{"L10.17", "L10.20"}, Kind: kindSlot, Desc: "every NumericCov enabled",
		Type: "history:NumericCovHistoryExt", Slot: "enabled", Want: "true", Default: "false"},
	{ID: "C10.6", Lab: "10", Keys: []string{"L10.23"}, Kind: kindFiles, Desc: "ten exported CSV files in the Station Home",
		Glob: csvGlob, Op: opEq, N: 10},
	{ID: "C10.7", Lab: "10", Keys: []string{"L10.6", "L10.7", "L10.15"}, Kind: kindJarText, Desc: "depends on history-rt, file-rt, bql-rt",
		Jar: trainingRT, Glob: trainingXML, Contains: []string{`name="history-rt"`, `name="file-rt"`, `name="bql-rt"`}},
	// L10.24 (open one CSV): every exported file is a trend CSV as the live TrainingStation wrote it
	// (shared/TrainingStation_NumericWritable1.csv, 2026-10-15): a BOM, the line history:<station>/<point>,
	// a blank line, the header Timestamp,Trend Flags,Status,Value, then one row per record with the
	// locale's decimal comma ("82,9" under es-MX). The file is the whole answer: no Unobserved note.
	{ID: "C10.8", Lab: "10", Keys: []string{"L10.24"}, Kind: kindFileText, Desc: "CSV line 1 names the history",
		Glob: csvGlob, Line: 1, Match: `^history:{station}/NumericWritable\d*$`},
	{ID: "C10.9", Lab: "10", Keys: []string{"L10.24"}, Kind: kindFileText, Desc: "CSV header on line 3",
		Glob: csvGlob, Line: 3, Match: `^Timestamp,Trend Flags,Status,Value$`},
	{ID: "C10.10", Lab: "10", Keys: []string{"L10.24"}, Kind: kindFileText, Desc: "CSV rows: timestamp, trend flags, status, value",
		Glob: csvGlob, Match: `^\d{1,2} \S+ \d{4} \d{2}:\d{2}:\d{2} \S+,\{[^}]*\},\{[^}]*\},"?-?\d+(?:[.,]\d+)?"?$`, Op: opMin, N: 1},

	// Lab 11. Intersections A-Q (L11.8): 17 server devices, four lights each.
	{ID: "C11.1", Lab: "11", Keys: []string{"L11.7"},
		Unobserved: map[string]string{"L11.7": "the Px view with the lights of every intersection"}, Kind: kindType, Desc: "one TrafficLightServerNetwork",
		Type: "trafficLightServer:TrafficLightServerNetwork", Op: opEq, N: 1},
	{ID: "C11.2", Lab: "11", Keys: []string{"L11.8"}, Kind: kindType, Desc: "17 server devices (intersections A-Q)",
		Type: "trafficLightServer:TrafficLightServerDevice", Op: opEq, N: 17},
	{ID: "C11.3", Lab: "11", Keys: []string{"L11.8"}, Kind: kindType, Desc: "68 server proxy extensions (4 lights each)",
		Type: "trafficLightServer:TrafficLightServerProxyExt", Op: opEq, N: 68},
	{ID: "C11.4", Lab: "11", Keys: []string{"L11.24"}, Kind: kindType, Desc: "one TrafficLightDriverNetwork, under Drivers",
		Type: "trafficLightDriver:TrafficLightDriverNetwork", Within: "driver:DriverContainer", Op: opEq, N: 1},
	{ID: "C11.5", Lab: "11", Keys: []string{"L11.31"}, Kind: kindType, Desc: "a driver device",
		Type: "trafficLightDriver:TrafficLightDriverDevice", Op: opMin, N: 1},
	{ID: "C11.6", Lab: "11", Keys: []string{"L11.40"}, Kind: kindType, Desc: "a driver proxy point",
		Type: "trafficLightDriver:TrafficLightDriverProxyExt", Op: opMin, N: 1},
	{ID: "C11.7", Lab: "11", Keys: []string{"L11.22"}, Kind: kindJarText, Desc: "driver requests NETWORK_COMMUNICATION",
		Jar: "trafficLightDriver-rt", Glob: trainingXML, Contains: []string{"<name>NETWORK_COMMUNICATION</name>"}},
	{ID: "C11.8", Lab: "11", Keys: []string{"L11.3", "L11.5"}, Kind: kindJarSigned, Desc: "trafficLightServer-rt is signed",
		Jar: "trafficLightServer-rt"},

	// Lab 11, nested slots (dotted paths below a composite slot, read from Comp.Nested). Defaults:
	// BTcpCommConfig address = new BIpAddress("local", 0), serverEnabled false
	// (organized/ndriver/ndriver-rt/decompiled/com/tridium/ndriver/datatypes/BTcpCommConfig.java:41-42);
	// BPingHealth down false, lastOkTime BAbsTime.DEFAULT
	// (organized/driver/driver-rt/decompiled/javax/baja/driver/ping/BPingHealth.java:42-44). The
	// server's palette entry sets 127.0.0.1:23 and serverEnabled (LAB/Lab11 NDriver/
	// trafficLightServer.zip!trafficLightServer/trafficLightServer-rt/module.palette:7-13); the
	// driver is pointed at localhost:23 (Lab Instructions LI:2323-2324).
	{ID: "C11.9", Lab: "11", Keys: []string{"L11.25"}, Kind: kindSlot, Desc: "driver Tcp Config address = localhost",
		Type: "trafficLightDriver:TrafficLightDriverTcpCommConfig", Slot: "address.ipAddress", Want: "localhost", Default: "local"},
	{ID: "C11.10", Lab: "11", Keys: []string{"L11.25"}, Kind: kindSlot, Desc: "driver Tcp Config port = 23",
		Type: "trafficLightDriver:TrafficLightDriverTcpCommConfig", Slot: "address.port", Want: "23", Default: "0"},
	{ID: "C11.11", Lab: "11", Keys: []string{"L11.26"},
		Unobserved: map[string]string{"L11.26": "the {ok} status (the bog reader keeps no status slot) and that the ping was just now"}, Kind: kindSlot, Desc: "network pinged ok (health.lastOkTime stored)",
		Type: "trafficLightDriver:TrafficLightDriverNetwork", Slot: "health.lastOkTime", Set: true},
	{ID: "C11.12", Lab: "11", Keys: []string{"L11.26"},
		Unobserved: map[string]string{"L11.26": "the {ok} status (the bog reader keeps no status slot) and that the ping was just now"}, Kind: kindSlot, Desc: "network pinged and not down (health.down = false, lastOkTime stored)",
		Type: "trafficLightDriver:TrafficLightDriverNetwork", Slot: "health.down", Want: "false", Default: "false",
		Stored: []string{"health.lastOkTime"}},
	{ID: "C11.13", Lab: "11", Keys: []string{"L11.32"},
		Unobserved: map[string]string{"L11.32": "the {ok} status (the bog reader keeps no status slot)"}, Kind: kindSlot, Desc: "every driver device pinged ok (health.lastOkTime stored)",
		Type: "trafficLightDriver:TrafficLightDriverDevice", Slot: "health.lastOkTime", Set: true},
	{ID: "C11.14", Lab: "11", Keys: []string{"L11.32"},
		Unobserved: map[string]string{"L11.32": "the {ok} status (the bog reader keeps no status slot)"}, Kind: kindSlot, Desc: "every driver device pinged and not down",
		Type: "trafficLightDriver:TrafficLightDriverDevice", Slot: "health.down", Want: "false", Default: "false",
		Stored: []string{"health.lastOkTime"}},
	{ID: "C11.15", Lab: "11", Keys: []string{"L11.7"},
		Unobserved: map[string]string{"L11.7": "the Px view with the lights of every intersection"}, Kind: kindSlot, Desc: "server Tcp Config address = 127.0.0.1",
		Type: "trafficLightServer:TrafficLightServerTcpCommConfig", Slot: "address.ipAddress", Want: "127.0.0.1", Default: "local"},
	{ID: "C11.16", Lab: "11", Keys: []string{"L11.7"},
		Unobserved: map[string]string{"L11.7": "the Px view with the lights of every intersection"}, Kind: kindSlot, Desc: "server Tcp Config port = 23",
		Type: "trafficLightServer:TrafficLightServerTcpCommConfig", Slot: "address.port", Want: "23", Default: "0"},
	{ID: "C11.17", Lab: "11", Keys: []string{"L11.7"},
		Unobserved: map[string]string{"L11.7": "the Px view with the lights of every intersection"}, Kind: kindSlot, Desc: "server Tcp Config serverEnabled = true",
		Type: "trafficLightServer:TrafficLightServerTcpCommConfig", Slot: "serverEnabled", Want: "true", Default: "false"},

	// Bonus lab. BTrafficLightDriverDevice.java (LAB/Lab11 NDriver/4 ...):64-67 intersectionId
	// default "A"; discovered devices carry the other ids.
	{ID: "CB.1", Lab: "B", Keys: []string{"LB.18"}, Kind: kindSlot, Desc: "devices added by discovery (distinct intersection ids)",
		Type: "trafficLightDriver:TrafficLightDriverDevice", Slot: "intersectionId", Default: "A", Distinct: true, Op: opMin, N: 2},
	{ID: "CB.2", Lab: "B", Keys: []string{"LB.16"}, Kind: kindJarEntry, Desc: "device discovery leaf in trafficLightDriver-rt",
		Jar: "trafficLightDriver-rt", Glob: "com/tridiumuniversity/trafficLightDriver/learn/BTrafficLightDriverDeviceDiscoveryLeaf.class", Op: opMin, N: 1},
	{ID: "CB.3", Lab: "B", Keys: []string{"LB.19"}, Kind: kindJarEntry, Desc: "point discovery leaf in trafficLightDriver-rt",
		Jar: "trafficLightDriver-rt", Glob: "com/tridiumuniversity/trafficLightDriver/point/BTrafficLightDriverPointDiscoveryLeaf.class", Op: opMin, N: 1},
	{ID: "CB.4", Lab: "B", Keys: []string{"LB.8"}, Kind: kindJarEntry, Desc: "point manager in trafficLightDriver-wb",
		Jar: "trafficLightDriver-wb", Glob: "**/BTrafficLightDriverPointManager*.class", Op: opMin, N: 1},
}
