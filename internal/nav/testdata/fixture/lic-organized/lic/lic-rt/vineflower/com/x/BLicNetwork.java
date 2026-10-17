package com.x;
public class BLicNetwork {
  static final String VENDOR = "acme";
  void a() { Sys.getLicenseManager().checkFeature(VENDOR, "licFeat"); }
  void b() { LicenseManager lm = Sys.getLicenseManager(); lm.getFeature("Acme", "otherFeat"); }
  void c() { Sys.getLicenseManager().getFeatures(); }
  void d() { Sys.getLicenseManager().checkFeature(vendorVar, featureVar); }
  void e() { use(Sys.getLicenseManager()); }
}
