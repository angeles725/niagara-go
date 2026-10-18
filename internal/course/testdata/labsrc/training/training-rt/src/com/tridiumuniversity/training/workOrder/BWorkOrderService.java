// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.training.workOrder;

@NiagaraType
@NiagaraProperty(name = "url", type = "String", defaultValue = "http://localhost:3001/workOrders")
@NiagaraProperty(name = "state", type = "BWorkOrderSubmissionState", defaultValue = "BWorkOrderSubmissionState.idle", flags = Flags.READONLY)
@NiagaraAction(name = "submit", parameterType = "BWorkOrder", defaultValue = "new BWorkOrder()")
@NiagaraAction(name = "reset")
public class BWorkOrderService extends BAbstractService
{
}
