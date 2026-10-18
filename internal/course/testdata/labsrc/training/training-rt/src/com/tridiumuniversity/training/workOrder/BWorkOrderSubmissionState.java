// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.training.workOrder;

@NiagaraType
@NiagaraEnum(range = { @Range("idle"), @Range("submitting") }, defaultValue = "idle")
public final class BWorkOrderSubmissionState extends BFrozenEnum
{
}
