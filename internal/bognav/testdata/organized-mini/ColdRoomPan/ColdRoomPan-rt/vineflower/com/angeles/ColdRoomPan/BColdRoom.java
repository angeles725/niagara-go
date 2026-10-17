package com.angeles.ColdRoomPan;

import javax.baja.sys.BComponent;
import javax.baja.sys.Property;

public class BColdRoom extends BComponent {
   public static final Property setpoint = newProperty(256, new BStatusNumeric(-18.0), null);
   public static final Property valveMode = newProperty(256, BHoaMode.auto, null);
   public static final Property defrostEnabled = newProperty(0, true, null);
}
