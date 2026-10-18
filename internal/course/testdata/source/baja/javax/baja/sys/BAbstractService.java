// A synthetic stand-in for the 4.15 BAbstractService: only the members the source-text checks read
// (C7.6-C7.8), in the shape the real file has them.
package javax.baja.sys;

public abstract class BAbstractService extends BComponent
{
  public static final Property status = newProperty(Flags.READONLY | Flags.TRANSIENT, BStatus.ok, null);
  public static final Property faultCause = newProperty(Flags.READONLY | Flags.TRANSIENT, "", null);
  public static final Property enabled = newProperty(0, true, null);

  public void updateStatus()
  {
    int newStatus = 0;
    // ... fault bit
    if (fatalFault || configFault)
    {
      newStatus |= BStatus.FAULT;
    }
    else
    {
      newStatus &= ~BStatus.FAULT;
    }
  }

  public final void configOk()
  {
    configFault = false;
    if (fatalFault)
    {
      return;
    }
    setFaultCause("");
    updateStatus();
  }
}
