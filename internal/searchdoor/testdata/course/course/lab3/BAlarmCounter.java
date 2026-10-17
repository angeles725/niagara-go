package com.course.lab3;

/** Counts alarm transitions of a point; the lab 3 exercise. */
public final class BAlarmCounter extends BComponent {
  public static final Property alarmCount = newProperty(0, 0, null);
  public void onAlarm(BAlarmRecord rec) { setAlarmCount(getAlarmCount() + 1); }
}
