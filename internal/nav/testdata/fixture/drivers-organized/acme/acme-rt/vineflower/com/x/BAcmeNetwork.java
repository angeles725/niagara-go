public class BAcmeNetwork extends BDdfNetwork {
  public Type getDeviceType() { return BAcmeDevice.TYPE; }
  void check() { Sys.getLicenseManager().checkFeature("acme", "acmeDriver"); }
}
