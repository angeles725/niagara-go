package com.acme;

public class BDashServlet extends BWebServlet
{
  public void doPost(BComponent comp, String prop, BValue value)
  {
    comp.set(prop, value);
    BOrd h1 = BOrd.make("history:/station/temp");
    BOrd h2 = BOrd.make("history:/station/temp");
    BOrd page = BOrd.make("file:^web/index.html");
    // forceDefrost is named in a comment, not called
  }
}
