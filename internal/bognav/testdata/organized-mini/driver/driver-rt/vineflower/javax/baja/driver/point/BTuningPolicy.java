package javax.baja.driver.point;

import javax.baja.sys.BComponent;
import javax.baja.sys.BRelTime;
import javax.baja.sys.Property;

public class BTuningPolicy extends BComponent {
   public static final Property minWriteTime = newProperty(0, BRelTime.make(0L), null);
   public static final Property writeOnStart = newProperty(0, true, null);
   public static final Property writeOnUp = newProperty(0, true, null);
}
