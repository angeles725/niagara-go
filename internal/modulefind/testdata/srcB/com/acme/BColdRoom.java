package com.acme;

@NiagaraProperty(name = "setpoint", type = "baja:Double", flags = Flags.SUMMARY | Flags.OPERATOR)
@NiagaraProperty(name = "mode", type = "baja:String", flags = Flags.SUMMARY)
@NiagaraProperty(name = "alarmLimit", type = "baja:StatusNumeric", flags = Flags.OPERATOR)
@NiagaraAction(name = "applySetpoint", flags = Flags.OPERATOR)
@NiagaraAction(name = "silence")
public class BColdRoom extends BComponent
{
}
