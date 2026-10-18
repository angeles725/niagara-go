package com.tridium.deep.pkg;

@Deprecated @SuppressWarnings("unchecked") public class Annotated extends Base {
}
@Doc("public class Annotated2 extends Fake") public final class Annotated2 extends Real {
}
@Foo(value = "x") class AnnSp {}
public @interface MyAnno {
   String value();
}
public record Point(int x, int y) implements Shape {
}
public sealed interface Shape permits Point, Circle {
}
non-sealed class NS extends Shape2 {}
