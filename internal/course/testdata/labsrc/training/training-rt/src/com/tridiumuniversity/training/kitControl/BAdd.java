// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.training.kitControl;

@NiagaraType
@NiagaraProperty(name = "out", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)",
  flags = Flags.SUMMARY | Flags.READONLY)
@NiagaraProperty(name = "in1", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in2", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)", flags = Flags.SUMMARY)
@NiagaraProperty(name = "in3", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)")
@NiagaraProperty(name = "in4", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)")
public class BAdd extends BComponent
{
}
