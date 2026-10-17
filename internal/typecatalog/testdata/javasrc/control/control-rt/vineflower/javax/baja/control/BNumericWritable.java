package javax.baja.control;
public class BNumericWritable
  extends BNumericPoint
  implements BIWritablePoint
{
  public static final Property in1 = newProperty(Flags.READONLY, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property in10 = newProperty(Flags.SUMMARY | Flags.TRANSIENT, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property fallback = newProperty(0, new BStatusNumeric(0, BStatus.nullStatus), null);
  public static final Property overrideExpiration = newProperty(Flags.OPERATOR | Flags.READONLY, BAbsTime.NULL, null);
  public static final Property units = newProperty(Flags.HIDDEN | 0x10000000, "Grados °C \"x\"\t;", BFacets.make("a", "b,c"));
  public static final Action emergencyAuto = newAction(0, null);
  public static final Action set = newAction(Flags.OPERATOR, BDouble.DEFAULT, null);
  public static final Topic fired = newTopic(Flags.SUMMARY, null);
  public static final Topic alarm = newTopic(Flags.SUMMARY, BString.DEFAULT, BFacets.NULL);
}
