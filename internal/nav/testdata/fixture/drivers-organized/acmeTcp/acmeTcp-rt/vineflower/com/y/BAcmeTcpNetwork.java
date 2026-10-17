public class BAcmeTcpNetwork extends BAcmeNetwork {
  void check() { Sys.getLicenseManager().getFeature(null, "acmeTcp"); Sys.getLicenseManager().checkFeature("", "blankVendor"); }
}
