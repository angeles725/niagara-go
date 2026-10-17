package a.b;
public class BMulti extends BComponent {
  public static final Property conversion = newProperty(0, BDefaultProxyConversion.DEFAULT, BFacets.make(BFacets.make(BFacets.FIELD_EDITOR, "driver:ProxyConversionFE"), BFacets.make(BFacets.UX_FIELD_EDITOR, "driver:ProxyConversionEditor")));
  public static final Property wrapped =
    newProperty(Flags.HIDDEN, "x",
      null);
}
