package nlicense

// usage is the tool's module docstring, printed verbatim by -h and on bad usage.
const usage = "\n" +
	"nlicense — OEM license tooling for Niagara N4 `.license` files.\n" +
	"\n" +
	"Replicates the platform's license signing/verification (validated against a REAL\n" +
	"signed license, pentest B316-L12/B322):\n" +
	"  * signature = DSA-1024 (q=160) / SHA-1 over the canonical XML re-encoding\n" +
	"    WITHOUT the <signature> element (LicenseUtil.encode semantics: attributes in\n" +
	"    document order, \">\n" +
	"\", children, \"</root>\n" +
	"\")\n" +
	"  * signature element = base64 DER SEQUENCE { INTEGER r(20B), INTEGER s(20B) }\n" +
	"  * certificate chain: license sig verified against {vendor}.certificate public\n" +
	"    key; the certificate's OWN sig verifies against the EMBEDDED root key in\n" +
	"    baja.jar (this build: single DSA root, B322)\n" +
	"\n" +
	"Subcommands:\n" +
	"  verify  <license.xml> <certificate.xml>   -> offline check (no station needed)\n" +
	"  sign    <license.xml> <privkey.pem>       -> re-sign in place (new <signature>)\n" +
	"  rehost  <license.xml> <new-hostid> <privkey.pem> -> change hostId AND re-sign\n" +
	"  gen     <out.license> <vendor> <hostid> <expiration> <features.csv> <privkey.pem>\n" +
	"          -> build a license from scratch and sign it\n" +
	"\n" +
	"The private key is the VENDOR's DSA private key matching {vendor}.certificate's\n" +
	"public key (the OEM holds it). NEVER store/commit private keys — they stay on the\n" +
	"operator's machine; this tool only reads them from the path you pass.\n" +
	"\n" +
	"Dependencies: python3 + cryptography (pip install cryptography).\n" +
	""
