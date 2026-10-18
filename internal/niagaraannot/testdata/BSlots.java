package com.example.fixture;

@NiagaraType
@NiagaraSlots(
  properties = { @NiagaraProperty(name = "a", type = "int", defaultValue = "0", flags = 1 | 8) },
  actions = @NiagaraAction(name = "go", flags = Flags.CONFIRM_REQUIRED | Flags.NON_CRITICAL | Flags.LINK_TARGET),
  topics = {}
)
@NiagaraProperties({
  @NiagaraProperty(name = "b", type = "boolean", defaultValue = "false", override = true)
})
@NiagaraEnum(range = @Range("only"))
interface BSlots
{
}
