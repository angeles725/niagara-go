package com.course.exam;

/** The verified exam solution: a room panel with a setpoint, an alarm limit and a fan. */
public final class BRoomPanel extends BComponent {
  public static final Property setpoint = newProperty(0, 21.0, null);
  public static final Property alarmLimit = newProperty(0, 28.0, null);
}
