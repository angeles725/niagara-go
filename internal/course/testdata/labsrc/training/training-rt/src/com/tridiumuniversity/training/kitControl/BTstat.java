// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.training.kitControl;

@NiagaraType
@NiagaraProperty(name = "out", type = "BStatusBoolean", defaultValue = "new BStatusBoolean(false, BStatus.nullStatus)",
  flags = Flags.SUMMARY | Flags.READONLY)
@NiagaraProperty(name = "cv", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)")
@NiagaraProperty(name = "sp", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)")
@NiagaraProperty(name = "diff", type = "BStatusNumeric", defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)")
@NiagaraProperty(name = "action", type = "BLoopAction", defaultValue = "BLoopAction.DEFAULT")
public class BTstat extends BComponent
{
}
