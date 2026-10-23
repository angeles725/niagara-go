package shippedxml

import (
	"reflect"
	"testing"
)

// groupsTemplate is the module-permissions.xml template Niagara 4.13-4.15
// ship (wbutil-wb rc/, devkit-wb gradle/includes/): permission groups whose
// one request is commented out, one comment per line.
const groupsTemplate = `<permissions>
  <niagara-permission-groups type="all">
    <!-- Insert any global permissions here. -->
  </niagara-permission-groups>
  <niagara-permission-groups type="station">
    <!--<req-permission>-->
    <!--<name>NETWORK_COMMUNICATION</name>-->
    <!--<purposeKey>Outside access for Driver</purposeKey>-->
    <!--<parameters>-->
      <!--<parameter name="hosts" value="127.0.0.1"/>-->
      <!--<parameter name="ports" value="*"/>-->
    <!--</parameters>-->
    <!--</req-permission>-->
    <req-permission>
      <name>FILE_ACCESS</name>
      <purposeKey>Logs</purposeKey>
    </req-permission>
  </niagara-permission-groups>
</permissions>
`

// javaTemplate is the 4.3 devkit-wb videoDriver template: java-permissions.
const javaTemplate = `<permissions>
  <java-permissions type="station">
    <java-permission action="read,write" class="java.io.FilePermission" name="${niagara.user.home}${/}logging"/>
  </java-permissions>
  <java-permissions type="all">
    <java-permission class="java.util.logging.LoggingPermission" name="control"/>
  </java-permissions>
</permissions>
`

func TestParsePermissionsReadsGroupsAndRequests(t *testing.T) {
	p, err := ParsePermissions("rc/module-permissions.xml", []byte(groupsTemplate))
	if err != nil {
		t.Fatal(err)
	}
	want := []Group{
		{Type: "all"},
		{Type: "station", Requests: []Request{
			{Name: "NETWORK_COMMUNICATION", PurposeKey: "Outside access for Driver", Commented: true,
				Parameters: [][2]string{{"hosts", "127.0.0.1"}, {"ports", "*"}}},
			{Name: "FILE_ACCESS", PurposeKey: "Logs"},
		}},
	}
	if !reflect.DeepEqual(p.Groups, want) || p.Path != "rc/module-permissions.xml" {
		t.Errorf("groups = %+v", p.Groups)
	}
}

func TestParsePermissionsReadsJavaPermissions(t *testing.T) {
	p, err := ParsePermissions("templates/videoDriver/module-permissions.xml", []byte(javaTemplate))
	if err != nil {
		t.Fatal(err)
	}
	want := []JavaPermission{
		{"station", "java.io.FilePermission", "${niagara.user.home}${/}logging", "read,write"},
		{"all", "java.util.logging.LoggingPermission", "control", ""},
	}
	if !reflect.DeepEqual(p.JavaPermissions, want) || len(p.Groups) != 0 {
		t.Errorf("java permissions = %+v groups = %+v", p.JavaPermissions, p.Groups)
	}
}

// TestParsePermissionsReadsAVelocityInclude: devkit-wb's gradle/module
// template only #parse-s the shared one; it is recorded, not parsed as XML.
func TestParsePermissionsReadsAVelocityInclude(t *testing.T) {
	p, err := ParsePermissions("gradle/module/module-permissions.xml.vm",
		[]byte("#parse ( \"$includePath/module-permissions.xml.vm\" )\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Velocity, []string{`#parse ( "$includePath/module-permissions.xml.vm" )`}) {
		t.Errorf("velocity = %q", p.Velocity)
	}
}

func TestParsePermissionsRefusesWhatItCannotRead(t *testing.T) {
	for name, c := range map[string][2]string{
		"not xml":       {"module-permissions.xml", "<permissions><java-permissions>"},
		"wrong root":    {"module-permissions.xml", "<module/>"},
		"stray request": {"module-permissions.xml", "<permissions><req-permission/></permissions>"},
		"vm with text":  {"module-permissions.xml.vm", "#parse (x)\nplain words"},
		"xml as text":   {"module-permissions.xml", "#parse (x)"},
	} {
		if _, err := ParsePermissions(c[0], []byte(c[1])); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
