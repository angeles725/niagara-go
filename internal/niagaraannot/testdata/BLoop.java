package com.example.fixture;

import javax.baja.nre.annotations.NiagaraEnum;
import javax.baja.nre.annotations.NiagaraType;
import javax.baja.nre.annotations.Range;

@NiagaraType
@NiagaraEnum(range = {
  @Range("direct"),
  @Range(value = "reverseActing", ordinal = 5),
  @Range("io16V1"),
  @Range(value = "nul", ordinal = -1)
}, defaultValue = "reverseActing")
public final class BLoop extends BFrozenEnum
{
}
