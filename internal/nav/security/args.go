// Package security declares the module-nav security commands (Security, licensing and permissions);
// they register into nav/registry from init.
package security

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("security", []registry.Command{
		{Name: "license-feature", Help: "Call sites that check or read a licensed feature (checkFeature/getFeature)", Args: argsLicenseFeature},
		{Name: "license-features", Help: "Licensed feature names with counts; --module: features a module requires", Args: argsLicenseFeatures},
		{Name: "permissions", Help: "List classes with permission checks and auth methods", Args: argsPermissions},
		{Name: "license-inspect", Help: "Parse installed .license files and aggregate features", Args: argsLicenseInspect},
		{Name: "policy-inspect", Help: "Parse bin/policy/{signing.properties,java.policy,java.security}", Args: argsPolicyInspect},
		{Name: "trust-anchor-check", Help: "Verify a JAR's signer cert against the trust anchor", Args: argsTrustAnchorCheck},
		{Name: "bypass-status", Help: "Report skipModuleValidation & security-manager-disable bypass state", Args: argsBypassStatus},
		{Name: "permission-report", Help: "Aggregate declared java-permissions into the 19 devguide groups", Args: argsPermissionReport},
		{Name: "feasibility-check", Help: "Simulate whether a module+cert would deploy against the trust anchor", Args: argsFeasibilityCheck},
		{Name: "permission-flow", Help: "Permission verification chain for a class", Args: argsPermissionFlow},
		{Name: "credentials", Help: "Detect credential handling (password, token, secret)", Args: argsCredentials},
	})
}

// argsLicenseFeature declares `license-feature`.
func argsLicenseFeature(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Feature name (e.g. historyImport)"}, "name")
	p.Add(argparse.Arg{Default: 100, Type: argparse.Int, Help: "Max results to show (default 100)"}, "-n")
	p.Add(argparse.Arg{Help: "Only call sites in this submodule or module (e.g. bacnet-rt, bacnet)", Dest: "lf_module"}, "--module", "-m")
}

// argsLicenseFeatures declares `license-features`.
func argsLicenseFeatures(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max feature names to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Show the features this submodule or module requires", Dest: "lf_module"}, "--module", "-m")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "List every call and getLicenseManager() site no feature name was found for", Dest: "lf_unresolved"}, "--unresolved")
}

// argsPermissions declares `permissions`.
func argsPermissions(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "perm_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 200, Type: argparse.Int, Help: "Max results to show (default 200)"}, "-n")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Scan META-INF/module.xml <permissions> blocks instead of callsites"}, "--declared")
	p.Add(argparse.Arg{Choices: []any{"station", "all", "workbench"}, Help: "Restrict --declared results to one permission type", Dest: "perm_type"}, "--type")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "With --declared: show aggregate counts (per module/type/class) instead of rows", Dest: "perm_summary"}, "--summary")
}

// argsLicenseInspect declares `license-inspect`.
func argsLicenseInspect(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Directory with .license files (default: $NIAGARA_HOME/security/licenses)", Dest: "lic_dir"}, "--dir")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Only show critical (bypass-enabling) features — skip headers and summary", Dest: "lic_critical_only"}, "--critical-features")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Emit markdown with YAML frontmatter instead of ASCII table", Dest: "lic_md"}, "--md")
}

// argsPolicyInspect declares `policy-inspect`.
func argsPolicyInspect(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Directory containing policy files (default: $NIAGARA_HOME/bin/policy)", Dest: "pol_dir"}, "--dir")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Report signature block status (PKCS7 verification not implemented)", Dest: "pol_verify"}, "--verify-signatures")
}

// argsTrustAnchorCheck declares `trust-anchor-check`.
func argsTrustAnchorCheck(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Path to the JAR file"}, "jar")
	p.Add(argparse.Arg{Help: "Directory with signing.properties (default: $NIAGARA_HOME/bin/policy)"}, "--anchor-dir")
	p.Add(argparse.Arg{Help: "Path to keytool binary (auto-resolved from $NIAGARA_HOME/jre/bin)", Dest: "keytool_path"}, "--keytool")
}

// argsBypassStatus declares `bypass-status`.
func argsBypassStatus(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Niagara install root (default: $NIAGARA_HOME)", Dest: "bypass_dir"}, "--dir")
}

// argsPermissionReport declares `permission-report`.
func argsPermissionReport(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to one group (e.g. NETWORK_COMMUNICATION). When set, also shows top 10 modules for that group.", Dest: "preport_group"}, "--group")
	p.Add(argparse.Arg{Choices: []any{"MILD", "MODERATE", "SEVERE"}, Help: "Filter groups by severity level", Dest: "preport_severity"}, "--severity")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Emit markdown with YAML frontmatter instead of ASCII table", Dest: "preport_md"}, "--md")
}

// argsFeasibilityCheck declares `feasibility-check`.
func argsFeasibilityCheck(p *argparse.Parser) {
	p.Add(argparse.Arg{Required: true, Help: "Path to source module-permissions.xml", Dest: "feas_perms"}, "--module-permissions")
	p.Add(argparse.Arg{Required: true, Help: "Path to signing certificate (PEM or DER, anything keytool accepts)", Dest: "feas_cert"}, "--signing-cert")
	p.Add(argparse.Arg{Help: "Override signing.properties location (default: $NIAGARA_HOME/bin/policy)", Dest: "feas_anchor"}, "--anchor-dir")
	p.Add(argparse.Arg{Help: "Explicit keytool binary path", Dest: "feas_keytool"}, "--keytool")
}

// argsPermissionFlow declares `permission-flow`.
func argsPermissionFlow(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BWebServlet)"}, "class_name")
}

// argsCredentials declares `credentials`.
func argsCredentials(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "cred_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}
