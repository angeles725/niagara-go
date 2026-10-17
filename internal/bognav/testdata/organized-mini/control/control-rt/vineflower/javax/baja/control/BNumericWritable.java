package javax.baja.control;

import javax.baja.status.BStatusNumeric;
import javax.baja.sys.Property;

public class BNumericWritable extends BControlPoint {
   public static final Property fallback = newProperty(0, new BStatusNumeric(0.0, BStatus.nullStatus), null);
}
