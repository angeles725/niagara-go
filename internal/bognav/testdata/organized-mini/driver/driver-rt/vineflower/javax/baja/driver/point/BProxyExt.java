package javax.baja.driver.point;

import javax.baja.sys.BComponent;
import javax.baja.sys.Property;

public abstract class BProxyExt extends BComponent {
   public static final Property enabled = newProperty(0, true, null);
   public static final Property tuningPolicyName = newProperty(0, "defaultPolicy", null);
}
