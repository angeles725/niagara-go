package com.tridium.eps;

import java.util.Map;
import static com.tridium.eps.Long.helper;
import com.tridium.eps.*;

/* leading block
   comment with class Fake { */
public class Members extends com.tridium.eps.Edge<String> implements Runnable {
    public static final String S = "brace { in string } and // not a comment";
    private char c = '{';
    private Map<String, java.util.List<Integer>> cache;
    int var1 = var2 + var33;

    public Members() {
        super();
        this(1);
    }

    Members(int x) { this.init(x); }

    @Override
    public void run() { helper(); Long.helper(); new Edge(); new java.util.HashMap<String, String>(); }

    protected synchronized <T extends Comparable<T>> T generic(final T a,
            @Deprecated Map<String, Integer> m,
            @SuppressWarnings("unchecked") java.util.List<T>... rest) throws Exception {
        super.run(); // super.hidden()
        String s = "Fake.call(x) \"Esc.aped()\""; char q = '"'; char r = '\'';
        Other.thing(); /* Inline.comment() */ return Fake.make(a);
    }

    static int[] arr(int a[], @A(x = 1) int b, @B(y z) int c) { return null; }

    abstract void noBody();

    public void tricky() { First.call(); /* opens a comment
        Hidden.call(); still inside */ void afterComment() { Visible.call(); }

    @interface Ann {
        String value() default "x";
    }

    enum Kind {
        ONE("a"), TWO("b"),
        THREE("c");
        Kind(String s) {}
        public String label() { return name(); }
    }

    class Inner {
        void innerMethod() { outer(); Members.this.run(); Util.café(); }
    }

    interface Iface { void m(); default void d() { Members.mk(); } }

    void überMethod() { Ünicode.call(); x.lower(); String.valueOf(1); new Ünit(); new int[3]; }

    record Point(int x, int y) { Point { Objects.requireNonNull(x); } }

    void multi(int a,
               int b
    ) {
        if (a > b) { foo(); }
        Foo.bar(); Foo.bar(); a.super(); x .this(); this.x = new Foo<Bar>(1);
    }

    void tooLong(int a,
        int b1,
        int b2,
        int b3,
        int b4,
        int b5,
        int b6,
        int b7,
        int b8,
        int b9,
        int b10,
        int b11,
        int b12,
        int b13,
        int b14,
        int b15,
        int b16) {
        Long.never();
    }

public void columnZero() { Zero.call(); }
    static { Static.init(); }
    void synchronizedCall() { synchronized (this) { Lock.held(); } }
    String Members2() { return Members.make().other(); }
}

class Second {
    Second() { super(); }
    void m() throws X { Third.go(); }
}
