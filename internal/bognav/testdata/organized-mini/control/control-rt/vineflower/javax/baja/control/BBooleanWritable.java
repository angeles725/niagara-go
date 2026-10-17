package javax.baja.control;

import javax.baja.status.BStatusBoolean;
import javax.baja.sys.BRelTime;
import javax.baja.sys.Property;

public class BBooleanWritable extends BBooleanPoint {
   public static final Property in10 = newProperty(10, new BStatusBoolean(false, BStatus.nullStatus), null);
   public static final Property fallback = newProperty(0, new BStatusBoolean(false, BStatus.nullStatus), null);
   public static final Property minActiveTime = newProperty(0, BRelTime.make(0L), null);
}
