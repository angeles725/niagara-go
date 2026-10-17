package com.acme.util;

public final class Helper
{
  static void kick(BColdRoom room)
  {
    room.forceDefrost ();
    room.setMode(BString.make("eco"));
  }
}
