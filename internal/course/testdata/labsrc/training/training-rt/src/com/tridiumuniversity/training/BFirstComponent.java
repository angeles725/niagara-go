// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.training;

@NiagaraType
@NiagaraProperty(name = "timeExecuted", type = "BAbsTime", defaultValue = "BAbsTime.DEFAULT", flags = Flags.SUMMARY)
@NiagaraAction(name = "execute", flags = Flags.SUMMARY)
public class BFirstComponent extends BComponent
{
}
