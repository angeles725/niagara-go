package javax.baja.control;

import javax.baja.status.BStatusBoolean;
import javax.baja.sys.Property;

public class BBooleanPoint extends BControlPoint {
   public static final Property out = newProperty(9, new BStatusBoolean(), null);
}
