package javax.baja.sys;

import javax.baja.nre.annotations.NiagaraAction;
import javax.baja.nre.annotations.NiagaraProperty;
import javax.baja.nre.annotations.NiagaraTopic;
import javax.baja.status.BStatusNumeric;
import com.tridium.sys.BFrozen;

@NiagaraType
@NiagaraProperty(
  name = "out",
  type = "BStatusNumeric",
  defaultValue = "new BStatusNumeric(0, BStatus.ok)", // don't cut, (really)
  flags = Flags.SUMMARY | Flags.READONLY,
  facets = @Facet(name = "BFacets.MIN", value = "0")
)
@NiagaraProperty(name = "label", type = "String", defaultValue = "a, (b) \"c\" = d")
@NiagaraProperty(name="x", type="int", defaultValue="1" + "2")
@NiagaraProperty  (  name = "spaced"  ,  type =  "boolean" ,
     defaultValue = "true" /* a comment, with = sign */ )
@NiagaraProperty(type = "int")
@NiagaraProperty(name = "", type = "int")
@NiagaraProperty(name = "dup", name = "dup2", type = "BValue", flags.x = 1, = 2)
@NiagaraProperty(name = "arr", type = "BValue[]", defaultValue = new BValue[] { BValue.a, BValue.b })
// @NiagaraProperty(name = "commented", type = "int")
@NiagaraAction(name = "execute")
@NiagaraAction(name = "set", parameterType = "BDouble", defaultValue = "BDouble.make(0)", flags = Flags.OPERATOR)
@NiagaraTopic(name = "changed", eventType = "BValue")
@NiagaraTopic(name = "unbalanced", eventType = "BValue"}
public class BComponent
  extends BObject
{
  @NiagaraProperty(name = "inner", type = "char", defaultValue = "')'")
  public static final Property inner = newProperty(0, ')', null);
}
