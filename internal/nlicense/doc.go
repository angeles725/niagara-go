// Package nlicense ports tools/niagara-license-tool.py (nlicense): offline
// verify, sign, rehost and gen of Niagara N4 .license files (DSA/SHA-1 over
// the LicenseUtil.encode canonical form, lxml-identical file output).
//
// verify says VALID only when the license signature verifies against the
// certificate's key, the vendors match, and the certificate's own signature
// verifies against the root key baja.jar embeds (chain.go).
//
// Exit: 0 VALID or done, 1 invalid (a FAIL line) or a failed operation, 2 bad
// usage or an input that could not be read or parsed.
package nlicense
