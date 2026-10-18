// Synthetic fixture for the course annotation checks: the annotations of the lab class only.
package com.tridiumuniversity.trafficLightDriver;

@NiagaraType
@NiagaraProperty(name = "pollFrequency", type = "BPollFrequency", defaultValue = "BPollFrequency.normal")
@NiagaraProperty(name = "intersectionId", type = "String", defaultValue = "A",
  facets = @Facet("SfUtil.incl(SfUtil.MGR_EDIT)"))
public class BTrafficLightDriverDevice extends BNDevice
{
}
