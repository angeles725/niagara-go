package javax.baja.control;

import javax.baja.sys.BComponent;
import javax.baja.sys.BFacets;
import javax.baja.sys.Property;

public abstract class BControlPoint extends BComponent {
   public static final Property facets = newProperty(0, BFacets.NULL, null);
}
