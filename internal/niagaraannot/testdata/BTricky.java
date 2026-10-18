/*
 * A fixture: @NiagaraProperty(name = "inComment", type = "BString") is in a comment.
 */
package com.example.fixture;

import javax.baja.nre.annotations.*;
import javax.baja.sys.Flags;

// @NiagaraAction(name = "inLineComment")
@NiagaraType
@javax.baja.nre.annotations.NiagaraProperty(
  name = "out",
  type = "BStatusNumeric",
  defaultValue = "new BStatusNumeric(0, BStatus.nullStatus)",
  flags = Flags.SUMMARY |
          Flags.READONLY
)
@NiagaraProperty(name = "label", type = "String", defaultValue = "@NiagaraProperty(name = \"inString\")")
@NiagaraProperty(
  name = "url",
  type = "String",
  defaultValue = "http://" + "localhost",
  flags = (Flags.HIDDEN | 0x10) | javax.baja.sys.Flags.OPERATOR,
  facets = { @Facet(name = "BFacets.FIELD_WIDTH", value = "40"), @Facet("SfUtil.incl(SfUtil.MGR_EDIT)") }
)
@NiagaraAction(
  name = "submit",
  parameterType = "BWorkOrder",
  returnType = "BString",
  defaultValue = "new BWorkOrder()",
  flags = Flags.ASYNC
)
@NiagaraAction(name = "reset")
@NiagaraTopic(name = "submitted", eventType = "BString", flags = Flags.SUMMARY)
@SuppressWarnings("unused")
public class BTricky
  extends BComponent
{
//region /*+ ------------ BEGIN BAJA AUTO GENERATED CODE ------------ +*/
  @Generated
  public static final Property out = newProperty(Flags.SUMMARY | Flags.READONLY, null, null);
//endregion /*+ ------------ END BAJA AUTO GENERATED CODE -------------- +*/

  @Override
  public String toString() { return "@NiagaraProperty"; }

  @NiagaraType
  @NiagaraProperty(name = "nested", type = "BString", defaultValue = "")
  public static class BNested extends BComponent {}
}
