package com.tridium.deep.pkg;

import java.util.List;

/**
 * class InJavadoc extends Nothing {
 */
public class Generic<T extends Comparable<T>, U> extends Base<T> implements A, B<C, D>, E {
   public static class Inner extends Thing {
      private interface Deep {
      }
   }

   protected abstract static class Abs
      extends Other<Map<K, V>>
      implements X,
         Y<Z> {
   }

   enum Color implements Paint {
      RED;
   }
}

final class Second /* extends Hidden */ extends Shown // implements Gone
   implements Kept {
}
