package com.acme;

@NiagaraType
@NiagaraProperty(
  name = "setpoint",
  type = "baja:StatusNumeric",
  defaultValue = "new BStatusNumeric(2.0)",
  flags = Flags.SUMMARY | Flags.OPERATOR,
  facets = @Facet("BFacets.make(BFacets.MIN, BDouble.make(-40.0d))")
)
@NiagaraProperty(name = "mode", type = "baja:String", flags = Flags.OPERATOR)
@NiagaraProperty(type = "baja:Integer")
@NiagaraAction(name = "applySetpoint", flags = Flags.OPERATOR)
@NiagaraAction(name = "defrost")
@NiagaraAction(name = "reset", flags = Flags.HIDDEN)
public class BColdRoom extends BComponent
{
  public void applySetpoint()
  {
    setSetpoint(new BStatusNumeric(4.0));
    set("mode", BString.make("auto"));
    this.set("mode", BString.make("manual"));
  }

  public void forceDefrost()
  {
    BOrd drivers = BOrd.make("station:|slot:/Drivers");
    BOrd rooms = BOrd.make("slot:/Config/Rooms");
  }
}
