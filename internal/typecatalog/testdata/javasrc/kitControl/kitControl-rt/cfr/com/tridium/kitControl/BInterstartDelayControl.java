package com.tridium.kitControl;
public class BInterstartDelayControl
extends BComponent {
    public static final Property delay = BInterstartDelayControl.newProperty((int)0, (BValue)BRelTime.DEFAULT, null);
    public static final Property master = BInterstartDelayControl.newProperty((int)1026, (BValue)BOrd.NULL, BFacets.make((String)"a", (String)"b"));
    public static final Action reset = BInterstartDelayControl.newAction((int)256, null);
    public static final Action pulse = newAction(Flags.ASYNC | Flags.CONFIRM_REQUIRED, BBoolean.TRUE, BFacets.NULL);
}
