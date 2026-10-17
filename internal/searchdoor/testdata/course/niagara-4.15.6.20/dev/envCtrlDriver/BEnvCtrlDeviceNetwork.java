package com.tridium.envCtrlDriver;

/** The dev kit example network: polls each device of the network over IP. */
public final class BEnvCtrlDeviceNetwork extends BDeviceNetwork {
  public static final Property ipAddress = newProperty(0, "127.0.0.1", null);
}
