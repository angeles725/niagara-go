package javax.baja.control;

public abstract class BNumericPoint extends BControlPoint {
   public static final Property out = newProperty((int)1026, (BValue)new BStatusNumeric(), (BFacets)null);
   public static final Property facets = newProperty(javax.baja.sys.Flags.SUMMARY|Flags.BOGUS, BFacets.DEFAULT,
      BFacets.make(BFacets.make(BFacets.FIELD_EDITOR, "driver:ProxyConversionFE"),
                   BFacets.make(BFacets.UX_FIELD_EDITOR, "driver:ProxyConversionEditor")));
}
