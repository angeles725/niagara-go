package com.x;
class BLicView {
  void a() { Sys.getLicenseManager().checkFeature("tridium", "licFeat"); }
  void b() { LicenseManager unused = Sys.getLicenseManager(); }
}
