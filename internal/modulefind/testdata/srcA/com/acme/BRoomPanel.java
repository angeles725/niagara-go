package com.acme;

@NiagaraProperties({ @NiagaraProperty(name = "temp", type = "BStatusNumeric", flags = Flags.OPERATOR), @NiagaraProperty(name = "label", type = "baja:String", flags = Flags.READONLY | Flags.SUMMARY) })
@NiagaraProperty(name = "cache", type = "baja:Integer", flags = Flags.TRANSIENT | Flags.HIDDEN)
@NiagaraProperty(name = "untyped")
public class BRoomPanel extends BColdRoom
{
  void refresh(BColdRoom room)
  {
    room.forceDefrost();
    setTemp(new BStatusNumeric(1.0));
  }
}
