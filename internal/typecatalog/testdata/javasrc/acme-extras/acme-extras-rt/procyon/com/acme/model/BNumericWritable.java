package com.acme.model;

public final class BNumericWritable extends BComponent
{
    public static final Property speed;
    public static final Action stop;
    public static final Property broken;

    static {
        speed = newProperty(Flags.OPERATOR, BDouble.make(1.5), null);
        stop = newAction(0x20, null);
        speedCheck = speed == null;
    }
}
