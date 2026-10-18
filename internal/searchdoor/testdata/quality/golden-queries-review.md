# Golden query fixture review (T15d)

Date: 2026-10-16. Fixture: `tools/tests/fixtures/golden-queries.jsonl` (70 questions at review; 81 since round 3 T4b, no version
field and no changelog convention; this file is the changelog for this review).

## Method

1. Ran `python3 tools/searchdoor/golden.py --misses` on the original fixture and dumped the top 10
   ranked items of every missed `(question, source)` pair.
2. For each miss, opened the expected location and the candidate locations (official docs under
   `niagara-help/` for the 4.14.0.162 root corpus, decompiled classes under
   `organized/*/vineflower/**`, corpus blocks via `corpus-nav.py show`) and checked them against the
   question and against the corpus block the row already expects.
3. Added a location only when its text or members answer the question (one evidence line per added
   location below). A returned candidate that does not answer the question was not added.
4. Removed an expectation only when the file has no text or member about the mechanism the
   question asks for (grep of the question's distinguishing terms returns no real hit) and the
   row's own corpus block does not cite it for that mechanism. Every removal is paired with a
   verified replacement, so no row lost its last location for a source.
5. Questions were not rewritten; rows that already hit were not touched.

Matching rules that shape the entries (from `tools/searchdoor/golden.py`): blocks by exact label,
docs by path prefix, code by class stem. A stem such as `BColor` or `LicenseUtil` also matches
same-named classes in other modules; the evidence names the class that was verified.

Evidence format: `<qid> <source> <location> — "<quote or member>" (<file:line>)`. Code paths are
relative to `organized/`, doc paths to `niagara-help/`.

## Summary

| | blocks | docs | code | total |
|---|---|---|---|---|
| Locations added | 2 | 29 | 52 | 83 |
| Locations removed | 0 | 5 | 9 | 14 |

Miss rows reviewed: 39 (49 missed pairs). Rows changed: 36. Rows unchanged: q01, q06, q57 (and the
blocks pairs of q38 and q41, whose rows changed in another source).

| Source | hit@1 before | hit@5 before | hit@1 after | hit@5 after |
|---|---|---|---|---|
| blocks | 54/70 | 65/70 | 54/70 | 66/70 |
| docs | 27/60 | 44/60 | 37/60 | 59/60 |
| code | 10/51 | 23/51 | 25/51 | 43/51 |
| overall hit@5 | | 132/181 | | 168/181 |

The "after" numbers measure the widened fixture with the same search code; they are not a ranking
improvement. Six docs rows (q38, q39, q40, q45 via `docs-text/docPlatform.txt`; q44, q70 via
`docs-text/docUser.txt`) now hit at rank 1 through one of two broad manuals whose verified section
answers the question. Code numbers depend on the module-navigator rank cache, which another writer
was changing during this review.

## Remaining misses after the review

| Row | Source | Rank | Kind |
|---|---|---|---|
| q06 | blocks | not returned (corpus-nav test: 88) | recall |
| q38 | blocks | 6 | ranking |
| q41 | blocks | 35 | ranking |
| q57 | blocks | 24 | ranking |
| q58 | docs | not returned | recall |
| q01 | code | not returned | recall |
| q15 | code | 10 | ranking |
| q18 | code | 12 | ranking |
| q27 | code | not returned | recall |
| q37 | code | not returned | recall |
| q39 | code | 7 | ranking |
| q41 | code | not returned | recall |
| q67 | code | 8 | ranking |

## Rows

### q01 — code (unchanged)
Verdict: expectation right, real recall miss. `BAlarmService.doAckAlarm` sets `BAckState.acked`
(`alarm/alarm-rt/vineflower/javax/baja/alarm/BAlarmService.java:559-572`). The returned `BAckState`
only lists the three states and `BAckAlarmCommand`/`AckAlarmResult` are cloudLink transport, so
nothing was added.

### q03 — code
- q03 code bacnetAlarmRouter/bacnetAlarmRouter-rt/vineflower/com/tridiumemea/addons/bacnet/alarmServiceFilter/BEscalationFilter.java — "BString esc = (BString)record.getAlarmData().get("escalated")" in `doRouteAlarm`, a filter that forwards alarms by escalation level (BEscalationFilter.java:30,57)

### q05 — code
- q05 code email/email-rt/vineflower/com/tridium/email/alarm/BEmailRecipient.java — "public void handleAlarm(BAlarmRecord alarm) { this.sendAlarm(this.getTo(), alarm);" then `BEmailService` lookup and `account.send(mail)` (BEmailRecipient.java:136-161)

### q06 — blocks (unchanged)
Verdict: expectation right, real recall miss. The returned B351, B278, B274, B282, B123 are about
the e-signature trail, BACnet trend export, COV, BACnet trend records and one live station, not
the history service.

### q10 — code
- q10 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/ObjectTypeList.java — "new ObjectTypeList(BOrd.make("module://bacnet/com/tridium/bacnet/objectTypes.xml"))", each `<property>` row parsed into a `PropertyInfo` (ObjectTypeList.java:43-55,148)
- q10 code bacnet/bacnet-wb/vineflower/com/tridium/bacnet/ui/point/PointLearn.java — "static boolean isMultiStatePresentValue(int propertyId, int objectType)" / "static boolean isBinaryPv(PropertyInfo info)", the Numeric/Boolean/Enum decision B271 §271.6 cites (PointLearn.java:167-202)
- Removed q10 code BBacnetNetwork — no objectType, XML or point-type code; its only discovery code submits the device job `BBacnetDiscoverDevicesJob` (BBacnetNetwork.java:366-367)

### q11 — code
- q11 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/asn/AsnOutputStream.java — "public class AsnOutputStream extends ByteArrayOutputStream implements AsnOutput, AsnConst", `writeBoolean(int contextTag, boolean value)` (AsnOutputStream.java:23,59)
- q11 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/asn/AsnInputStream.java — "public boolean readBoolean(int contextTag) throws AsnException" (AsnInputStream.java:24,189)
- q11 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/stack/transport/ApplicationPdu.java — "int pduType = (rawApdu.read() & 240) >> 4;" APDU type dispatch (ApplicationPdu.java:142-170)
- Removed q11 code BBacnetNetwork — no ASN.1 or APDU encoding; its only APDU reference is the `maxAPDULengthAccepted` argument of a device-registration call (BBacnetNetwork.java:487)

### q12 — code
- q12 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/stack/link/ip/BBacnetIpLinkLayer.java — "public static final Property udpPort = newProperty(0, "0xBAC0", null);" plus `ipAddress`, `bbmdAddress` (BBacnetIpLinkLayer.java:173,196)
- q12 code bacnet/bacnet-rt/vineflower/javax/baja/bacnet/BBacnetDevice.java — `@NiagaraProperty(name = "address", type = "BBacnetAddress")` and "public final void setObjectId(BBacnetObjectIdentifier objectId, Context cx)" (BBacnetDevice.java:101,404)
- Kept BBacnetNetwork: `lookupDeviceById` / `lookupDeviceByAddress` resolve devices by instance and address (BBacnetNetwork.java:146-149).

### q14 — code
- q14 code bacnet/bacnet-rt/vineflower/com/tridium/bacnet/stack/link/sc/BScLinkLayer.java — "public final class BScLinkLayer extends BBacnetLinkLayer" with `vmacAddress`, `credentials`, `webSocketInitiator`, `hubConnector` (BScLinkLayer.java:56-94)
- Removed q14 code BBacnetNetwork — no BACnet/SC, hub, TLS or WebSocket code (the grep hits are `descendantsStarted` and device discovery)

### q15 — code
- q15 code fox/fox-rt/vineflower/com/tridium/fox/session/FoxFrame.java — "SYNC = 115; ASYNC = 97; REPLY = 114; ERROR = 101; NULL = 110; KEEPALIVE = 107; CLOSE = 99" and `read` consuming "fox " (FoxFrame.java:13-19,66-97)
- q15 code fox/fox-rt/vineflower/com/tridium/fox/session/FoxSession.java — "FoxFrame readFrame() throws IOException", `writeFrame(new FoxFrame(97, ...))` (FoxSession.java:615,893)
- Kept BFoxService: it owns `keepAliveInterval` and the frame trace switches (BFoxService.java:256-264).

### q16 — code
- q16 code fox/fox-rt/vineflower/com/tridium/fox/util/FoxScramShaUtil.java — "public static String handleScramServerCallback(FoxSession session, ScramServerCallback callback, Type schemeType)" and `handleClientAuthentication` with `authInputScram` (FoxScramShaUtil.java:39,161-167)
- q16 code fox/fox-rt/vineflower/com/tridium/fox/session/Tuner.java — "String method = unknownUser ? "n4digest" : "foxUnsupported";" and the hello/challenge tuning (Tuner.java:108-166,547)
- Kept BFoxService: its tuning listener receives `connectionAuthenticated` (BFoxService.java:1036).

### q18 — code
- q18 code backup/backup-rt/vineflower/com/tridium/backup/BBackupChannel.java — "if (!service.getPermissions(this.getSessionContext()).has(48))" then `service.zip(null, out, true, null)` (BBackupChannel.java:79,109)

### q22 — docs
- q22 docs devguide-clean/servlets/webModules.txt — "The Web Server currently being used is Jetty. Jetty is built around the standard Java Servlet Specification." (webModules.txt:8)
- q22 docs devguide-clean/web.txt — "javax.servlet.Filter: Unsupported interface / javax.servlet.FilterChain: Unsupported interface" in the servlet section (web.txt:20-27)
- q22 docs bajadoc-clean/javax/baja/web/filters/ — both pages read: CsrfProtectedFilter "configure this filter in web.xml of the web module" and UserActivityFilter "configure this filter on your servlet in your web.xml file" (CsrfProtectedFilter.txt:9, UserActivityFilter.txt:9)

### q25 — docs and code
- q25 docs devguide-clean/ui/introUI.txt — "bajaux is Niagara’s JavaScript-based UI component framework, natively targeting the browser." (introUI.txt:78-82)
- q25 docs devguide-clean/jsdoc/bajaux-ux/module-bajaux_spandrel.txt — "The purpose of spandrel is to provide a reasonably pure-functional, diffable method of defining a nested structure of bajaux Widgets" (module-bajaux_spandrel.txt:84)
- q25 docs devguide-clean/jsdoc/bajaux-ux/tutorial-50-spandrel.txt — "Building Composite Widgets With spandrel ... The spandrel API is new in Niagara 4.10." (tutorial-50-spandrel.txt:80)
- Removed q25 docs devguide-clean/bajaui.txt — the Java `bajaui` toolkit page has no bajaux or spandrel text; introUI.txt:60 states "bajaui code only works in Workbench", the other framework
- q25 code bajaux/bajaux-rt/vineflower/javax/baja/bajaux/BIJavaScriptWidget.java — "public interface BIJavaScriptWidget extends BIJavaScript", `forType` resolves the widget agent; the rt→web bridge of B204 §204.5 (BIJavaScriptWidget.java:12,15)
- q25 code bajaux/bajaux-rt/vineflower/com/tridium/ux/WbWebWidgetServlet.java — "public final class WbWebWidgetServlet extends HttpServlet", `doGet` emits the `#bajaux-widget` mount (WbWebWidgetServlet.java:50,59,185)
- Kept `Widget.js`: it is the bajaux Widget, but the code source indexes Java only, so it cannot be returned.

### q26 — code
- q26 code baja/baja/vineflower/javax/baja/rpc/NiagaraRpc.java — "public @interface NiagaraRpc { Transport[] transports();" (NiagaraRpc.java:10-11)
- q26 code web/web-rt/vineflower/javax/baja/web/servlets/NiagaraRpcServlet.java — "rpcPathPattern = Pattern.compile("/([^/]+)/(.+)")" in `doPost` (NiagaraRpcServlet.java:29-31)
- Removed q26 code BWebServlet — zero occurrences of "rpc" in the class

### q27 — docs and code
B179 §179.1: Px has no native dropdown; the closest pattern is the Popup binding.
- q27 docs guides-clean/Graphics/AboutPopupBindings.txt — "This type of binding may be used to open a Px view in an additional popup window" (AboutPopupBindings.txt:7-9)
- q27 docs guides-clean/Graphics/UsingthePopupBinding.txt — "You add a Popup Binding to an object that serves as a hyperlink." (UsingthePopupBinding.txt:7)
- q27 docs docs-text/docGraphics.txt — the same Popup Binding sections in the Graphics guide ("Hyperlinks with Popup Bindings (an example)") (docGraphics.txt:1114-1123)
- Removed q27 docs guides-clean/Graphics — a directory prefix (no trailing slash) that accepted any of the Graphics pages, such as bajaui-RadioButton.txt; replaced by the two pages above
- q27 code kitPx/kitPx-wb/vineflower/com/tridium/kitpx/BPopupBinding.java — "public final class BPopupBinding extends BBinding" with `title`, `position`, `size`, `modal` (BPopupBinding.java:33-54)
- Removed q27 code BPxEditor — no dropdown, popup or combo code; its only menu code is the Workbench `getViewMenus` (BPxEditor.java:60,116)

### q29 — code
- q29 code gx/gx-rt/vineflower/javax/baja/gx/BColor.java — "public String encodeToString()" / "public static BColor make(String s)" (BColor.java:216,314); B183 §183.2
- q29 code gx/gx-rt/vineflower/javax/baja/gx/BFont.java — "public static BFont make(String s)", `out.writeUTF(this.encodeToString())` (BFont.java:70,185); B183 §183.3

### q33 — code
- q33 code control/control-rt/vineflower/javax/baja/control/WritableSupport.java — "abstract class WritableSupport", `in16()`, override timer and fallback arbitration (WritableSupport.java:22,60-80,128,158)
- q33 code control/control-rt/vineflower/javax/baja/control/BNumericWritable.java — `name = "in16"`, `"fallback"`, `"emergencyOverride"`, `"override"`, `"auto"` (BNumericWritable.java:103-129)
- q33 code control/control-rt/vineflower/javax/baja/control/BBooleanWritable.java — 16 `in` slots, `name = "in16"` (BBooleanWritable.java:105)
- q33 code control/control-rt/vineflower/javax/baja/control/BEnumWritable.java — 16 `in` slots, `name = "in16"` (BEnumWritable.java:104)
- q33 code control/control-rt/vineflower/javax/baja/control/BStringWritable.java — 16 `in` slots, `name = "in16"` (BStringWritable.java:102)
- q33 code control/control-rt/vineflower/javax/baja/control/enums/BPriorityLevel.java — range "none, level_1 ... level_16, fallback" (BPriorityLevel.java:14,51-52)
- Kept BControlPoint: it delegates to `writableSupport()` (BControlPoint.java:358-379).

### q36 — code
- q36 code kitControl/kitControl-rt/vineflower/com/tridium/kitControl/enums/BReliability.java — range "noFaultDetected, noSensor, overRange, underRange, openLoop, shortedLoop, ..." (BReliability.java:12-14)
- Kept BLoopPoint: B547 lists it as the consumer of the kitControl enums `BLoopAction` and `BDisableAction`.

### q37 — code
- q37 code program/program-wb/vineflower/com/tridium/program/ui/Compiler.java — "public void compile(String className, BCode code, String source)", `this.exec(cmd)`, "Problem loading 'javac' command." (Compiler.java:63,109,115)
- Removed q37 code BProgram — no javac or compile code in the class; B426 (the row's block) states "program-rt.jar has zero references to javac/tools.jar/JavaCompiler" and places compilation in program-wb (niagara-mental-model-bloque426.md:27)

### q38 — docs (blocks unchanged)
- q38 docs docs-text/docPlatform.txt — "The Niagara license file is a structured XML file ... All contents (lines) in between are <feature> elements, plus one signature element." (docPlatform.txt:2003-2020)
- Removed q38 docs docs-text/docModuleSign.txt — the Third Party Module Signing guide; its only two "license" hits are the legal notice (docModuleSign.txt:5,16-18)
- Blocks: expectation right, ranking miss (B483 at 6). The returned B139 describes a third-party module's own XML license, not the Niagara license, so it was not added.

### q39 — docs and code
- q39 docs docs-text/docPlatform.txt — "This ending element contains a digital <signature> that is created when the license file is generated. It prevents tampering" (docPlatform.txt:2110-2118). Partial: it names what the signer emits and the verifier checks; the byte-level procedure is only in B323.
- Removed q39 docs docs-text/docModuleSign.txt — module JAR signing (Jar Signer Tool), not license signing; B323 never mentions module signing
- q39 code baja/baja/vineflower/com/tridium/sys/license/LicenseUtil.java — "public static byte[] encode(XElem elem)" (canonical re-encode) and "public static boolean verify(byte[] data, byte[] sig, PublicKey publicKey, String signatureAlgorithm)", default "DSA" (LicenseUtil.java:645,692-707)
- Removed q39 code LicenseManager — a 9-line interface with only `getFeature`, `checkFeature`, `getFeatures` (LicenseManager.java:3-8)

### q40 — docs
- q40 docs docs-text/docPlatform.txt — "When one of these resources goes over a licensed limit, the resource either goes into fatal fault or becomes inactive." with `point.limit="1250" device.limit="50"` (docPlatform.txt:5528-5550)
- q40 docs guides-clean/Platform/CheckingCapacityLicensingStatus-2DBFB22B.txt — "globalCapacity.resource statuses show the current counts along with respective license limits" (CheckingCapacityLicensingStatus-2DBFB22B.txt:19-22)
- Removed q40 docs docs-text/docN4Install.txt — covers license installation and confirmation only (docN4Install.txt:164,502-506); no limit, count or enforcement text

### q41 — code (blocks unchanged)
B301 §301.1: one feature per Modbus network class; modbusCore has none.
- q41 code modbusAsync/modbusAsync-rt/vineflower/com/tridium/modbusAsync/BModbusAsyncNetwork.java — "getFeature("tridium", "modbusAsync")" (BModbusAsyncNetwork.java:183)
- q41 code modbusTcp/modbusTcp-rt/vineflower/com/tridium/modbusTcp/BModbusTcpNetwork.java — "getFeature("tridium", "modbusTcp")" (BModbusTcpNetwork.java:52)
- q41 code modbusSlave/modbusSlave-rt/vineflower/com/tridium/modbusSlave/BModbusSlaveNetwork.java — "getFeature("tridium", "modbusSlave")" (BModbusSlaveNetwork.java:101)
- q41 code modbusTcpSlave/modbusTcpSlave-rt/vineflower/com/tridium/modbusTcpSlave/BModbusTcpSlaveNetwork.java — "getFeature("tridium", "modbusTcpSlave")" (BModbusTcpSlaveNetwork.java:97)
- Kept BModbusNetwork: it has no feature, which is half of the answer (B301 §301.1).
- Blocks: expectation right, ranking miss (B301 at 35).

### q44 — docs and code
- q44 docs docs-text/docUser.txt — "baja-StationSaveJob — This component appears as a child of the JobService and displays the following properties relative to the save job" (docUser.txt:11298-11303)
- q44 code baja/baja/vineflower/com/tridium/sys/station/BStationSaveJob.java — "public class BStationSaveJob extends BSimpleJob", `run` calls `Station.saveSync(this)` (BStationSaveJob.java:11,21-22)

### q45 — docs
- q45 docs docs-text/docPlatform.txt — "All Niagara 4 platforms have a system passphrase (password), used to encrypt sensitive information ... This system passphrase applies to the JACE-8000 and JACE-9000 controllers." (docPlatform.txt:3247-3252)
- q45 docs docs-text/docJ8Startup.txt — "All Niagara platforms have a system passphrase used to protect and encrypt the system information in the platform’s file systems." (docJ8Startup.txt:1128-1130)
- Kept docStationSecurity.txt: it covers the bog-file passphrase that encodes stored passwords (docStationSecurity.txt:2542-2583).

### q47 — docs and code
- q47 docs docs-text/docHardening.txt — "In Niagara 4, password strength is enforced by the Password Strength property on the authentication scheme Global Password Configuration property" (docHardening.txt:216-222)
- q47 code baja/baja/vineflower/javax/baja/user/BPasswordStrength.java — "public boolean isPasswordValid(char[] password, Consumer<Localizable> messageConsumer)" with `minimumLength` 10, `minimumDigits` 1 (BPasswordStrength.java:81-86,204)

### q48 — code
- q48 code baja/baja/vineflower/javax/baja/security/BAbstractPasswordEncoder.java — "return isReversible ? BAes256PasswordEncoder.ENCODING_TYPE : BPbkdf2HmacSha256PasswordEncoder.ENCODING_TYPE;" (BAbstractPasswordEncoder.java:64,97-98)
- q48 code baja/baja/vineflower/javax/baja/security/BPbkdf2HmacSha256PasswordEncoder.java — `iterationCount = 10000`, "public boolean isReversible() { return false; }" (BPbkdf2HmacSha256PasswordEncoder.java:25,45,73)
- q48 code baja/baja/vineflower/javax/baja/security/BAes256PasswordEncoder.java — "public class BAes256PasswordEncoder extends BAbstractAes256PasswordEncoder", the reversible (encrypted) default (BAes256PasswordEncoder.java:17)

### q50 — code
- q50 code hierarchy/hierarchy-rt/vineflower/javax/baja/hierarchy/BRoleHierarchies.java — "@AgentOn(types = {"baja:IRole"})", `hierarchyNames`, "public class BRoleHierarchies extends BComponent implements BIMixIn" (BRoleHierarchies.java:20-30)
- Removed q50 code BRoleService — zero occurrences of "hierarch"; B565 (the row's block) never cites it

### q52 — blocks and docs
- q52 blocks B810 — "§810.1 The hierarchy you subclass: BDeviceNetwork ... BDevice ... BPointDeviceExt ... BProxyExt" (niagara-mental-model-bloque810.md, §810.1)
- q52 blocks B955 — "§955.1 The driver component hierarchy: a network owns devices, a device owns a point-device-ext, which owns point folders of proxy points" (niagara-mental-model-bloque955.md, §955.1)
- q52 docs devguide-clean/driverFramework.txt — "BDeviceNetwork: This models a physical or logical network of devices. BDevice: This models a physical or logical device" (driverFramework.txt:10-12)
- q52 docs devguide-clean/driverPoint.txt — "each driver provides a specialization of BProxyExt for driver specific addressing, tuning, and IO" (driverPoint.txt:8)

### q53 — code
- q53 code modbusCore/modbusCore-rt/vineflower/com/tridium/modbusCore/client/point/BModbusClientProxyExt.java — "public Object getPollGroupCode()" setting `BDataSourceEnum.devicePoll` or `pointPoll` (BModbusClientProxyExt.java:36,68-98)
- q53 code modbusCore/modbusCore-rt/vineflower/com/tridium/modbusCore/client/datatypes/BDevicePollConfigEntry.java — "public class BDevicePollConfigEntry", `consecutivePointsToPoll`, `setByteArray(byte[] data)` (BDevicePollConfigEntry.java:33,48,193)
- Kept BModbusNetwork: the root of the 3-level hierarchy in B294.

### q54 — docs
- q54 docs docs-text/Spyder_Tool_for_WEBs-N4_User_Guide_-_31-00089.txt — "ERROR VIEW ON LON SPYDER DEVICE ... ERROR VIEW ON BACNET SPYDER DEVICE", the N4 Spyder driver guide for both LON and BACnet Spyders (Spyder_Tool_for_WEBs-N4_User_Guide_-_31-00089.txt:60-83)

### q57 — blocks (unchanged)
Verdict: expectation right, ranking miss (B4 at 24). The returned B1192, B430, B1258, B22, B754 cover
the BOX wire, the property sheet, driver tests, Px runtime and module versioning.

### q58 — docs and code
- Kept devguide-clean/componentModel.txt: "Slots also contain a 32-bit mask of flags ... defined in the javax.baja.sys.Flags class" (componentModel.txt:34); it lacks the words "NiagaraProperty" and "operator" but answers what a slot and its flags are.
- q58 docs bajadoc-clean/javax/baja/sys/Flags.txt — "operator ... This makes a slot an operator security level. By default when this flag is clear, the slot is an admin security level." (Flags.txt:30-31)
- q58 docs devguide-clean/slot-o-matic.txt — "A @NiagaraProperty annotation defines a single Property on a BComplex", `flags= Flags.HIDDEN | Flags.ASYNC` (slot-o-matic.txt:297,314,323)
- q58 code baja/baja/vineflower/javax/baja/sys/Flags.java — "public static final int OPERATOR = 256;" (Flags.java:17)
- q58 code baja/baja/vineflower/javax/baja/sys/Slot.java — "public interface Slot", `isProperty`, `isAction`, `isTopic`, `getDefaultFlags` (Slot.java:3-39)
- q58 code baja/baja/vineflower/javax/baja/sys/Property.java — "public interface Property extends Slot", `getDefaultValue()` (Property.java:3,10)
- Docs remain a recall miss: none of the three pages is returned.

### q59 — docs
- q59 docs docs-text/docHierarchies.txt — "NEQL and BQL can be used together ... ip:<host>|foxs:|station:|slot:/|neql:n:device|bql:select toDisplayPathString, status" and LevelDefs built from tag queries (docHierarchies.txt:705-716,1365-1369)

### q60 — docs
- q60 docs docs-text/docStationSecurity.txt — "HTTPBasicScheme performs HTTP-Basic authentication using standard HTTP headers. It only works via the web" (docStationSecurity.txt:2969-2971) and the offline config.bog edit procedure (docStationSecurity.txt:2542-2583); both halves of B290

### q61 — docs
- q61 docs bajadoc-clean/javax/baja/migration/BIBogElementConverter.txt — "This interface is implemented by classes that handle the migration of a particular Niagara AX module or type to Niagara 4." (BIBogElementConverter.txt:8-11)
- q61 docs bajadoc-clean/javax/baja/migration/ConverterRegistry.txt — "This class manages a registry of BIBogElementConverters, which are registered on Niagara AX types to convert them to Niagara 4." (ConverterRegistry.txt:6-7)
- Kept devguide-clean/bog.txt: "version | Bog schema version. Value is 1.0 for Niagara AX, 4.0 for Niagara 4" (bog.txt:111).

### q67 — code
- q67 code provisioningNiagara/provisioningNiagara-wb/vineflower/javax/baja/provisioningNiagara/backup/BProvisioningBackupStep.java — "public class BProvisioningBackupStep extends BDeviceJobStep", backs up a subordinate station through `BBackupService` and `BBackupStationExt` (BProvisioningBackupStep.java:43,70-75)
- Kept BProvisioningRobot: the provisioning robot program holder (no backup code, but the row asks about the provisioning service).

### q69 — code
- q69 code lonworks/lonworks-rt/vineflower/javax/baja/lonworks/BNetworkVariable.java — "public BNetworkVariable(int nvIndex, int snvtType, ...)", `SnvtUtil.getLonData(snvtType)`, `getData().fromNetBytes(nvData)` (BNetworkVariable.java:126-135,417)
- q69 code lonworks/lonworks-rt/vineflower/javax/baja/lonworks/enums/BLonSnvtType.java — "public static final int SNVT_AMP = 1;" the SNVT type enum (BLonSnvtType.java:228-231)

### q70 — docs and code
- q70 docs devguide-clean/security/csrfProtection.txt — "CsrfProtectedFilter is the Filter class that is defined in the web.XML file of the web module" (csrfProtection.txt:6,12)
- q70 docs devguide-rc/com/tridium/example/auth/client/AuthClientExample.java — "getUserCookieName() { return "niagara_userid"; }" and the `csrfToken` input pattern (AuthClientExample.java:540,657)
- q70 docs docs-text/docUser.txt — "true sets the niagara_userid cookie with the user's username ... false, the niagara_userid cookie is not used at all" (docUser.txt:13028-13030)
- q70 code web/web-rt/vineflower/javax/baja/web/CsrfUtil.java — "public static final boolean verifyCsrfToken(HttpServletRequest req)", header "x-niagara-csrfToken" (CsrfUtil.java:13-17)
- q70 code web/web-rt/vineflower/javax/baja/web/filters/CsrfProtectedFilter.java — "public class CsrfProtectedFilter implements Filter", `if (CsrfUtil.verifyCsrfToken(req))` (CsrfProtectedFilter.java:17,37)
- q70 code web/web-rt/vineflower/com/tridium/web/CookieUtil.java — "public static final String CNAME_USERID = "niagara_userid";" (CookieUtil.java:16)
- Removed q70 code BWebServlet — zero occurrences of "csrf", "cookie" or "session" in the class

## Course exemplar rows q71–q73 (tooling-round3 T1, 2026-10-16)

Three rows added with block B1354 (the participant's verified exam and Review code, copied to
`sources/n4-dev-course/solutions/`). Each expects B1354 plus the earlier course block that covers the
same topic. The `code` lists name the copied class first; `module_nav.py` indexes only `organized/`, so
that path cannot match until the course code becomes a search source (gap B1354-G2, feature decision
"option A vs B"). Where a framework class also answers the question it is listed as well. `doc` is
empty: the official docs describe the Workbench side, not how to write these classes. `baseline` was
measured on 2026-10-16 with the same three CLIs (rank from the front door at limit 10). Their doc
baselines follow the `capped` rule stated for q74–q81 below (true when niagara-help returned its full
`-n 10` page): all three returned 10 rows, recorded `capped: false` until round 3 R5 set them true.
Rows q01–q70 predate that rule and keep their T12 values.

| Row | Question | blocks rank | code rank | Expected code (evidence) |
|---|---|---|---|---|
| q71 | minimal NDriver device ping over a TCP socket connect | 1 (B1354) | 5 (BNDevice) | copied `BWebsiteDriverDevice` (`doPing()` socket connect, `BWebsiteDriverDevice.java:205-218`); `BNDevice` — `public abstract void doPing()` and `postPing()` → network worker (`docSource extracted ndriver-rt BNDevice.java:51,76-78`) |
| q72 | service that exports a history to a CSV file | 4 (B1354) | miss | copied `BHistoryExportService` (`BHistoryExportService.java:123-143`); `BHistoryToCsv` — the exporter Lab 10 uses (`Lab10 Jobs/BHistoriesToCsvService.java:46-48`) |
| q73 | verified review solution component with id and regenerate id action | 1 (B1254) | miss | copied `BComponentWithId` (`BComponentWithId.java:81-95`); no framework class answers it |

Fixture totals after the three rows (`python3 tools/searchdoor/golden.py --misses`, 2026-10-16):
blocks hit@5 69/73, docs 59/60, code 44/54, overall 172/187; new code misses q72, q73.

## Driver rows q74–q81 (tooling-round3 T4b, 2026-10-17)

Eight driver questions (four in Spanish) with verified answer locations per source. A `doc` list is
empty only when the official docs do not answer the question (q79: no page lists which network checks
which feature). `baseline` is the front door after round 3 (nsearch with course code and structure
sub-scopes, docs pinned 4.14.0.162, `--limit 10`). A rank is the position in the FULL ranked list a
source returned (`n_results` rows), not in the 10 rows printed: q75 code 16, q79 blocks 14 and q79
code 24 are found but outside the printed top 10. The baseline uses the schema of the other rows
(`command, rank, ms, exit, n_results`, plus `capped` for docs): `exit` is the source's front-door
state (0 rows, 1 searched zero, 2 unavailable) and `capped` is true when niagara-help returned its
full `-n 10` page. Evidence format as above; code paths under `organized/` unless they start with
`sources/`.

### q74 — all subclasses of BDeviceNetwork including BNiagaraNetwork
- q74 blocks B810 — "`BDeviceNetwork extends BComponent implements BIDeviceFolder,BIStatus,BIPingable,BILicensed`" (niagara-mental-model-bloque810.md:20)
- q74 blocks B415 — "`BNiagaraNetwork` extiende `BDeviceNetwork`" (niagara-mental-model-bloque415.md:55)
- q74 blocks B877 — "ndriver and DDF are PARALLEL chassis, both built directly on `javax.baja.driver`" (niagara-mental-model-bloque877.md:5-6)
- q74 docs bajadoc-clean/javax/baja/driver/BDeviceNetwork.txt — "SUBCLASSES: BLoadableNetwork, BLonNetwork, BObixNetwork, BRdbmsNetwork" (BDeviceNetwork.txt:6; BNiagaraNetwork is not in this list)
- q74 docs bajadoc-clean/com/tridium/nd/BNiagaraNetwork.txt — "EXTENDS: BDeviceNetwork" (BNiagaraNetwork.txt:4)
- q74 code niagaraDriver/niagaraDriver-rt/vineflower/com/tridium/nd/BNiagaraNetwork.java — "public final class BNiagaraNetwork extends BDeviceNetwork" (BNiagaraNetwork.java:117-118); `module_nav hierarchy BDeviceNetwork` does not list it yet (multi-line `extends`, task T4a)

### q75 — como funciona el ping de salud de un dispositivo pingFail pingOk (es)
- q75 blocks B1348 — "generic driver-rt never turns a failed poll into a ping fail ... `pingEnabled` lives on the network's `monitor` (`BPingMonitor`)" (niagara-mental-model-bloque1348.md:1)
- q75 blocks B810 — "**`doPing()`** (`:552`, the health probe — must call `pingOk`/`pingFail`)" (niagara-mental-model-bloque810.md:24)
- q75 docs bajadoc-clean/javax/baja/driver/BDevice.txt — "void pingFail(java.lang.String cause)", "void pingOk()" (BDevice.txt:52-53)
- q75 docs bajadoc-clean/javax/baja/driver/ping/BPingMonitor.txt — "The PingMonitor periodically calls ping on all the IPingables ... alarms when pingables are down" (BPingMonitor.txt:8-10)
- q75 code driver/driver-rt/vineflower/javax/baja/driver/BDevice.java — "public void pingOk() { this.getHealth().pingOk(); }", "public void pingFail(String cause)" (BDevice.java:287-293)
- q75 code driver/driver-rt/vineflower/javax/baja/driver/ping/BPingHealth.java — "public void pingOk()", "public void pingFail(String cause)" (BPingHealth.java:106,121)

### q76 — driver device discovery job for the device manager
- q76 blocks B28 — "NO existe `BDiscoveryJob` base" (each driver writes its own job) (niagara-mental-model-bloque28.md:38-40)
- q76 blocks B1091 — "| `discover` | MgrCommand | BARS | Fires the discovery job |" (niagara-mental-model-bloque1091.md:22)
- q76 docs docs-text/docDrivers.txt — "The discovery job splits the screen into two panes and, as it runs, displays a progress bar" (docDrivers.txt:612-616)
- q76 code ndriver/ndriver-rt/vineflower/com/tridium/ndriver/discover/BNDiscoveryJob.java — "public class BNDiscoveryJob extends BJob" (BNDiscoveryJob.java:29)
- q76 code sources/n4-dev-course/niagara-4.15.6.20/dev/.../envctrldriver/BDeviceDiscoveryJob.java — "public class BDeviceDiscoveryJob extends BSimpleJob" (dev kit example, BDeviceDiscoveryJob.java:30-31; the stem also matches bacnetEDE-wb's BDeviceDiscoveryJob)

### q77 — politica de tuning de los proxy points stale time y min write time (es)
- q77 blocks B872 — "## 872.2 — `BTuningPolicy` / `BTuningPolicyMap`: write behavior", "`minWriteTime` | BRelTime | **0 (off)**" (niagara-mental-model-bloque872.md:47,53)
- q77 blocks B810 — "a `BDeviceNetwork` missing a `tuningPolicies` map" (niagara-mental-model-bloque810.md:99)
- q77 docs bajadoc-clean/javax/baja/driver/point/BTuningPolicy.txt — "static Property minWriteTime", "static Property staleTime" (BTuningPolicy.txt:14-15)
- q77 docs guides-clean/Drivers/driver-TuningPolicy.txt — "| Stale Time | ... Defines the period of time without a successful read ... after which a point’s value is considered to be too old" (driver-TuningPolicy.txt:19)
- q77 code driver/driver-rt/vineflower/javax/baja/driver/point/BTuningPolicy.java — "public static final Property minWriteTime", "public static final Property staleTime" (BTuningPolicy.java:47,52)

### q78 — proxy point conversion between device value and point value BProxyConversion
- q78 blocks B7 — "Property `conversion` (BProxyConversion)", "Excepción en conversion → FAULT status" (niagara-mental-model-bloque7.md:79,81)
- q78 blocks B446 — "The proxy conversions are `BProxyConversion` subclasses in **`com.tridium.nrio.conv`**" (niagara-mental-model-bloque446.md:18)
- q78 docs bajadoc-clean/javax/baja/driver/point/BProxyConversion.txt — "abstract void convertDeviceToProxy(BProxyExt ext, BStatusValue deviceValue, BStatusValue proxyValue)" (BProxyConversion.txt:18-19)
- q78 code driver/driver-rt/vineflower/javax/baja/driver/point/BProxyConversion.java — "public abstract void convertDeviceToProxy(...)", "convertProxyToDevice(...)" (BProxyConversion.java:17,19)

### q79 — que feature de licencia revisa cada red de driver getLicenseFeature (es)
- q79 blocks B301 — "| Network class | `getLicenseFeature()` returns | File |" table of the four Modbus networks (niagara-mental-model-bloque301.md:30-35)
- q79 blocks B497 — "the `tridium:opcUaClient` license gate" (niagara-mental-model-bloque497.md:1)
- q79 blocks B1222 — "License feature `tridium:opc` (OpcNetwork `getLicenseFeature`)" (niagara-mental-model-bloque1222.md:46)
- q79 docs — none: the official docs name licensed features per product, not the check each network class makes (docDrivers.txt:394 mentions only the FileNetwork licence)
- q79 code driver/driver-rt/vineflower/javax/baja/driver/BDeviceNetwork.java — "public Feature getLicenseFeature() { return null; }" and `checkLicense` → `feature.check()` (BDeviceNetwork.java:302-320)

### q80 — chasis de drivers DDF devDriver red y dispositivo BDdfNetwork (es)
- q80 blocks B877 — "NDriver chassis + DDF framework: the two generic driver bases"; "Answers \"chasis de drivers\"" (niagara-mental-model-bloque877.md:1,4)
- q80 docs bajadoc-clean/com/tridium/ddf/BDdfNetwork.txt — "EXTENDS: BLoadableNetwork" (BDdfNetwork.txt:4)
- q80 docs bajadoc-clean/com/tridium/ddf/BDdfDevice.txt — "EXTENDS: BLoadableDevice" (BDdfDevice.txt:4)
- q80 code devDriver/devDriver-rt/vineflower/com/tridium/ddf/BDdfNetwork.java — "public abstract class BDdfNetwork extends BLoadableNetwork implements IDdfFacetConst, BIService" (BDdfNetwork.java:26)
- q80 code devDriver/devDriver-rt/vineflower/com/tridium/ddf/BDdfDevice.java — "public abstract class BDdfDevice extends BLoadableDevice implements IDdfPingable" (BDdfDevice.java:36)

### q81 — NDriver link layer LinkMessage framing over TCP from the course lab
- q81 blocks B1239 — "**ILinkLayer** (talks to the device: `BUdpLinkLayer`/`BTcpLinkLayer`/`BSerialLinkLayer`)", "`LinkMessage`: typically, functionality for all message types are included in one subclass" (niagara-mental-model-bloque1239.md:78-82)
- q81 blocks B877 — "`ILinkLayer` (`comm/ILinkLayer.java:6-13`): `start()`, `stop()`, **`sendMessage(LinkMessage)`**" (niagara-mental-model-bloque877.md:57)
- q81 docs bajadoc-clean/com/tridium/ndriver/comm/ILinkLayer.txt — "ILinkLayer is interface implemented by linkLayers which are coupled to NComm", "abstract void sendMessage(LinkMessage msg)" (ILinkLayer.txt:5,9)
- q81 docs bajadoc-clean/com/tridium/ndriver/comm/LinkMessage.txt — "LinkMessage is a LinkLayer message class that provides mechanisms to stream data to and from a byte array" (LinkMessage.txt:7-8)
- q81 code ndriver/ndriver-rt/vineflower/com/tridium/ndriver/comm/ILinkLayer.java — "public interface ILinkLayer", "void sendMessage(LinkMessage var1)" (ILinkLayer.java:6,13)
- q81 code sources/n4-dev-course/course/Labs/Lab11 NDriver/1 Creating Common Classes/TrafficLightDriverLinkMessage.java — "public class TrafficLightDriverLinkMessage extends LinkMessage", "public boolean receive(InputStream in)" (:21-22,30)
- q81 code sources/n4-dev-course/solutions/cert-exam/websiteDriver/.../comm/WebsiteDriverLinkMessage.java — "public class WebsiteDriverLinkMessage extends LinkMessage" (:16-17)

| Row | blocks rank | docs rank | code rank |
|---|---|---|---|
| q74 | 1 | miss | 2 (token hit; the structure listing lacks it until T4a) |
| q75 | 1 | 5 | 16 |
| q76 | 1 | 2 | 5 |
| q77 | 3 | miss (niagara-help's glossary copy has no "politica"; the canonical one does) | 2 |
| q78 | 2 | miss | 1 |
| q79 | 14 | — | 24 |
| q80 | 1 | 7 | 1 |
| q81 | 1 | miss | 2 |

## D41 course rows (q82-q99, 2026-10-17)

Eighteen course questions added after S3.1 (decision D41), one per `*Why ...?*` item of
`docs/n4-dev-course-solutions/labs.md` (the item ID is in each heading), phrased as a user asks
(8 Spanish, 10 English), topic `course`. Method: `course show <ID>` printed the written answer; a
location was added only when its text or members answer the question (rule 3 above), checked by
opening the line cited below. Blocks that only restate the observation (B1257 screenshots) or
cite the item for another reason (B1253 D12, the L11.36 cast fix) were left out. Docs are listed
only where a niagara-help page states the mechanism; code is the course lab file that carries it
plus the framework class the answer names. Code paths are relative to the checkout here (they
start `organized/` or `sources/`), doc paths to `niagara-help/`; `LAB` = `sources/n4-dev-course/course/Labs`.

Baselines: Go nsearch in process (niagara-go `TestSearchQuality` helper), `--limit 10`, docs
4.14.0.162. The niagara-go quality gate keeps q01-q81 and q82-q99 as separate floors, so the S3.1
baseline (blocks 62/77, docs 36/62, code 30/53) stays checked on its own. New rows at the gate's
`--limit 5`: blocks 10/15 of 18, docs 0/1 of 2, code 10/13 of 16 (hit@1/hit@5).

### q82 — L2.9 (es)
- q82 blocks B1231 — "the observed result: In3/In4 not on the wire sheet (no SUMMARY)" (niagara-mental-model-bloque1231.md:215)
- q82 blocks B1240 — "visible on Wiresheet (`:21`) | `Flags.SUMMARY` (0x08)" (niagara-mental-model-bloque1240.md:228)
- q82 code LAB/Lab2 Add Part 1/BAdd.java — `in1`/`in2` "flags = Flags.SUMMARY", `in3`/`in4` without flags (:16-39)

### q83 — L2.10
- q83 blocks B1231 — "READONLY → non-editable on the AX Property Sheet" (niagara-mental-model-bloque1231.md:211)
- q83 blocks B1240 — "not editable on AX Property Sheet (`:22`) | `Flags.READONLY` (0x01)" (niagara-mental-model-bloque1240.md:229)
- q83 code LAB/Lab2 Add Part 1/BAdd.java — `out` "flags = Flags.SUMMARY | Flags.READONLY" (:16-19)

### q84 — L3.10 (es)
- q84 blocks B1232 — "Neither slide example has the `if (!isRunning()) return;` guard, while both lab solutions do" (niagara-mental-model-bloque1232.md:68)
- q84 blocks B1247 — "During boot, `added(prop, Context.decoding)` and `changed(prop, Context.decoding)` run on every decoded" (niagara-mental-model-bloque1247.md:119)
- q84 code LAB/Lab3 Add Part 2/BAdd.java — "public void changed(Property property, Context context)", "if (!isRunning())" (:50-52)

### q85 — L4.7
- q85 blocks B1253 — "K5 — L4.7/L4.11/L4.15: lab Tstat vs kitControl Tstat ... kitControl's `nullOnInControl` defaults" (niagara-mental-model-bloque1253.md:127)
- q85 blocks B1232 — "Differences in the lab version: no `nullOnInControl`" (niagara-mental-model-bloque1232.md:264)
- q85 docs guides-clean/KitControl/kitControl-Tstat.txt — "| Null On In Control | true (default) or false | true sets an In value to null." (:23)
- q85 docs docs-text/docKitControl.txt — "Null On In Control  true (default) or  true sets an In value to null." (:4251)
- q85 code kitControl/kitControl-rt/vineflower/com/tridium/kitControl/hvac/BTstat.java — `name = "nullOnInControl"` (:35); LAB/Lab4 Tstat/BTstat.java (the lab Tstat, :45)

### q86 — L5.14 (es)
- q86 blocks B1234 — "High = CA signed, internal CA allowed if its cert is in the user trust store, self-signed not allowed" (niagara-mental-model-bloque1234.md:139-140)
- q86 docs guides-clean/ModuleSign/VerificationModesModuleSigning-182D0E86.txt — "High — All modules must be signed with a CA signed certificate" (:15)
- q86 code baja/baja/vineflower/com/tridium/sys/module/ModuleClassLoader.java — "certificates.size() == 1 && CertUtils.checkDnEquality(certificate.getSubjectX500Principal(), certificate.getIssuerX500Principal())" (:420)

### q87 — L5.21
- q87 blocks B1253 — "K10 — L5.21: why `gradlew clean` before `jar`. With unchanged inputs the jar tasks are up to date" (niagara-mental-model-bloque1253.md:160)
- q87 blocks B1234 — "E. Wire gradle | ... `gradlew clean`, `gradlew jar`" (niagara-mental-model-bloque1234.md:102)
- q87 code — none: the answer is Gradle's up-to-date check, no course class

### q88 — L6.8
- q88 blocks B1235 — "`HttpConnection` won't resolve because it lives in `net-rt`; add `api(\":net-rt\")`" (niagara-mental-model-bloque1235.md:263)
- q88 code net/net-rt/vineflower/javax/baja/net/HttpConnection.java — "public class HttpConnection" (:36); LAB/Lab6 Work Order Service/BWorkOrderServiceWithoutSerialization.java — "import javax.baja.net.HttpConnection;" (:14)

### q89 — L6.31 (es)
- q89 blocks B1235 — "Unsigned + signature-requiring group = the group is silently dropped, not the module" (niagara-mental-model-bloque1235.md:210)
- q89 blocks B1234 — "REFLECTION permission group requires a trusted signature (Lab 6)" (niagara-mental-model-bloque1234.md:50)
- q89 code baja/baja/vineflower/com/tridium/security/NiagaraPermissionGroupFactory.java — "Permission group (%s) requires signature but the module (%s) is not signed" (:198); ReflectionPermissionGroup.java — "public boolean requiresSignature()" (:39)

### q90 — L6.32
- q90 blocks B1235 — "The action now takes a parameter (`@NiagaraAction(name=\"submit\", parameterType=\"BWorkOrder\", ...`)" (niagara-mental-model-bloque1235.md:296)
- q90 code LAB/Lab6 Work Order Service/BWorkOrderServiceWithSerialization.java — `parameterType = "BWorkOrder"` (:69); LAB/Lab6 Work Order Service/BWorkOrder.java — "public class BWorkOrder" (:43)

### q91 — L7.9
- q91 blocks B1253 — "K8 — L7.9: why nine tests. The class carries `@Test` at class level" (niagara-mental-model-bloque1253.md:150)
- q91 blocks B1236 — "**Tests (9 methods, all mocked — no network, no json-server).** `@NiagaraType @Test class BWorkOrderServiceTest extends" (niagara-mental-model-bloque1236.md:270)
- q91 code LAB/Lab7 Testing, Lexicons, and Logging/BWorkOrderServiceTest.java — class-level "@Test" (:32)

### q92 — L7.19 (es)
- q92 blocks B1236 — "Default `.level = INFO`, ConsoleHandler at FINEST, persisted in `<userHome>/logging/logging.properties`" (niagara-mental-model-bloque1236.md:42)
- q92 code LAB/Lab7 Testing, Lexicons, and Logging/BWorkOrderService.java — "logger.fine(\"Attempting to connect to \" ...)" behind `isLoggable(Level.FINE)` (:137-139)

### q93 — L9.6 (es)
- q93 blocks B1235 — "Lab 9 names the Java permission behind it: `java.lang.RuntimePermission \"modifyThread\"`" (niagara-mental-model-bloque1235.md:188)
- q93 blocks B1238 — "`shutdownNow()` needs `RuntimePermission \"modifyThread\"` → request **`MANAGE_EXECUTION`**" (niagara-mental-model-bloque1238.md:176)
- q93 code baja/baja/vineflower/com/tridium/security/ManageExecutionPermissionGroup.java — "permissions.add(new RuntimePermission(\"modifyThread\"))" (:44); LAB/Lab9 Multithreaded/BExecutorServiceExecutor.java — "executorService.shutdownNow();" (:57)

### q94 — L9.9
- q94 blocks B1238 — "§1238.3 — a non-ASYNC action invoked from Workbench runs on the session's `Fox:Dispatcher` thread" (niagara-mental-model-bloque1238.md:85)
- q94 code LAB/Lab9 Multithreaded/BClientThreadExecutor.java — "Thread.sleep(10000L);" in a plain action (:32)

### q95 — L10.7
- q95 blocks B1238 — "`api(\":file-rt\")` (its superclass `BITableToCsv` lives in `file`" (niagara-mental-model-bloque1238.md:335)
- q95 code file/file-rt/vineflower/com/tridium/file/exporters/BITableToCsv.java (:77); history/history-rt/vineflower/com/tridium/history/exporters/BHistoryToCsv.java — "public class BHistoryToCsv extends BITableToCsv" (:47); LAB/Lab10 Jobs/BHistoriesToCsvService.java — `type = "BHistoryToCsv"` (:47)

### q96 — L10.20 (es)
- q96 blocks B1253 — "Copied history extensions disabled (L10.20) | already covered → [B1188] (`enabled` = `DEFAULT_ON_CLONE`)" (niagara-mental-model-bloque1253.md:49)
- q96 blocks B1188 — "`BHistoryExt.java:273`: `newProperty(Flags.DEFAULT_ON_CLONE, false, null)`" (niagara-mental-model-bloque1188.md:29)
- q96 code history/history-rt/vineflower/javax/baja/history/ext/BHistoryExt.java — "public static final Property enabled = newProperty(64, false, null);" (:119)

### q97 — L11.36
- q97 blocks B1239 — "**Design choice taught:** poll the **device**, not each point, because `get <iid>` returns a whole intersection" (niagara-mental-model-bloque1239.md:259)
- q97 code LAB/Lab11 NDriver/4 Reading and Writing Points/BTrafficLightDriverDevice.java — "public void doPoll()" (:192)

### q98 — LB.17
- q98 blocks B1239 — "`isExisting` (case-insensitive id compare) is what greys out the" learned row (niagara-mental-model-bloque1239.md:361)
- q98 code LAB/Bonus Labs/Driver Discovery and UI/Implementing Device Discovery/BTrafficLightDriverDeviceDiscoveryLeaf.java — "return device.getIntersectionId().equalsIgnoreCase(getIntersectionId());" (:78)

### q99 — LB.7 (es)
- q99 blocks B1239 — "(2) **delete `module-include.xml`** in the `-wb` part because \"the Gradle tasks sometimes fail to overwrite existing" (niagara-mental-model-bloque1239.md:324)
- q99 blocks B1233 — "fail to overwrite existing <type> entries correctly … delete the module-include.xml file" (niagara-mental-model-bloque1233.md:115)
- q99 code — none: a build-file step

| Row | blocks rank | docs rank | code rank |
|---|---|---|---|
| q82 | 4 | — | 1 |
| q83 | 3 | — | 1 |
| q84 | 4 | — | 1 |
| q85 | 1 | 2 | 1 |
| q86 | 10 | miss | miss |
| q87 | 1 | — | — |
| q88 | 1 | — | 1 |
| q89 | 37 | — | 9 |
| q90 | 1 | — | 2 |
| q91 | 1 | — | 1 |
| q92 | 1 | — | 1 |
| q93 | 2 | — | 1 |
| q94 | 1 | — | 1 |
| q95 | 1 | — | 1 |
| q96 | miss | — | miss |
| q97 | 1 | — | 2 |
| q98 | 2 | — | 3 |
| q99 | 1 | — | — |
