package check

// The review-assignment checks (RA1, RA2): what the participant's solution leaves in the review
// station (sources/n4-dev-course/text/Review Assignment <k>.txt asks for one station per
// participant, <First>_<Last>_Review) and in the built module <First>_<Last>_Review-rt. Keys are
// the requirement IDs of review-assignments.md (RA1.V9–V14 and RA2.V10–V15 are one range item each,
// reached by any covered ID) and the needs-a-run checks X1, X3, X6, X10 that a station or a jar
// answers. The participant's names are those of the verified module (Cristian_Angeles_Review):
// the module name and the types follow from them.
//
// Defaults: BComponentWithId id "" and BPointSummaryService summaryFile BOrd.DEFAULT ("null")
// (sources/n4-dev-course/solutions/review/ and review-assignments.md §1.3, §2.3); BNumericWritable
// fallback 0.0 (organized/control/control-rt/decompiled/javax/baja/control/BNumericWritable.java:78),
// read as the nested slot fallback.value (bog.Options{NestedSlots: true}): the value the set action
// writes.

const (
	reviewRT        = "Cristian_Angeles_Review-rt"
	componentWithID = "Cristian_Angeles_Review:ComponentWithId"
	pointSummary    = "Cristian_Angeles_Review:PointSummaryService"
	reviewTxt       = "shared/Review.txt" // file:^Review.txt: the station's Files space

	// The UI steps of RA1.V9–V14 no bog or jar holds.
	ra1UISteps = "the Wire Sheet Id column, the regenerateId confirmation dialog and the read-only Property Sheet (UI steps)"
	// What the station rows of RA2.V10–V15 do not read: that the summary lists every point with its
	// out (CRA2.16 compares the file with the bog; the others count or match lines).
	ra2Listing = "that Review.txt lists every NumericWritable of the station with its out (CRA2.16 compares them)"
	// The participant's review sources in the corpus solutions (--solutions), as the module was built.
	reviewSrc = "review/Cristian_Angeles_Review/Cristian_Angeles_Review-rt/src/com/review/Cristian_Angeles_Review/"
	// What a jar entry does not say about the written listing (X1).
	writtenListing = "the written listing of review-assignments.md (the jar holds the participant's built code, not the listing)"
)

var reviewSpec = []Check{
	// RA1. Module identity (module.xml written by the build from the -rt Gradle script).
	{ID: "CRA1.1", Lab: "RA1", Keys: []string{"RA1.M2"}, Kind: kindJarText, Desc: "module name <First>_<Last>_Review",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`moduleName="Cristian_Angeles_Review"`}},
	{ID: "CRA1.2", Lab: "RA1", Keys: []string{"RA1.M3"}, Kind: kindJarText, Desc: "description <First> <Last> Niagara Review Assignments",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`description="Cristian Angeles Niagara Review Assignments"`}},
	{ID: "CRA1.3", Lab: "RA1", Keys: []string{"RA1.M4"}, Kind: kindJarText, Desc: "vendor review",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`vendor="review"`}},
	{ID: "CRA1.4", Lab: "RA1", Keys: []string{"RA1.M5"},
		Unobserved: map[string]string{"RA1.M5": "that the wizard created no other part (-ux, -wb)"}, Kind: kindJarText, Desc: "runtime profile rt",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`runtimeProfile="rt"`}},
	{ID: "CRA1.5", Lab: "RA1", Keys: []string{"RA1.M6", "RA1.R8"}, Kind: kindJarText, Desc: "palette lists ComponentWithId",
		Jar: reviewRT, Glob: "module.palette", Contains: []string{`n="ComponentWithId"`}},
	{ID: "CRA1.6", Lab: "RA1", Keys: []string{"RA1.R1"},
		Unobserved: map[string]string{"RA1.R1": "that the class extends BComponent"}, Kind: kindJarText, Desc: "module.xml registers ComponentWithId",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`name="ComponentWithId"`}},
	{ID: "CRA1.7", Lab: "RA1", Keys: []string{"RA1.R1", "X1"},
		Unobserved: map[string]string{"RA1.R1": "that the class extends BComponent", "X1": writtenListing}, Kind: kindJarEntry,
		Desc: "BComponentWithId.class built", Jar: reviewRT, Glob: "**/BComponentWithId.class", Op: opMin, N: 1},

	// RA1 station steps (RA1.V9–V14): Config/Review, one added, copy/paste gives a different id.
	{ID: "CRA1.8", Lab: "RA1", Keys: []string{"RA1.V11"},
		Unobserved: map[string]string{"RA1.V11": ra1UISteps}, Kind: kindType, Desc: "two ComponentWithId (one added, one pasted)",
		Type: componentWithID, Op: opMin, N: 2},
	{ID: "CRA1.9", Lab: "RA1", Keys: []string{"RA1.V12", "RA1.R6", "X3"},
		Unobserved: map[string]string{"RA1.V12": ra1UISteps, "RA1.R6": "that started() set the ids (the bog holds them, not when they were set)",
			"X3": "that cut/paste keeps the id"}, Kind: kindSlot, Desc: "the ComponentWithId ids are distinct",
		Type: componentWithID, Slot: "id", Default: "", Distinct: true, Op: opMin, N: 2},
	{ID: "CRA1.10", Lab: "RA1", Keys: []string{"RA1.V10"},
		Unobserved: map[string]string{"RA1.V10": ra1UISteps}, Kind: kindType, Desc: "ComponentWithId in the folder Config/Review",
		Type: componentWithID, Under: "Review", Op: opMin, N: 1},

	// RA1 requirements from the participant's sources (corpus solutions; review-assignments.md §1.4
	// traceability: extends BComponent, the id property, its flags, the regenerateId action).
	{ID: "CRA1.11", Lab: "RA1", Keys: []string{"RA1.R1"}, Kind: kindAnnotation, Desc: "BComponentWithId extends BComponent",
		Root: rootSolutions, Glob: reviewSrc + "BComponentWithId.java", Fact: factExtends, Want: "BComponent"},
	{ID: "CRA1.12", Lab: "RA1", Keys: []string{"RA1.R2"}, Kind: kindAnnotation, Desc: `frozen property id: String = ""`,
		Root: rootSolutions, Glob: reviewSrc + "BComponentWithId.java", Fact: factDefaults, Slot: "id", Want: `id: String = ""`},
	{ID: "CRA1.13", Lab: "RA1", Keys: []string{"RA1.R3a", "RA1.R3b", "RA1.R3c"},
		Unobserved: map[string]string{"RA1.R3a": "the Wire Sheet by hand (the row reads SUMMARY, s)",
			"RA1.R3b": "the checkSetFlags guard and the Property Sheet by hand (the row reads READONLY, r)",
			"RA1.R3c": "a paste by hand (the row reads DEFAULT_ON_CLONE, d)"}, Kind: kindAnnotation,
		Desc: "id flags: SUMMARY, READONLY, DEFAULT_ON_CLONE", Root: rootSolutions, Glob: reviewSrc + "BComponentWithId.java",
		Fact: factFlags, Slot: "id", Want: "id=rsd"},
	{ID: "CRA1.14", Lab: "RA1", Keys: []string{"RA1.R4"}, Kind: kindAnnotation, Desc: "frozen action regenerateId",
		Root: rootSolutions, Glob: reviewSrc + "BComponentWithId.java", Fact: factSlots, Slot: "regenerateId", Want: "action regenerateId()"},
	{ID: "CRA1.15", Lab: "RA1", Keys: []string{"RA1.R5"}, Unobserved: map[string]string{"RA1.R5": "the confirmation dialog by hand (the row reads CONFIRM_REQUIRED, c)"},
		Kind: kindAnnotation, Desc: "regenerateId flags: CONFIRM_REQUIRED", Root: rootSolutions, Glob: reviewSrc + "BComponentWithId.java",
		Fact: factFlags, Slot: "regenerateId", Want: "regenerateId=c"},

	// RA2. The service and its summary file.
	{ID: "CRA2.1", Lab: "RA2", Keys: []string{"RA2.R1"}, Kind: kindJarText, Desc: "depends on control-rt",
		Jar: reviewRT, Glob: trainingXML, Contains: []string{`name="control-rt"`}},
	{ID: "CRA2.2", Lab: "RA2", Keys: []string{"RA2.R2", "X1"},
		Unobserved: map[string]string{"RA2.R2": "that the class extends BAbstractService", "X1": writtenListing}, Kind: kindJarEntry,
		Desc: "BPointSummaryService.class built", Jar: reviewRT, Glob: "**/BPointSummaryService.class", Op: opMin, N: 1},
	{ID: "CRA2.3", Lab: "RA2", Keys: []string{"RA2.R7"}, Kind: kindJarText, Desc: "palette lists PointSummaryService",
		Jar: reviewRT, Glob: "module.palette", Contains: []string{`n="PointSummaryService"`}},
	{ID: "CRA2.4", Lab: "RA2", Keys: []string{"RA2.R8", "X10"},
		Unobserved: map[string]string{"RA2.R8": "that the certificate is the one of the Code Signing lab",
			"X10": "Module Info's green check at moduleVerificationMode=high (runtime)"}, Kind: kindJarSigned,
		Desc: "Cristian_Angeles_Review-rt is signed", Jar: reviewRT},
	{ID: "CRA2.5", Lab: "RA2", Keys: []string{"RA2.R9"}, Kind: kindJar, Desc: "Cristian_Angeles_Review-rt jar built", Jar: reviewRT},
	{ID: "CRA2.6", Lab: "RA2", Keys: []string{"RA2.V10"},
		Unobserved: map[string]string{"RA2.V10": ra2Listing}, Kind: kindType, Desc: "NumericWritables in Config/Review2",
		Type: numericWritable, Under: "Review2", Op: opMin, N: 2},
	{ID: "CRA2.7", Lab: "RA2", Keys: []string{"RA2.V12", "RA2.R2"},
		Unobserved: map[string]string{"RA2.R2": "that the class extends BAbstractService", "RA2.V12": ra2Listing}, Kind: kindType,
		Desc: "one PointSummaryService, under Services", Type: pointSummary, Within: services, Op: opEq, N: 1},
	{ID: "CRA2.8", Lab: "RA2", Keys: []string{"RA2.V13", "RA2.R3"},
		Unobserved: map[string]string{"RA2.R3": "that summaryFile is a frozen BOrd property with default BOrd.DEFAULT",
			"RA2.V13": ra2Listing}, Kind: kindSlot,
		Desc: "summaryFile = file:^Review.txt", Type: pointSummary, Slot: "summaryFile", Want: "file:^Review.txt", Default: "null"},
	{ID: "CRA2.9", Lab: "RA2", Keys: []string{"RA2.V15", "X6"},
		Unobserved: map[string]string{"RA2.V15": ra2Listing}, Kind: kindFiles, Desc: "Review.txt in the station's Files space",
		Glob: reviewTxt, Op: opEq, N: 1},
	{ID: "CRA2.10", Lab: "RA2", Keys: []string{"RA2.R5b", "RA2.V15"},
		Unobserved: map[string]string{"RA2.R5b": "that every NumericWritable of the station is listed (the check wants two lines or more)",
			"RA2.V15": ra2Listing}, Kind: kindFileText,
		Desc: "Review.txt lines: name: value {status} @ priority", Glob: reviewTxt,
		Match: `^\S+: -?[0-9][0-9.,]* \{[^}]*\} @ \S+$`, Op: opMin, N: 2},
	// RA2.V11 "add several NumericWritables to Review2 and set them to different values": the set
	// action writes the fallback, kept below the composite slot (fallback.value, default 0.0).
	{ID: "CRA2.11", Lab: "RA2", Keys: []string{"RA2.V11"},
		Unobserved: map[string]string{"RA2.V11": ra2Listing}, Kind: kindSlot,
		Desc: "distinct fallback values in Config/Review2", Type: numericWritable, Under: "Review2",
		Slot: "fallback.value", Default: "0.0", Distinct: true, Op: opMin, N: 2},
	// RA2.R8: the review module is signed with the Code Signing lab's certificate, the signer of
	// training-rt (C5.11-C5.14).
	{ID: "CRA2.12", Lab: "RA2", Keys: []string{"RA2.R8"}, Kind: kindJarCert, Desc: "signed with the certificate of training-rt (Lab 5)",
		Jar: reviewRT, Fact: certSame, CertOf: trainingRT},
	// RA2 requirements from the participant's sources (review-assignments.md §2.4 traceability). The
	// key's design adds what the participant's service does not declare: noted, never checked.
	{ID: "CRA2.13", Lab: "RA2", Keys: []string{"RA2.R2"},
		Unobserved: map[string]string{"RA2.R2": "the key's implements BIRestrictedComponent and getServiceTypes() (the participant's class declares only the superclass)"},
		Kind:       kindAnnotation, Desc: "BPointSummaryService extends BAbstractService", Root: rootSolutions,
		Glob: reviewSrc + "BPointSummaryService.java", Fact: factExtends, Want: "BAbstractService"},
	{ID: "CRA2.14", Lab: "RA2", Keys: []string{"RA2.R3"}, Kind: kindAnnotation, Desc: "frozen property summaryFile: BOrd = BOrd.DEFAULT",
		Root: rootSolutions, Glob: reviewSrc + "BPointSummaryService.java", Fact: factDefaults, Slot: "summaryFile", Want: "summaryFile: BOrd = BOrd.DEFAULT"},
	{ID: "CRA2.15", Lab: "RA2", Keys: []string{"RA2.R4"},
		Unobserved: map[string]string{"RA2.R4": "the key's flags = Flags.ASYNC (the participant's action declares no flags; RA2.R6 leaves the thread technique free)"},
		Kind:       kindAnnotation, Desc: "frozen action createSummaryFile", Root: rootSolutions,
		Glob: reviewSrc + "BPointSummaryService.java", Fact: factSlots, Slot: "createSummaryFile", Want: "action createSummaryFile()"},
	// RA2.R5a/R5b, RA2.V15: Review.txt names exactly the station's NumericWritables (the whole
	// station, as the BQL of RA2.R5a selects them), each at priority def (RA2.V11's set writes the
	// fallback) with its fallback value as out.
	{ID: "CRA2.16", Lab: "RA2", Keys: []string{"RA2.V15", "RA2.R5a", "RA2.R5b"},
		Unobserved: map[string]string{"RA2.R5a": "that a BQL query produced the listing (the row compares the file the action wrote with the bog)",
			"RA2.R5b": "that an existing file is overwritten (runtime: the row reads the last file)"}, Kind: kindListing,
		Desc: "Review.txt lists every NumericWritable at def with its fallback", Glob: reviewTxt, Type: numericWritable,
		Match: `^(?P<name>\S+): (?P<value>-?[0-9][0-9.,]*) \{[^}]*\} @ (?P<priority>\S+)$`, Slot: "fallback.value", Default: "0.0", Want: "def"},
}
